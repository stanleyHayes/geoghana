package graphql

import (
	"context"
	"net/http"
	"sync"

	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// Per-request loaders, which is how Spec §8.4 asks for N+1 to be prevented.
//
// A query like `districts(first: 20) { region { capital } }` would otherwise
// issue twenty region lookups — and because those twenty districts usually
// share a handful of regions, nineteen of them would be identical.
//
// These loaders MEMOIZE within a request: the first lookup of a given id runs,
// every later one waits on that same result. For this schema's shape that
// collapses the twenty calls to one or two.
//
// What this is NOT is batching: it does not gather ids across a tick and issue
// a single multi-get. Twenty DIFFERENT regions would still be twenty queries.
// Adding that needs GetMany on the repositories, and is worth doing when a
// query shape appears that actually fans out that way. Memoization solves the
// shape this schema has today, and pretending otherwise in a comment would be
// worse than the limitation.
type loaders struct {
	geo *appgeo.Service

	mu        sync.Mutex
	regions   map[string]*result[domain.Region]
	districts map[string]*result[domain.District]
}

type result[T any] struct {
	once sync.Once
	val  *T
	err  error
}

type loaderKey struct{}

func newLoaders(geo *appgeo.Service) *loaders {
	return &loaders{
		geo:       geo,
		regions:   map[string]*result[domain.Region]{},
		districts: map[string]*result[domain.District]{},
	}
}

// Middleware attaches a fresh set of loaders to each request. They must NOT be
// shared across requests: a cache that outlives a request would serve stale
// data after a dataset release, silently.
func LoaderMiddleware(geo *appgeo.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), loaderKey{}, newLoaders(geo))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func loadersFrom(ctx context.Context) *loaders {
	l, _ := ctx.Value(loaderKey{}).(*loaders)
	return l
}

func (l *loaders) Region(ctx context.Context, id string) (*domain.Region, error) {
	l.mu.Lock()
	r, ok := l.regions[id]
	if !ok {
		r = &result[domain.Region]{}
		l.regions[id] = r
	}
	l.mu.Unlock()

	r.once.Do(func() { r.val, r.err = l.geo.GetRegion(ctx, id) })
	return r.val, r.err
}

func (l *loaders) District(ctx context.Context, id string) (*domain.District, error) {
	l.mu.Lock()
	d, ok := l.districts[id]
	if !ok {
		d = &result[domain.District]{}
		l.districts[id] = d
	}
	l.mu.Unlock()

	d.once.Do(func() { d.val, d.err = l.geo.GetDistrict(ctx, id) })
	return d.val, d.err
}
