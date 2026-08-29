package mongo

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// Regression: the driver decodes a BSON array into bson.A, a NAMED type. A
// type switch on []any does not match it, so every stored coordinate decoded
// to nil — the database held 15,925 correct centroids while the API returned
// none of them.
func TestCoordinateDecodesFromDriverTypes(t *testing.T) {
	want := geography.Coordinate{Latitude: 5.55602, Longitude: -0.1969}

	cases := map[string]any{
		"bson.A (what the driver actually returns)": bson.A{-0.1969, 5.55602},
		"[]any":                    []any{-0.1969, 5.55602},
		"[]float64":                []float64{-0.1969, 5.55602},
		"bson.A with int32 values": bson.A{int32(-1), int32(6)},
	}

	for name, coords := range cases {
		t.Run(name, func(t *testing.T) {
			got := coordOf(&geoJSON{Type: "Point", Coordinates: coords})
			if got == nil {
				t.Fatal("coordinate decoded to nil — the API would report no location")
			}
			if name == "bson.A with int32 values" {
				if got.Longitude != -1 || got.Latitude != 6 {
					t.Errorf("integer coordinates mis-decoded: %+v", got)
				}
				return
			}
			if got.Latitude != want.Latitude || got.Longitude != want.Longitude {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}
}

func TestGeographyMetadataBackfillIsCompleteAndIdempotent(t *testing.T) {
	doc := bson.M{
		"_id": "gh-place-gn-2306104", "name": "Accra",
		"datasetVersion": "2026.08.2-seed",
		"provenance":     bson.M{"sourceId": "geonames", "externalId": "2306104"},
	}
	set, err := geographyMetadataDefaults(doc, "2026-08-29T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if set["provenance.retrievedAt"] == "" {
		t.Error("retrieval time was not backfilled")
	}
	hash, _ := set["provenance.sourcePayloadHash"].(string)
	if len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		t.Fatalf("payload hash is not lowercase SHA-256: %q", hash)
	}

	complete := bson.M{
		"_id": "gh-place-gn-2306104", "datasetVersion": "2026.08.2-seed",
		"provenance": bson.M{
			"sourceId": "geonames", "externalId": "2306104",
			"retrievedAt": "2026-08-29T12:00:00Z", "sourcePayloadHash": hash,
		},
	}
	if again, err := geographyMetadataDefaults(complete, time.Now().UTC().Format(time.RFC3339)); err != nil || len(again) != 0 {
		t.Fatalf("second backfill changed complete metadata: set=%v err=%v", again, err)
	}
}

func TestMergeUpsertOmitsAbsentEnrichment(t *testing.T) {
	fields, err := mergeFields(regionDoc{ID: "gh-region-x", Name: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := fields["_id"]; exists {
		t.Fatal("immutable _id was included in $set")
	}
	for _, preserved := range []string{"geometry", "centroid", "capital", "officialCode"} {
		if _, exists := fields[preserved]; exists {
			t.Errorf("absent %s would erase existing enrichment", preserved)
		}
	}
	if fields["name"] != "X" {
		t.Errorf("present source fields are not updated: %v", fields)
	}
}

// GeoJSON position order is [longitude, latitude] — the reverse of how humans
// say it. Getting this backwards puts Accra in the Gulf of Guinea.
func TestPositionOrderIsLongitudeFirst(t *testing.T) {
	accra := geography.Coordinate{Latitude: 5.556, Longitude: -0.182}
	encoded := pointOf(&accra)
	coords, ok := encoded.Coordinates.([]float64)
	if !ok || len(coords) != 2 {
		t.Fatalf("unexpected encoding: %#v", encoded.Coordinates)
	}
	if coords[0] != accra.Longitude || coords[1] != accra.Latitude {
		t.Errorf("encoded as [%v, %v], want [longitude, latitude] = [%v, %v]",
			coords[0], coords[1], accra.Longitude, accra.Latitude)
	}
	// And it must survive the round trip.
	back := coordOf(encoded)
	if back == nil || *back != accra {
		t.Errorf("round trip lost the coordinate: %+v", back)
	}
}

func TestNilAndNonPointGeometryDecodeToNil(t *testing.T) {
	if coordOf(nil) != nil {
		t.Error("nil geometry should decode to nil")
	}
	if coordOf(&geoJSON{Type: "Polygon", Coordinates: bson.A{}}) != nil {
		t.Error("a polygon is not a centroid and must not decode to one")
	}
	if coordOf(&geoJSON{Type: "Point", Coordinates: bson.A{1.0}}) != nil {
		t.Error("a one-element position is malformed and must decode to nil")
	}
}

// Geometry must survive a full round trip for every entity that can carry one.
//
// It previously did not: geomOf mapped the write direction, but no from*Doc
// read it back, so boundaries were stored and then silently dropped on every
// read. Nothing failed loudly — /boundaries/{id} queried the collection
// directly and masked it, and the loss only surfaced as a GeoJSON export in
// which every feature had null geometry.
func TestGeometrySurvivesRoundTrip(t *testing.T) {
	poly := &geography.Geometry{
		Type: "Polygon",
		Coordinates: []any{[]any{
			[]any{-0.2, 5.5}, []any{-0.1, 5.5}, []any{-0.1, 5.6},
			[]any{-0.2, 5.6}, []any{-0.2, 5.5},
		}},
	}

	t.Run("region", func(t *testing.T) {
		got := fromRegionDoc(toRegionDoc(geography.Region{
			ID: "gh-region-x", Name: "X", Geometry: poly,
		}))
		if got.Geometry == nil {
			t.Fatal("region geometry dropped on read")
		}
		if got.Geometry.Type != "Polygon" {
			t.Errorf("region geometry type = %q", got.Geometry.Type)
		}
	})

	t.Run("district", func(t *testing.T) {
		got := fromDistrictDoc(toDistrictDoc(geography.District{
			ID: "gh-district-x", Name: "X", Geometry: poly,
		}))
		if got.Geometry == nil {
			t.Fatal("district geometry dropped on read")
		}
	})

	t.Run("place", func(t *testing.T) {
		got := fromPlaceDoc(toPlaceDoc(geography.Place{
			ID: "gh-place-x", Name: "X", Geometry: poly,
		}))
		if got.Geometry == nil {
			t.Fatal("place geometry dropped on read")
		}
	})

	t.Run("nil stays nil", func(t *testing.T) {
		// A record with no boundary must not gain an empty one.
		if got := fromRegionDoc(toRegionDoc(geography.Region{ID: "r"})); got.Geometry != nil {
			t.Errorf("absent geometry became %+v", got.Geometry)
		}
	})
}
