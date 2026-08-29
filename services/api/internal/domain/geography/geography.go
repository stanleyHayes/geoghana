// Package geography holds the canonical domain model for Ghanaian geography.
//
// This package has ZERO infrastructure dependencies: no bson tags, no json tags,
// no driver imports. Persistence mapping lives in internal/adapters/mongo so the
// database representation can change without touching domain rules.
package geography

import (
	"errors"
	"fmt"
	"strings"
)

// Ghana's bounding box, used to reject implausible coordinates (Spec 22.1).
const (
	GhanaMinLat = 4.5
	GhanaMaxLat = 11.5
	GhanaMinLng = -3.5
	GhanaMaxLng = 1.5
)

var (
	ErrEmptyID          = errors.New("id must not be empty")
	ErrEmptyName        = errors.New("name must not be empty")
	ErrMissingRegion    = errors.New("district must reference a region")
	ErrLatOutOfRange    = errors.New("latitude out of valid range")
	ErrLngOutOfRange    = errors.New("longitude out of valid range")
	ErrNotInGhana       = errors.New("coordinate is outside Ghana's bounding box")
	ErrUnknownPlaceType = errors.New("unknown place type")
)

// Status is the lifecycle state of a canonical record.
type Status string

const (
	StatusActive     Status = "ACTIVE"
	StatusDeprecated Status = "DEPRECATED"
	StatusMerged     Status = "MERGED"
)

// VerificationStatus tracks how far a record has moved toward canonical truth.
// The seed CSVs ship REFERENCE and SEED_NEEDS_CANONICAL_RECONCILIATION rows;
// neither may be promoted to CANONICAL by an automated path (plan rule R5).
type VerificationStatus string

const (
	VerificationReference  VerificationStatus = "REFERENCE"
	VerificationNeedsRecon VerificationStatus = "SEED_NEEDS_CANONICAL_RECONCILIATION"
	VerificationReviewed   VerificationStatus = "REVIEWED"
	VerificationCanonical  VerificationStatus = "CANONICAL"
)

// PromotableToCanonical reports whether an automated process may publish this
// record as canonical. Seed rows never are (R5).
func (v VerificationStatus) PromotableToCanonical() bool {
	return v == VerificationReviewed || v == VerificationCanonical
}

// PlaceType enumerates human geography. Ghana is deliberately not modelled as
// country -> state -> city (Spec 3).
type PlaceType string

const (
	PlaceCity            PlaceType = "CITY"
	PlaceTown            PlaceType = "TOWN"
	PlaceVillage         PlaceType = "VILLAGE"
	PlaceCommunity       PlaceType = "COMMUNITY"
	PlaceSuburb          PlaceType = "SUBURB"
	PlaceNeighbourhood   PlaceType = "NEIGHBOURHOOD"
	PlaceHamlet          PlaceType = "HAMLET"
	PlaceSettlement      PlaceType = "SETTLEMENT"
	PlaceLocality        PlaceType = "LOCALITY"
	PlaceRegionalCapital PlaceType = "REGIONAL_CAPITAL"
)

var validPlaceTypes = map[PlaceType]struct{}{
	PlaceCity: {}, PlaceTown: {}, PlaceVillage: {}, PlaceCommunity: {},
	PlaceSuburb: {}, PlaceNeighbourhood: {}, PlaceHamlet: {}, PlaceSettlement: {},
	PlaceLocality: {}, PlaceRegionalCapital: {},
}

// ParsePlaceType validates a wire value into a PlaceType.
func ParsePlaceType(s string) (PlaceType, error) {
	t := PlaceType(strings.ToUpper(strings.TrimSpace(s)))
	if _, ok := validPlaceTypes[t]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownPlaceType, s)
	}
	return t, nil
}

// Coordinate is a WGS84 point. MongoDB accepts no other CRS for 2dsphere.
type Coordinate struct {
	Latitude  float64
	Longitude float64
}

// Validate enforces global coordinate ranges and Ghana plausibility (Spec 22.1).
func (c Coordinate) Validate() error {
	if c.Latitude < -90 || c.Latitude > 90 {
		return fmt.Errorf("%w: %v", ErrLatOutOfRange, c.Latitude)
	}
	if c.Longitude < -180 || c.Longitude > 180 {
		return fmt.Errorf("%w: %v", ErrLngOutOfRange, c.Longitude)
	}
	if c.Latitude < GhanaMinLat || c.Latitude > GhanaMaxLat ||
		c.Longitude < GhanaMinLng || c.Longitude > GhanaMaxLng {
		return fmt.Errorf("%w: (%v, %v)", ErrNotInGhana, c.Latitude, c.Longitude)
	}
	return nil
}

// Provenance records where a record came from. A record without it cannot be
// published (plan rule R2).
type Provenance struct {
	SourceID          string
	ExternalID        string
	SourceURL         string
	RetrievedAt       string
	SourcePayloadHash string
	Notes             string
}

// Region is a first-level administrative area. Ghana has 16.
type Region struct {
	ID                 string
	CountryCode        string
	Name               string
	Capital            string
	OfficialCode       string
	Status             Status
	VerificationStatus VerificationStatus
	Centroid           *Coordinate
	Geometry           *Geometry
	Provenance         Provenance
	DatasetVersion     string
}

func (r Region) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return ErrEmptyID
	}
	if strings.TrimSpace(r.Name) == "" {
		return ErrEmptyName
	}
	if r.Centroid != nil {
		if err := r.Centroid.Validate(); err != nil {
			return fmt.Errorf("region %s centroid: %w", r.ID, err)
		}
	}
	return nil
}

// District is a second-level administrative area (MMDA). Ghana has 261 in the seed.
type District struct {
	ID                 string
	RegionID           string
	RegionName         string
	Name               string
	DistrictType       string
	OfficialCode       string
	Capital            string
	Status             Status
	VerificationStatus VerificationStatus
	Centroid           *Coordinate
	Geometry           *Geometry
	Provenance         Provenance
	DatasetVersion     string
}

func (d District) Validate() error {
	if strings.TrimSpace(d.ID) == "" {
		return ErrEmptyID
	}
	if strings.TrimSpace(d.Name) == "" {
		return ErrEmptyName
	}
	if strings.TrimSpace(d.RegionID) == "" {
		return fmt.Errorf("%w: district %s", ErrMissingRegion, d.ID)
	}
	if d.Centroid != nil {
		if err := d.Centroid.Validate(); err != nil {
			return fmt.Errorf("district %s centroid: %w", d.ID, err)
		}
	}
	return nil
}

// Alias is an alternative name for a place: colloquial, abbreviated or
// in another Ghanaian language.
type Alias struct {
	Value           string
	NormalizedValue string
	AliasType       string
	Language        string
	IsPreferred     bool
}

// Place is a locality in human geography.
type Place struct {
	ID                 string
	Name               string
	NormalizedName     string
	Type               PlaceType
	RegionID           string
	RegionName         string
	DistrictID         string
	DistrictName       string
	ParentPlaceID      string
	Aliases            []Alias
	Centroid           *Coordinate
	Geometry           *Geometry
	Population         *int64
	Status             Status
	VerificationStatus VerificationStatus
	Provenance         Provenance
	DatasetVersion     string
}

func (p Place) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return ErrEmptyID
	}
	if strings.TrimSpace(p.Name) == "" {
		return ErrEmptyName
	}
	if _, ok := validPlaceTypes[p.Type]; !ok {
		return fmt.Errorf("%w: place %s has %q", ErrUnknownPlaceType, p.ID, p.Type)
	}
	if p.Centroid != nil {
		if err := p.Centroid.Validate(); err != nil {
			return fmt.Errorf("place %s centroid: %w", p.ID, err)
		}
	}
	return nil
}

// Redirect preserves a merged or deprecated ID so old identifiers keep
// resolving instead of 404ing (Spec 18, plan GEO-3.3).
type Redirect struct {
	OldID    string
	NewID    string
	Reason   string
	MergedAt string
}
