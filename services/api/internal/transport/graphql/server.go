package graphql

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/gqlerror"

	appdataset "github.com/ghanageo/ghanageo/services/api/internal/app/dataset"
	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/observability"
	"github.com/ghanageo/ghanageo/services/api/internal/transport/graphql/generated"
	"github.com/ghanageo/ghanageo/services/api/internal/transport/graphql/model"
	"github.com/vektah/gqlparser/v2/ast"
)

// Protections required by Spec §8.4. A public GraphQL endpoint without these
// is an invitation to exhaust the database with one deeply nested query.
const (
	// MaxDepth caps nesting. The deepest legitimate query in this schema is
	// place → district → region → districts → places, which is five.
	MaxDepth = 8
	// MaxComplexity is the weighted field budget per request.
	MaxComplexity = 300
	// QueryTimeout bounds a single operation.
	QueryTimeout = 15 * time.Second
)

// Field cost multipliers. Geometry and spatial work costs more because it
// costs more to serve, matching the REST cost classes in Appendix D.
const (
	costBoundary = 40 // a full polygon can be half a megabyte
	costSpatial  = 12
	costSearch   = 6
	costList     = 2
)

// NewHandler builds the /graphql endpoint.
func NewHandler(geo *appgeo.Service, search *appsearch.Service, datasets ...*appdataset.Service) http.Handler {
	var datasetSvc *appdataset.Service
	if len(datasets) > 0 {
		datasetSvc = datasets[0]
	}
	return newHandler(geo, search, datasetSvc, nil)
}

func NewHandlerWithTelemetry(geo *appgeo.Service, search *appsearch.Service, datasetSvc *appdataset.Service, telemetry *observability.Telemetry) http.Handler {
	return newHandler(geo, search, datasetSvc, telemetry)
}

func newHandler(geo *appgeo.Service, search *appsearch.Service, datasetSvc *appdataset.Service, telemetry *observability.Telemetry) http.Handler {
	cfg := generated.Config{Resolvers: &Resolver{GeoSvc: geo, SearchSvc: search, DatasetSvc: datasetSvc}}

	// Per-field cost. Without these every field costs 1 and the complexity
	// budget would treat a boundary fetch the same as reading a name.
	cfg.Complexity.Region.Boundary = func(childComplexity int) int { return costBoundary }
	cfg.Complexity.District.Boundary = func(childComplexity int) int { return costBoundary }
	cfg.Complexity.Place.Boundary = func(childComplexity int) int { return costBoundary }
	cfg.Complexity.Query.Nearby = func(childComplexity int, lat, lng float64, radius *int, first *int) int {
		return costSpatial * limitOf(first, 20)
	}
	cfg.Complexity.Query.Reverse = func(childComplexity int, lat, lng float64) int { return costSpatial }
	cfg.Complexity.Query.Search = func(childComplexity int, q string, t []model.PlaceType, r *string, first *int) int {
		return costSearch * limitOf(first, 20)
	}
	cfg.Complexity.Query.Regions = func(childComplexity int, first *int, after *string) int {
		return costList * limitOf(first, 20) * childComplexity
	}
	cfg.Complexity.Query.Districts = func(childComplexity int, r, q *string, first *int, after *string) int {
		return costList * limitOf(first, 20) * childComplexity
	}
	cfg.Complexity.Query.Places = func(childComplexity int, d, r *string, t *model.PlaceType, q *string, first *int, after *string) int {
		return costList * limitOf(first, 20) * childComplexity
	}

	srv := handler.New(generated.NewExecutableSchema(cfg))

	// POST only. GET would make a mutation-shaped request cacheable and
	// CSRF-able; this API is read-only today but the endpoint should not
	// become unsafe the moment a mutation is added.
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.Options{})

	queryCache := graphql.Cache[*ast.QueryDocument](lru.New[*ast.QueryDocument](200))
	persistedCache := graphql.Cache[string](lru.New[string](200))
	if telemetry != nil {
		queryCache = observedCache[*ast.QueryDocument]{name: "graphql_query", next: queryCache, telemetry: telemetry}
		persistedCache = observedCache[string]{name: "graphql_apq", next: persistedCache, telemetry: telemetry}
	}
	srv.SetQueryCache(queryCache)
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{Cache: persistedCache})
	srv.Use(depthLimit{max: MaxDepth})
	srv.Use(extension.FixedComplexityLimit(MaxComplexity))

	// Errors carry the same stable codes REST returns, so a client can branch
	// on extensions.code exactly as it branches on error.code (Spec §19).
	srv.SetErrorPresenter(func(ctx context.Context, e error) *gqlerror.Error {
		err := graphql.DefaultErrorPresenter(ctx, e)
		var ae *apierr.Error
		if errors.As(e, &ae) {
			if err.Extensions == nil {
				err.Extensions = map[string]any{}
			}
			err.Extensions["code"] = string(ae.Code)
			err.Extensions["docs"] = ae.Code.DocsURL()
			for k, v := range ae.Details {
				err.Extensions[k] = v
			}
			err.Message = ae.Message
		}
		return err
	})

	// A panic must not take the process down, and must not leak a stack trace
	// to a caller either.
	srv.SetRecoverFunc(func(ctx context.Context, err any) error {
		return apierr.New(apierr.Internal, "An unexpected error occurred.")
	})

	// Loaders are attached per request; sharing them across requests would
	// serve stale data after a dataset release.
	return http.TimeoutHandler(LoaderMiddleware(geo, srv), QueryTimeout, `{"errors":[{"message":"Query timed out.","extensions":{"code":"DEADLINE_EXCEEDED"}}]}`)
}

type observedCache[T any] struct {
	name      string
	next      graphql.Cache[T]
	telemetry *observability.Telemetry
}

func (c observedCache[T]) Get(ctx context.Context, key string) (T, bool) {
	value, ok := c.next.Get(ctx, key)
	if ok {
		c.telemetry.ObserveCache(c.name, "hit")
	} else {
		c.telemetry.ObserveCache(c.name, "miss")
	}
	return value, ok
}

func (c observedCache[T]) Add(ctx context.Context, key string, value T) {
	c.next.Add(ctx, key, value)
}
