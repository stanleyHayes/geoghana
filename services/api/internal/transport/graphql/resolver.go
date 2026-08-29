package graphql

import (
	appdataset "github.com/ghanageo/ghanageo/services/api/internal/app/dataset"
	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
)

//go:generate go run github.com/99designs/gqlgen generate

// Resolver wires the GraphQL transport to the SAME use cases REST and gRPC
// call. That is the whole point of Spec §6: a resolver that reached into a
// repository directly would let GraphQL and REST drift apart, and the
// cross-protocol parity test exists to catch exactly that.
// The fields are suffixed Svc because a resolver method named Search would
// otherwise shadow a field named Search through the embedded *Resolver, and
// r.Search inside that method would resolve to the method itself.
type Resolver struct {
	GeoSvc     *appgeo.Service
	SearchSvc  *appsearch.Service
	DatasetSvc *appdataset.Service
}
