package geography

import (
	"errors"
	"fmt"
	"strings"
)

// Roads and points of interest (Spec §4, story GEO-4.7).
//
// Both are derived from OpenStreetMap, which is ODbL. That licence is share-
// alike: a record derived from it carries obligations a CC BY record does not,
// so Attribution is a required field rather than a convenience — a road with
// no attribution cannot be published, and the type makes that checkable.

var (
	ErrNoAttribution = errors.New("a derived record must carry its source attribution")
	ErrNoName        = errors.New("name must not be empty")
	ErrUnknownClass  = errors.New("unknown road class")
)

// RoadClass is the functional classification, normalised from OSM's highway
// tag. Only classes worth publishing are modelled; a footpath is not a road a
// geocoder needs.
type RoadClass string

const (
	RoadMotorway     RoadClass = "MOTORWAY"
	RoadTrunk        RoadClass = "TRUNK"
	RoadPrimary      RoadClass = "PRIMARY"
	RoadSecondary    RoadClass = "SECONDARY"
	RoadTertiary     RoadClass = "TERTIARY"
	RoadResidential  RoadClass = "RESIDENTIAL"
	RoadUnclassified RoadClass = "UNCLASSIFIED"
	RoadTrack        RoadClass = "TRACK"
	RoadService      RoadClass = "SERVICE"
)

var roadClasses = map[RoadClass]struct{}{
	RoadMotorway: {}, RoadTrunk: {}, RoadPrimary: {}, RoadSecondary: {},
	RoadTertiary: {}, RoadResidential: {}, RoadUnclassified: {},
	RoadTrack: {}, RoadService: {},
}

// ParseRoadClass maps an OSM highway value onto a published class.
//
// The link variants (motorway_link and friends) are slip roads, and they
// collapse into their parent: a consumer asking "what road is this" does not
// want "the slip road onto the N1", and keeping them separate would double
// the class list for no gain.
func ParseRoadClass(osmHighway string) (RoadClass, bool) {
	v := strings.ToLower(strings.TrimSpace(osmHighway))
	v = strings.TrimSuffix(v, "_link")
	c := RoadClass(strings.ToUpper(v))
	if _, ok := roadClasses[c]; ok {
		return c, true
	}
	return "", false
}

// Road is a named way in the road network.
type Road struct {
	ID   string
	Name string
	// Ref is the route number where one exists — "N1", "R40". A Ghanaian
	// address is far more often given by road name, but the ref is what
	// appears on a sign.
	Ref        string
	Class      RoadClass
	RegionID   string
	RegionName string
	DistrictID string
	// Geometry is a LineString or MultiLineString.
	Geometry           *Geometry
	Status             Status
	VerificationStatus VerificationStatus
	Provenance         Provenance
	// Attribution is the licence notice that must travel with this record.
	Attribution    string
	DatasetVersion string
}

func (r Road) Validate() error {
	if strings.TrimSpace(r.Name) == "" && strings.TrimSpace(r.Ref) == "" {
		// A road with neither a name nor a route number cannot be referred
		// to, so it is not useful to publish.
		return fmt.Errorf("%w: a road needs a name or a ref", ErrNoName)
	}
	if _, ok := roadClasses[r.Class]; !ok {
		return fmt.Errorf("%w: %q", ErrUnknownClass, r.Class)
	}
	if strings.TrimSpace(r.Attribution) == "" {
		return ErrNoAttribution
	}
	if r.Geometry != nil {
		if err := r.Geometry.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// POIClass groups points of interest into the handful of categories a
// location API is actually asked about.
type POIClass string

const (
	POIEducation   POIClass = "EDUCATION"
	POIHealth      POIClass = "HEALTH"
	POIGovernment  POIClass = "GOVERNMENT"
	POIFinance     POIClass = "FINANCE"
	POITransport   POIClass = "TRANSPORT"
	POIWorship     POIClass = "WORSHIP"
	POICommerce    POIClass = "COMMERCE"
	POIHospitality POIClass = "HOSPITALITY"
	POILandmark    POIClass = "LANDMARK"
)

var poiClasses = map[POIClass]struct{}{
	POIEducation: {}, POIHealth: {}, POIGovernment: {}, POIFinance: {},
	POITransport: {}, POIWorship: {}, POICommerce: {}, POIHospitality: {},
	POILandmark: {},
}

// POI is a named point of interest.
type POI struct {
	ID    string
	Name  string
	Class POIClass
	// Category is the finer OSM value behind the class — "school",
	// "pharmacy", "bank". Kept because the class alone loses information a
	// consumer may want to filter on.
	Category           string
	RegionID           string
	RegionName         string
	DistrictID         string
	Centroid           *Coordinate
	Status             Status
	VerificationStatus VerificationStatus
	Provenance         Provenance
	Attribution        string
	DatasetVersion     string
}

func (p POI) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return ErrNoName
	}
	if _, ok := poiClasses[p.Class]; !ok {
		return fmt.Errorf("%w: %q", ErrUnknownClass, p.Class)
	}
	if strings.TrimSpace(p.Attribution) == "" {
		return ErrNoAttribution
	}
	if p.Centroid == nil {
		return fmt.Errorf("a point of interest needs a location")
	}
	return p.Centroid.Validate()
}

// ODbLAttribution is the notice every OpenStreetMap-derived record carries.
//
// ODbL is share-alike, so this is a licence obligation rather than a courtesy:
// it must appear on API responses and inside bulk downloads, not merely in a
// website footer that a downloaded file never sees.
const ODbLAttribution = "© OpenStreetMap contributors, licensed under the " +
	"Open Database License (ODbL). https://www.openstreetmap.org/copyright"
