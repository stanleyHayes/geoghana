package geography

import (
	"errors"
	"testing"
)

// ODbL is share-alike, so attribution is a licence obligation, not a nicety.
// A derived record without it must not validate — that is the only way the
// obligation is enforceable rather than merely documented.
func TestDerivedRecordsRequireAttribution(t *testing.T) {
	road := Road{Name: "Independence Avenue", Class: RoadPrimary, Attribution: ODbLAttribution}
	if err := road.Validate(); err != nil {
		t.Fatalf("a complete road was rejected: %v", err)
	}
	road.Attribution = ""
	if err := road.Validate(); !errors.Is(err, ErrNoAttribution) {
		t.Errorf("a road with no attribution validated: %v", err)
	}

	poi := POI{
		Name: "Korle Bu Teaching Hospital", Class: POIHealth,
		Centroid:    &Coordinate{Latitude: 5.5366, Longitude: -0.2262},
		Attribution: ODbLAttribution,
	}
	if err := poi.Validate(); err != nil {
		t.Fatalf("a complete POI was rejected: %v", err)
	}
	poi.Attribution = ""
	if err := poi.Validate(); !errors.Is(err, ErrNoAttribution) {
		t.Errorf("a POI with no attribution validated: %v", err)
	}
}

// A road with neither a name nor a route number cannot be referred to, so
// publishing it would add noise a consumer can never act on.
func TestRoadNeedsANameOrARef(t *testing.T) {
	r := Road{Class: RoadPrimary, Attribution: ODbLAttribution}
	if err := r.Validate(); !errors.Is(err, ErrNoName) {
		t.Errorf("an unnamed, unnumbered road validated: %v", err)
	}
	r.Ref = "N1"
	if err := r.Validate(); err != nil {
		t.Errorf("a road identified only by its ref was rejected: %v", err)
	}
}

// Slip roads collapse into their parent class: a consumer asking what road
// they are on does not want "the slip road onto the N1".
func TestParseRoadClassCollapsesLinks(t *testing.T) {
	cases := map[string]RoadClass{
		"motorway":      RoadMotorway,
		"motorway_link": RoadMotorway,
		"trunk_link":    RoadTrunk,
		"primary":       RoadPrimary,
		"PRIMARY_LINK":  RoadPrimary,
		"residential":   RoadResidential,
		"unclassified":  RoadUnclassified,
	}
	for in, want := range cases {
		got, ok := ParseRoadClass(in)
		if !ok || got != want {
			t.Errorf("ParseRoadClass(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}

	// Everything else is deliberately not published. A footpath or a driveway
	// is not a road a geocoder needs, and importing them would multiply the
	// dataset for no benefit.
	for _, skip := range []string{"footway", "path", "cycleway", "steps", "", "corridor"} {
		if _, ok := ParseRoadClass(skip); ok {
			t.Errorf("ParseRoadClass(%q) was accepted; it should not be published", skip)
		}
	}
}

func TestPOINeedsALocation(t *testing.T) {
	p := POI{Name: "Somewhere", Class: POILandmark, Attribution: ODbLAttribution}
	if err := p.Validate(); err == nil {
		t.Error("a POI with no coordinates validated")
	}
	// And a coordinate outside Ghana is rejected by the shared validator, so
	// an extract that strays over a border cannot import silently.
	p.Centroid = &Coordinate{Latitude: 51.5, Longitude: -0.12}
	if err := p.Validate(); err == nil {
		t.Error("a POI in London validated as Ghanaian geography")
	}
}

func TestUnknownClassesAreRejected(t *testing.T) {
	if err := (Road{Name: "X", Class: "SOMETHING", Attribution: ODbLAttribution}).Validate(); !errors.Is(err, ErrUnknownClass) {
		t.Error("an unknown road class validated")
	}
	if err := (POI{
		Name: "X", Class: "SOMETHING", Attribution: ODbLAttribution,
		Centroid: &Coordinate{Latitude: 5.6, Longitude: -0.18},
	}).Validate(); !errors.Is(err, ErrUnknownClass) {
		t.Error("an unknown POI class validated")
	}
}
