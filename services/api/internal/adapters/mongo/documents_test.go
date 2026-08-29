package mongo

import (
	"testing"

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
