package rest

import (
	"context"
	"net/http"
	"regexp"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	geo "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Roads and points of interest (story GEO-4.7).
//
// Both are OpenStreetMap-derived, so every response carries the ODbL notice.
// ODbL is share-alike: the attribution is a licence obligation that travels
// with the data, not a footer on a website the consumer never sees.

type roadJSON struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Ref      string   `json:"ref,omitempty"`
	Class    string   `json:"class"`
	District *refJSON `json:"district,omitempty"`
	Region   *refJSON `json:"region,omitempty"`
	Geometry any      `json:"geometry,omitempty"`
}

type poiJSON struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Class    string     `json:"class"`
	Category string     `json:"category,omitempty"`
	District *refJSON   `json:"district,omitempty"`
	Region   *refJSON   `json:"region,omitempty"`
	Centroid *coordJSON `json:"centroid,omitempty"`
}

type refJSON struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type coordJSON struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func clampListLimit(r *http.Request, def, max int) int {
	n, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func (h *Handler) listRoads(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Road data is not configured."))
		return
	}
	q := bson.M{}
	if name := r.URL.Query().Get("q"); name != "" {
		// Prefix match on the normalized name, which the index covers.
		q["normalizedName"] = bson.M{"$regex": "^" + regexEscape(normalizeQuery(name))}
	}
	if c := r.URL.Query().Get("class"); c != "" {
		q["class"] = c
	}
	if d := r.URL.Query().Get("districtId"); d != "" {
		q["districtId"] = d
	}
	// Geometry is excluded unless asked for: a road linestring is large and
	// almost no list caller wants one, and fetching it dominated the query.
	proj := bson.M{"geometry": 0}
	if r.URL.Query().Get("geometry") == "true" {
		proj = bson.M{}
	}

	cur, err := h.store.DB().Collection(mongoadapter.ColRoads).Find(r.Context(), q,
		options.Find().SetLimit(int64(clampListLimit(r, 25, 100))).
			SetSort(bson.D{{Key: "name", Value: 1}}).SetProjection(proj))
	if err != nil {
		writeErr(w, r, apierr.Wrap(apierr.Internal, "Could not list roads.", err))
		return
	}
	defer func() { _ = cur.Close(r.Context()) }()

	out := make([]roadJSON, 0, 25)
	for cur.Next(r.Context()) {
		var d struct {
			ID         string `bson:"_id"`
			Name       string `bson:"name"`
			Ref        string `bson:"ref"`
			Class      string `bson:"class"`
			DistrictID string `bson:"districtId"`
			RegionID   string `bson:"regionId"`
			RegionName string `bson:"regionName"`
			Geometry   any    `bson:"geometry"`
		}
		if err := cur.Decode(&d); err != nil {
			continue
		}
		out = append(out, roadJSON{
			ID: d.ID, Name: d.Name, Ref: d.Ref, Class: d.Class,
			District: refOrNil(d.DistrictID, ""), Region: refOrNil(d.RegionID, d.RegionName),
			Geometry: d.Geometry,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": out, "attribution": geo.ODbLAttribution, "license": "ODbL-1.0",
	})
}

func (h *Handler) listPOIs(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Point-of-interest data is not configured."))
		return
	}
	q := bson.M{}
	if name := r.URL.Query().Get("q"); name != "" {
		q["normalizedName"] = bson.M{"$regex": "^" + regexEscape(normalizeQuery(name))}
	}
	if c := r.URL.Query().Get("class"); c != "" {
		q["class"] = c
	}
	if d := r.URL.Query().Get("districtId"); d != "" {
		q["districtId"] = d
	}

	// A spatial query when a point is given, which is what a POI lookup is
	// usually for.
	lat, latErr := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, lngErr := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	if latErr == nil && lngErr == nil {
		radius := clampListLimit(r, 5000, 50000)
		if v, err := strconv.Atoi(r.URL.Query().Get("radius")); err == nil && v > 0 {
			radius = v
			if radius > 50000 {
				writeErr(w, r, apierr.New(apierr.RadiusOutOfRange, "Radius exceeds the maximum.").
					WithDetail("maxRadiusMeters", 50000))
				return
			}
		}
		q["centroid"] = bson.M{"$nearSphere": bson.M{
			"$geometry":    bson.M{"type": "Point", "coordinates": []float64{lng, lat}},
			"$maxDistance": radius,
		}}
	}

	cur, err := h.store.DB().Collection(mongoadapter.ColPOIs).Find(r.Context(), q,
		options.Find().SetLimit(int64(clampListLimit(r, 25, 100))))
	if err != nil {
		writeErr(w, r, apierr.Wrap(apierr.Internal, "Could not list points of interest.", err))
		return
	}
	defer func() { _ = cur.Close(r.Context()) }()

	out := make([]poiJSON, 0, 25)
	for cur.Next(r.Context()) {
		var d struct {
			ID         string `bson:"_id"`
			Name       string `bson:"name"`
			Class      string `bson:"class"`
			Category   string `bson:"category"`
			DistrictID string `bson:"districtId"`
			RegionID   string `bson:"regionId"`
			RegionName string `bson:"regionName"`
			Centroid   *struct {
				Coordinates []float64 `bson:"coordinates"`
			} `bson:"centroid"`
		}
		if err := cur.Decode(&d); err != nil {
			continue
		}
		item := poiJSON{
			ID: d.ID, Name: d.Name, Class: d.Class, Category: d.Category,
			District: refOrNil(d.DistrictID, ""), Region: refOrNil(d.RegionID, d.RegionName),
		}
		if d.Centroid != nil && len(d.Centroid.Coordinates) == 2 {
			// Stored as GeoJSON [longitude, latitude]; emitted as named
			// fields so the order cannot be got wrong.
			item.Centroid = &coordJSON{
				Longitude: d.Centroid.Coordinates[0], Latitude: d.Centroid.Coordinates[1],
			}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": out, "attribution": geo.ODbLAttribution, "license": "ODbL-1.0",
	})
}

// normalizeQuery folds a query the same way names are stored, so a prefix
// match compares like with like. Without it, "Osu" would never match the
// stored "osu".
func normalizeQuery(s string) string { return normalize.Name(s) }

// regexEscape neutralises regex metacharacters in user input.
//
// The query goes into a $regex, so an unescaped "(" is a syntax error and a
// crafted ".*" would scan the whole collection — a cheap denial of service
// against an endpoint anyone can call anonymously.
func regexEscape(s string) string { return regexp.QuoteMeta(s) }

func refOrNil(id, name string) *refJSON {
	if id == "" {
		return nil
	}
	return &refJSON{ID: id, Name: name}
}

var _ = context.Background
