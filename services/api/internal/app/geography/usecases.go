// Package geography holds the application use cases for canonical geography.
//
// These are the ONLY place business rules live. REST, GraphQL and gRPC all call
// them, which is what makes the three transports semantically consistent
// (Spec 6, plan rule R6).
package geography

import (
	"context"
	"errors"
	"strings"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// ErrNotFound is what a repository returns for a missing id.
var ErrNotFound = errors.New("not found")

type Service struct {
	regions   ports.RegionRepository
	districts ports.DistrictRepository
	places    ports.PlaceRepository
	redirects ports.RedirectRepository
	version   string
}

func NewService(
	r ports.RegionRepository, d ports.DistrictRepository,
	p ports.PlaceRepository, rd ports.RedirectRepository, datasetVersion string,
) *Service {
	return &Service{regions: r, districts: d, places: p, redirects: rd, version: datasetVersion}
}

func (s *Service) DatasetVersion() string { return s.version }

func notFound(kind, id string) *apierr.Error {
	return apierr.New(apierr.NotFound, kind+" not found").WithDetail("id", id)
}

func (s *Service) ListRegions(ctx context.Context, p ports.ListParams) (ports.Page[domain.Region], error) {
	page, err := s.regions.List(ctx, p)
	if err != nil {
		return page, apierr.Wrap(apierr.Internal, "Failed to list regions.", err)
	}
	page.DatasetVersion = s.version
	return page, nil
}

func (s *Service) GetRegion(ctx context.Context, id string) (*domain.Region, error) {
	if strings.TrimSpace(id) == "" {
		return nil, apierr.New(apierr.InvalidArgument, "A region id is required.")
	}
	r, err := s.regions.Get(ctx, id)
	if err != nil {
		return nil, s.mapLookupErr(ctx, "Region", id, err)
	}
	r.DatasetVersion = s.version
	return r, nil
}

func (s *Service) ListDistricts(ctx context.Context, f ports.DistrictFilter) (ports.Page[domain.District], error) {
	page, err := s.districts.List(ctx, f)
	if err != nil {
		return page, apierr.Wrap(apierr.Internal, "Failed to list districts.", err)
	}
	page.DatasetVersion = s.version
	return page, nil
}

// ListDistrictsInRegion returns 404 for an unknown region rather than an empty
// list, so a typo is distinguishable from a genuinely empty region.
func (s *Service) ListDistrictsInRegion(
	ctx context.Context, regionID string, p ports.ListParams,
) (ports.Page[domain.District], error) {
	if _, err := s.GetRegion(ctx, regionID); err != nil {
		return ports.Page[domain.District]{}, err
	}
	return s.ListDistricts(ctx, ports.DistrictFilter{ListParams: p, RegionID: regionID})
}

func (s *Service) GetDistrict(ctx context.Context, id string) (*domain.District, error) {
	if strings.TrimSpace(id) == "" {
		return nil, apierr.New(apierr.InvalidArgument, "A district id is required.")
	}
	d, err := s.districts.Get(ctx, id)
	if err != nil {
		return nil, s.mapLookupErr(ctx, "District", id, err)
	}
	d.DatasetVersion = s.version
	return d, nil
}

func (s *Service) ListPlaces(ctx context.Context, f ports.PlaceFilter) (ports.Page[domain.Place], error) {
	if f.Type != "" {
		t, err := domain.ParsePlaceType(f.Type)
		if err != nil {
			return ports.Page[domain.Place]{}, apierr.
				New(apierr.InvalidArgument, "Unknown place type.").
				WithDetail("type", f.Type)
		}
		f.Type = string(t)
	}
	page, err := s.places.List(ctx, f)
	if err != nil {
		return page, apierr.Wrap(apierr.Internal, "Failed to list places.", err)
	}
	page.DatasetVersion = s.version
	return page, nil
}

func (s *Service) ListPlacesInDistrict(
	ctx context.Context, districtID string, f ports.PlaceFilter,
) (ports.Page[domain.Place], error) {
	if _, err := s.GetDistrict(ctx, districtID); err != nil {
		return ports.Page[domain.Place]{}, err
	}
	f.DistrictID = districtID
	return s.ListPlaces(ctx, f)
}

func (s *Service) GetPlace(ctx context.Context, id string) (*domain.Place, error) {
	if strings.TrimSpace(id) == "" {
		return nil, apierr.New(apierr.InvalidArgument, "A place id is required.")
	}
	p, err := s.places.Get(ctx, id)
	if err != nil {
		return nil, s.mapLookupErr(ctx, "Place", id, err)
	}
	p.DatasetVersion = s.version
	return p, nil
}

// Nearby is the spatial proximity query, backed by $nearSphere.
func (s *Service) Nearby(
	ctx context.Context, lat, lng float64, radiusMeters, limit int,
) ([]domain.Place, error) {
	c := domain.Coordinate{Latitude: lat, Longitude: lng}
	if err := c.Validate(); err != nil {
		// Outside Ghana is a legitimate empty result, not an error (Spec 10);
		// a genuinely impossible coordinate is an error.
		if errors.Is(err, domain.ErrNotInGhana) {
			return []domain.Place{}, nil
		}
		return nil, apierr.Wrap(apierr.InvalidCoordinate, "Coordinates are out of range.", err)
	}
	const maxRadius = 50_000
	if radiusMeters <= 0 {
		radiusMeters = 5_000
	}
	if radiusMeters > maxRadius {
		return nil, apierr.
			New(apierr.RadiusOutOfRange, "Radius exceeds the maximum.").
			WithDetail("maxRadiusMeters", maxRadius)
	}
	out, err := s.places.Nearby(ctx, c, radiusMeters, limit)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Nearby search failed.", err)
	}
	return out, nil
}

// mapLookupErr turns a repository miss into either a redirect-aware 410 or a
// 404, so deprecated ids keep telling callers where the record went (Spec 18).
func (s *Service) mapLookupErr(ctx context.Context, kind, id string, err error) error {
	if !isNotFound(err) {
		return apierr.Wrap(apierr.Internal, "Lookup failed.", err)
	}
	if s.redirects != nil {
		if rd, rerr := s.redirects.Resolve(ctx, id); rerr == nil && rd != nil {
			return apierr.
				New(apierr.ResourceGone, kind+" was merged into another record.").
				WithDetail("mergedInto", rd.NewID).
				WithDetail("reason", rd.Reason)
		}
	}
	return notFound(kind, id)
}

func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
