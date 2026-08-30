// Package ports declares the interfaces the application layer needs.
// Adapters implement them; the app layer never imports an adapter.
package ports

import (
	"context"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	ingestdomain "github.com/ghanageo/ghanageo/services/api/internal/domain/ingest"
)

// Page is a cursor-paginated result. Cursor pagination lives in the port
// signature rather than being bolted on at the transport (plan GEO-7.2).
type Page[T any] struct {
	Data           []T
	NextCursor     string
	Total          int64
	DatasetVersion string
}

// ListParams are the shared list controls.
type ListParams struct {
	Cursor string
	Limit  int
	// IncludeGeometry loads boundary polygons with the list.
	//
	// Off by default because it is expensive and almost nobody wants it: a
	// district carries roughly 165KB of polygon, so a 50-row page pulls ~8MB
	// out of Mongo to decode and then discard, which took the districts list
	// from ~100ms to a p95 over 1s. Public list endpoints return metadata;
	// geometry comes from /boundaries/{id} or the bulk download. Only the
	// dataset exporter sets this.
	IncludeGeometry bool
}

// Normalize clamps the page size to the documented maximum.
func (p ListParams) Normalize() ListParams {
	const defaultLimit, maxLimit = 20, 100
	if p.Limit <= 0 {
		p.Limit = defaultLimit
	}
	if p.Limit > maxLimit {
		p.Limit = maxLimit
	}
	return p
}

type DistrictFilter struct {
	ListParams
	RegionID string
	Query    string
}

type PlaceFilter struct {
	ListParams
	DistrictID string
	RegionID   string
	Type       string
	Query      string
}

// RegionRepository reads and writes canonical regions.
type RegionRepository interface {
	List(ctx context.Context, p ListParams) (Page[geography.Region], error)
	Get(ctx context.Context, id string) (*geography.Region, error)
	Upsert(ctx context.Context, r geography.Region) (created bool, err error)
	Count(ctx context.Context) (int64, error)
}

type DistrictRepository interface {
	List(ctx context.Context, f DistrictFilter) (Page[geography.District], error)
	Get(ctx context.Context, id string) (*geography.District, error)
	Upsert(ctx context.Context, d geography.District) (created bool, err error)
	Count(ctx context.Context) (int64, error)
	CountOrphans(ctx context.Context) (int64, error)
}

type PlaceRepository interface {
	List(ctx context.Context, f PlaceFilter) (Page[geography.Place], error)
	Get(ctx context.Context, id string) (*geography.Place, error)
	Upsert(ctx context.Context, p geography.Place) (created bool, err error)
	Count(ctx context.Context) (int64, error)
	// Nearby returns places within radiusMeters of a point, nearest first.
	Nearby(ctx context.Context, c geography.Coordinate, radiusMeters int, limit int) ([]geography.Place, error)
}

// ImportRunRepository owns the durable ingestion unit of work. Record commits
// atomically persist the canonical place, its privacy-safe source reference,
// and the run counters, so an interrupted import can be retried safely.
type ImportRunRepository interface {
	Queue(context.Context, ingestdomain.Run) (ingestdomain.Run, bool, error)
	Start(context.Context, string, time.Time) error
	CommitRecord(context.Context, ingestdomain.RawRecord, *geography.Place) (created bool, err error)
	Complete(context.Context, string, time.Time, int64, int64) error
	Fail(context.Context, string, ingestdomain.Error, time.Time) error
}

// RedirectRepository resolves deprecated and merged identifiers so old IDs
// keep working (Spec 18).
type RedirectRepository interface {
	Resolve(ctx context.Context, oldID string) (*geography.Redirect, error)
	Put(ctx context.Context, r geography.Redirect) error
}

// BoundaryRepository is the privileged boundary-write contract. The expected
// geometry makes updates compare-and-swap: a steward cannot silently overwrite
// a boundary changed since it was loaded.
type BoundaryRepository interface {
	GetGeometry(ctx context.Context, id string) (*geography.Geometry, error)
	SetGeometryCAS(ctx context.Context, id string, expected, next *geography.Geometry, evidence audit.Entry) (bool, error)
}

// AdminGeographyRepository owns privileged canonical writes. Implementations
// commit the canonical change, redirect/alias projection and audit evidence in
// one database transaction.
type AdminGeographyRepository interface {
	CreateRegion(context.Context, geography.Region, audit.Entry) error
	CreateDistrict(context.Context, geography.District, audit.Entry) error
	CreatePlace(context.Context, geography.Place, audit.Entry) error
	CreateRoad(context.Context, geography.Road, audit.Entry) error
	CreatePOI(context.Context, geography.POI, audit.Entry) error
	UpdateRegion(context.Context, geography.Region, audit.Entry) error
	UpdateDistrict(context.Context, geography.District, audit.Entry) error
	UpdatePlace(context.Context, geography.Place, audit.Entry) error
	Deprecate(context.Context, string, string, string, string, audit.Entry) error
	ListRedirects(context.Context, ListParams) (Page[geography.Redirect], error)
	ListAliases(context.Context, string) ([]geography.Alias, error)
	CreateAlias(context.Context, geography.Alias, audit.Entry) error
	DeprecateAlias(context.Context, string, string, audit.Entry) error
}

// SearchPort abstracts the search engine. Two implementations exist:
// Typesense (self-hosted, used locally and in CI) and Atlas Search (hosted).
// Neither the use cases nor the transports know which is active.
type SearchPort interface {
	Index(ctx context.Context, docs []SearchDoc) error
	// Rebuild discards the index and starts clean. Required because Index is
	// an upsert: without it, a record deleted or merged in the database stays
	// searchable forever.
	Rebuild(ctx context.Context) error
	Search(ctx context.Context, q SearchQuery) ([]SearchHit, error)
	Autocomplete(ctx context.Context, prefix string, limit int) ([]SearchHit, error)
	EnsureSchema(ctx context.Context) error
}

// SearchDoc is one indexed record. Search spans regions, districts AND places,
// because a user typing "Tema" means a district as readily as a locality.
type SearchDoc struct {
	ID           string
	Name         string
	Normalized   string
	Type         string
	RegionID     string
	RegionName   string
	DistrictID   string
	DistrictName string
	Aliases      []string
	Latitude     float64
	Longitude    float64
	// Kind is "region", "district" or "place" — what the id resolves to.
	Kind string
	// Weight breaks ties: administrative units rank above minor localities.
	Weight int32
}

type SearchQuery struct {
	Text       string
	Limit      int
	RegionID   string
	DistrictID string
	Type       string
}

// SearchHit carries the confidence score and match explanation that Spec 10
// requires every result to expose.
type SearchHit struct {
	Doc         SearchDoc
	Score       float64
	MatchReason string
}

// Clock is injected so time-dependent behaviour is testable.
type Clock interface{ NowRFC3339() string }
