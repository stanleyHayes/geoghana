// Package search holds the search, autocomplete, geocode and reverse-geocode
// use cases. Like all use cases these are transport-agnostic: REST, GraphQL
// and gRPC all call them, which is what makes results consistent.
package search

import (
	"context"
	"errors"
	"sort"
	"strings"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/relevance"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// MinQueryLength is enforced server-side, not only in the client SDK.
const MinQueryLength = 2

type Service struct {
	search    ports.SearchPort
	regions   ports.RegionRepository
	districts ports.DistrictRepository
	places    ports.PlaceRepository
	version   string
}

func NewService(
	s ports.SearchPort, r ports.RegionRepository,
	d ports.DistrictRepository, p ports.PlaceRepository, version string,
) *Service {
	return &Service{search: s, regions: r, districts: d, places: p, version: version}
}

func (s *Service) DatasetVersion() string { return s.version }

func validateQuery(q string) (string, error) {
	q = strings.TrimSpace(q)
	if len([]rune(q)) < MinQueryLength {
		return "", apierr.
			New(apierr.QueryTooShort, "The search query is too short.").
			WithDetail("minLength", MinQueryLength)
	}
	return q, nil
}

// Search returns ranked candidates. An ambiguous query yields several
// candidates with confidence scores rather than a single guess (Spec 10).
func (s *Service) Search(ctx context.Context, q ports.SearchQuery) ([]ports.SearchHit, error) {
	text, err := validateQuery(q.Text)
	if err != nil {
		return nil, err
	}
	if q.Type != "" {
		t, terr := domain.ParsePlaceType(q.Type)
		if terr != nil {
			return nil, apierr.New(apierr.InvalidArgument, "Unknown place type.").WithDetail("type", q.Type)
		}
		q.Type = string(t)
	}
	q.Text = text
	// Over-fetch, because the engine RETRIEVES and the domain RANKS. Typesense
	// cannot separate "Accra" from "Kwahu Afram Plains South" for the query
	// "acra" — both are one-token fuzzy hits with an identical packed score —
	// so trimming to the caller's limit before rescoring would discard the
	// right answer.
	want := q.Limit
	if want <= 0 {
		want = 20
	}
	q.Limit = min(want*5, 100)

	hits, err := s.search.Search(ctx, q)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Search failed.", err)
	}
	return rescore(text, hits, want), nil
}

// rescore replaces the engine's opaque score with a domain confidence in 0..1
// and a human explanation, then sorts and trims. Both SearchPort
// implementations therefore return identical confidences (Spec 10).
func rescore(query string, hits []ports.SearchHit, limit int) []ports.SearchHit {
	nq := normalize.Name(query)
	out := make([]ports.SearchHit, 0, len(hits))
	for _, h := range hits {
		name := h.Doc.Normalized
		if name == "" {
			name = normalize.Name(h.Doc.Name)
		}
		aliases := make([]string, 0, len(h.Doc.Aliases))
		for _, a := range h.Doc.Aliases {
			aliases = append(aliases, normalize.Name(a))
		}
		r := relevance.Score(nq, name, aliases)
		if r.Kind == relevance.NoMatch {
			continue
		}
		h.Score = round2(r.Score)
		h.MatchReason = r.Reason
		out = append(out, h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		// Equal confidence: prefer the more significant administrative unit,
		// so "Greater Accra" the region leads "Accra" the locality.
		return out[i].Doc.Weight > out[j].Doc.Weight
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

func (s *Service) Autocomplete(ctx context.Context, prefix string, limit int) ([]ports.SearchHit, error) {
	text, err := validateQuery(prefix)
	if err != nil {
		return nil, err
	}
	want := limit
	if want <= 0 {
		want = 10
	}
	hits, err := s.search.Autocomplete(ctx, text, min(want*5, 100))
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Autocomplete failed.", err)
	}
	return rescore(text, hits, want), nil
}

// Geocode is text to ranked geographic candidates. It shares ranking with
// Search deliberately: two endpoints that disagree about what "Osu" means
// would be a bug, not a feature.
func (s *Service) Geocode(ctx context.Context, query string, limit int) ([]ports.SearchHit, error) {
	return s.Search(ctx, ports.SearchQuery{Text: query, Limit: limit})
}

// ReverseResult is what a coordinate resolves to.
type ReverseResult struct {
	Region         *domain.Region
	District       *domain.District
	Nearby         []domain.Place
	DatasetVersion string
}

// Reverse resolves a coordinate to its containing geography.
//
// A coordinate outside Ghana returns an EMPTY result, not an error: asking
// about Lagos is a legitimate question with the answer "nothing here"
// (Spec 10). An impossible coordinate is a genuine error.
func (s *Service) Reverse(ctx context.Context, lat, lng float64) (*ReverseResult, error) {
	c := domain.Coordinate{Latitude: lat, Longitude: lng}
	if err := c.Validate(); err != nil {
		if errors.Is(err, domain.ErrNotInGhana) {
			return &ReverseResult{DatasetVersion: s.version}, nil
		}
		return nil, apierr.Wrap(apierr.InvalidCoordinate, "Coordinates are out of range.", err)
	}

	out := &ReverseResult{DatasetVersion: s.version}

	// Nearest localities first. Containment against district polygons is added
	// by GEO-4.6 once GSS boundary geometry is ingested; until then proximity
	// is the honest answer rather than a fabricated containment.
	near, err := s.places.Nearby(ctx, c, 25_000, 5)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Reverse geocode failed.", err)
	}
	out.Nearby = near

	if len(near) > 0 && near[0].DistrictID != "" {
		if d, derr := s.districts.Get(ctx, near[0].DistrictID); derr == nil {
			out.District = d
		}
	}
	if len(near) > 0 && near[0].RegionID != "" {
		if r, rerr := s.regions.Get(ctx, near[0].RegionID); rerr == nil {
			out.Region = r
		}
	}
	return out, nil
}

// Reindex rebuilds the search index from canonical data. Regions, districts
// and places are all indexed: a user typing "Tema" means a district as
// readily as a locality.
func (s *Service) Reindex(ctx context.Context) (int, error) {
	// A true rebuild, not an upsert pass: indexing is upsert-only, so anything
	// merged or deleted since the last run would otherwise linger and keep
	// appearing in results.
	if err := s.search.Rebuild(ctx); err != nil {
		return 0, err
	}

	var docs []ports.SearchDoc

	regions, err := s.regions.List(ctx, ports.ListParams{Limit: 100})
	if err != nil {
		return 0, err
	}
	for _, r := range regions.Data {
		docs = append(docs, ports.SearchDoc{
			ID: r.ID, Name: r.Name, Normalized: normalize.Name(r.Name),
			Type: "REGION", RegionID: r.ID, RegionName: r.Name,
			Kind: "region", Weight: 300,
		})
	}

	cursor := ""
	for {
		page, derr := s.districts.List(ctx, ports.DistrictFilter{
			ListParams: ports.ListParams{Limit: 100, Cursor: cursor},
		})
		if derr != nil {
			return 0, derr
		}
		for _, d := range page.Data {
			docs = append(docs, ports.SearchDoc{
				ID: d.ID, Name: d.Name, Normalized: normalize.Name(d.Name),
				Type: "DISTRICT", RegionID: d.RegionID, RegionName: d.RegionName,
				DistrictID: d.ID, DistrictName: d.Name,
				Kind: "district", Weight: 200,
			})
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	cursor = ""
	for {
		page, perr := s.places.List(ctx, ports.PlaceFilter{
			ListParams: ports.ListParams{Limit: 100, Cursor: cursor},
		})
		if perr != nil {
			return 0, perr
		}
		for _, p := range page.Data {
			aliases := make([]string, 0, len(p.Aliases))
			for _, a := range p.Aliases {
				aliases = append(aliases, a.Value)
			}
			doc := ports.SearchDoc{
				ID: p.ID, Name: p.Name, Normalized: p.NormalizedName,
				Type: string(p.Type), Aliases: aliases,
				RegionID: p.RegionID, RegionName: p.RegionName,
				DistrictID: p.DistrictID, DistrictName: p.DistrictName,
				Kind: "place", Weight: placeWeight(p.Type),
			}
			if p.Centroid != nil {
				doc.Latitude = p.Centroid.Latitude
				doc.Longitude = p.Centroid.Longitude
			}
			docs = append(docs, doc)
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	// Import in batches so a large canonical dataset never builds one enormous
	// request body.
	const batch = 500
	for i := 0; i < len(docs); i += batch {
		end := min(i+batch, len(docs))
		if err := s.search.Index(ctx, docs[i:end]); err != nil {
			return 0, err
		}
	}
	return len(docs), nil
}

// placeWeight ranks a capital above a hamlet when scores tie.
func placeWeight(t domain.PlaceType) int32 {
	switch t {
	case domain.PlaceRegionalCapital:
		return 250
	case domain.PlaceCity:
		return 180
	case domain.PlaceTown:
		return 150
	case domain.PlaceSuburb, domain.PlaceNeighbourhood:
		return 120
	case domain.PlaceCommunity, domain.PlaceVillage:
		return 100
	default:
		return 80
	}
}
