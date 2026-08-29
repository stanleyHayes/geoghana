package geography

import (
	"errors"
	"testing"
)

// square returns a valid closed CCW ring.
func square() Ring {
	return Ring{{-0.3, 5.5}, {-0.1, 5.5}, {-0.1, 5.7}, {-0.3, 5.7}, {-0.3, 5.5}}
}

func polygon(rings ...Ring) *Geometry {
	coords := make([][][]float64, 0, len(rings))
	for _, r := range rings {
		ring := make([][]float64, 0, len(r))
		for _, p := range r {
			ring = append(ring, []float64{p[0], p[1]})
		}
		coords = append(coords, ring)
	}
	return &Geometry{Type: GeomPolygon, Coordinates: coords}
}

// TestKnownBadPolygonsAreRejected is the gate that replaces PostGIS ST_IsValid.
// MongoDB would accept several of these and then return silently wrong
// $geoIntersects results (risk RK-17).
func TestKnownBadPolygonsAreRejected(t *testing.T) {
	cases := []struct {
		name string
		ring Ring
		want error
	}{
		{
			name: "self-intersecting bowtie",
			ring: Ring{{-0.3, 5.5}, {-0.1, 5.7}, {-0.1, 5.5}, {-0.3, 5.7}, {-0.3, 5.5}},
			want: ErrGeomSelfIntersect,
		},
		{
			name: "unclosed ring",
			ring: Ring{{-0.3, 5.5}, {-0.1, 5.5}, {-0.1, 5.7}, {-0.3, 5.7}},
			want: ErrGeomRingNotClosed,
		},
		{
			name: "too few positions",
			ring: Ring{{-0.3, 5.5}, {-0.1, 5.5}, {-0.3, 5.5}},
			want: ErrGeomRingTooShort,
		},
		{
			name: "latitude out of range",
			ring: Ring{{-0.3, 95.0}, {-0.1, 95.0}, {-0.1, 96.0}, {-0.3, 96.0}, {-0.3, 95.0}},
			want: ErrLatOutOfRange,
		},
		{
			name: "longitude out of range",
			ring: Ring{{-190.0, 5.5}, {-189.0, 5.5}, {-189.0, 5.7}, {-190.0, 5.7}, {-190.0, 5.5}},
			want: ErrLngOutOfRange,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := polygon(tc.ring).Validate()
			if err == nil {
				t.Fatalf("expected %v, got nil — this geometry would corrupt $geoIntersects", tc.want)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestValidPolygonAccepted(t *testing.T) {
	if err := polygon(square()).Validate(); err != nil {
		t.Fatalf("valid square rejected: %v", err)
	}
}

func TestPolygonWithHoleAccepted(t *testing.T) {
	hole := Ring{{-0.25, 5.55}, {-0.15, 5.55}, {-0.15, 5.65}, {-0.25, 5.65}, {-0.25, 5.55}}
	if err := polygon(square(), hole).Validate(); err != nil {
		t.Fatalf("polygon with hole rejected: %v", err)
	}
}

// TestNormalizeWinding proves exterior rings end up counter-clockwise and holes
// clockwise. MongoDB reads a >hemisphere polygon by winding order, so a
// backwards exterior ring means "everywhere except this district".
func TestNormalizeWinding(t *testing.T) {
	cw := reverseRing(square()) // clockwise exterior
	if SignedArea(cw) > 0 {
		t.Fatal("fixture is not clockwise")
	}
	hole := Ring{{-0.25, 5.55}, {-0.15, 5.55}, {-0.15, 5.65}, {-0.25, 5.65}, {-0.25, 5.55}}

	out := NormalizeWinding([]Ring{cw, hole})
	if SignedArea(out[0]) <= 0 {
		t.Error("exterior ring should be counter-clockwise after normalization")
	}
	if SignedArea(out[1]) >= 0 {
		t.Error("hole ring should be clockwise after normalization")
	}
}

func TestCoordinateValidation(t *testing.T) {
	cases := []struct {
		name string
		c    Coordinate
		ok   bool
	}{
		{"Accra", Coordinate{Latitude: 5.556, Longitude: -0.182}, true},
		{"Kumasi", Coordinate{Latitude: 6.688, Longitude: -1.624}, true},
		{"Tamale", Coordinate{Latitude: 9.407, Longitude: -0.853}, true},
		{"null island is not in Ghana", Coordinate{Latitude: 0, Longitude: 0}, false},
		{"London", Coordinate{Latitude: 51.5, Longitude: -0.12}, false},
		{"lat/lng swapped", Coordinate{Latitude: -0.182, Longitude: 5.556}, false},
		{"impossible latitude", Coordinate{Latitude: 100, Longitude: 0}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if tc.ok && err != nil {
				t.Fatalf("expected valid, got %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected rejection, got nil")
			}
		})
	}
}

func TestSeedVerificationStatusNeverAutoPromotes(t *testing.T) {
	// Plan rule R5: seed rows must never reach canonical by an automated path.
	if VerificationNeedsRecon.PromotableToCanonical() {
		t.Error("SEED_NEEDS_CANONICAL_RECONCILIATION must not be auto-promotable")
	}
	if VerificationReference.PromotableToCanonical() {
		t.Error("REFERENCE must not be auto-promotable")
	}
	if !VerificationReviewed.PromotableToCanonical() {
		t.Error("REVIEWED should be promotable")
	}
}

func TestPlaceTypeParsing(t *testing.T) {
	if _, err := ParsePlaceType("suburb"); err != nil {
		t.Fatalf("lowercase should parse: %v", err)
	}
	if _, err := ParsePlaceType("CITY_STATE"); !errors.Is(err, ErrUnknownPlaceType) {
		t.Fatalf("expected ErrUnknownPlaceType, got %v", err)
	}
}

func TestDistrictRequiresRegion(t *testing.T) {
	d := District{ID: "gh-district-x", Name: "X"}
	if err := d.Validate(); !errors.Is(err, ErrMissingRegion) {
		t.Fatalf("expected ErrMissingRegion, got %v", err)
	}
}
