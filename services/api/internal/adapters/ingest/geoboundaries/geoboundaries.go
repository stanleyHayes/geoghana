// Package geoboundaries ingests administrative boundary polygons.
//
// LICENCE: CC BY 4.0 (geoBoundaries, gbOpen release). Attribution travels with
// every record (plan rule R4).
//
// GADM was rejected despite being the better-known source: its licence permits
// academic use only and forbids redistribution, which would make our bulk
// downloads unlawful. A licence that does not survive contact with the product
// is not a usable licence.
package geoboundaries

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ghanageo/ghanageo/services/api/internal/app/ingest"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// Level is the administrative depth of a boundary file.
type Level string

const (
	ADM1 Level = "ADM1" // regions
	ADM2 Level = "ADM2" // districts / MMDAs
)

// Feature is one GeoJSON feature from a geoBoundaries release.
type Feature struct {
	Type       string `json:"type"`
	Properties struct {
		ShapeName  string `json:"shapeName"`
		ShapeISO   string `json:"shapeISO"`
		ShapeID    string `json:"shapeID"`
		ShapeGroup string `json:"shapeGroup"`
		ShapeType  string `json:"shapeType"`
	} `json:"properties"`
	Geometry json.RawMessage `json:"geometry"`
}

type collection struct {
	Type     string    `json:"type"`
	Features []Feature `json:"features"`
}

func Licence() ingest.Licence {
	return ingest.Licence{
		SPDX:            "CC-BY-4.0",
		Name:            "Creative Commons Attribution 4.0",
		URL:             "https://creativecommons.org/licenses/by/4.0/",
		Attribution:     "Boundaries from geoBoundaries (https://www.geoboundaries.org), licensed CC BY 4.0.",
		Redistributable: true,
	}
}

// Load reads a geoBoundaries GeoJSON file.
//
// The whole file is read into memory deliberately: the Ghana ADM2 release is
// about 25 MB, which is fine, and streaming GeoJSON correctly is far more
// code than the size justifies. If a country-scale file ever needs this, it
// should stream rather than grow the buffer.
func Load(path string) ([]Feature, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c collection
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(c.Features) == 0 {
		return nil, fmt.Errorf("%s contains no features", path)
	}
	return c.Features, nil
}

// ToGeometry converts a feature's raw GeoJSON geometry into the domain type,
// validating it on the way. MongoDB will accept a self-intersecting polygon
// and then return silently wrong $geoIntersects results, so validation here is
// a blocking gate, not a formality (plan GEO-5.3).
func ToGeometry(raw json.RawMessage) (*geography.Geometry, error) {
	var g struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, fmt.Errorf("parse geometry: %w", err)
	}

	geom := &geography.Geometry{Type: geography.GeometryType(g.Type)}

	switch geom.Type {
	case geography.GeomPolygon:
		var coords [][][]float64
		if err := json.Unmarshal(g.Coordinates, &coords); err != nil {
			return nil, fmt.Errorf("parse polygon coordinates: %w", err)
		}
		geom.Coordinates = coords
	case geography.GeomMultiPolygon:
		var coords [][][][]float64
		if err := json.Unmarshal(g.Coordinates, &coords); err != nil {
			return nil, fmt.Errorf("parse multipolygon coordinates: %w", err)
		}
		geom.Coordinates = coords
	default:
		return nil, fmt.Errorf("unsupported boundary geometry %q", g.Type)
	}

	if err := geom.Validate(); err != nil {
		return nil, fmt.Errorf("invalid geometry: %w", err)
	}
	return geom, nil
}

// NormalizeWinding enforces the right-hand rule on a polygon's rings.
//
// MongoDB interprets a polygon larger than a hemisphere by its winding order.
// A backwards exterior ring turns "this district" into "everywhere on Earth
// except this district", which would silently break every containment query
// rather than failing loudly.
func NormalizeWinding(g *geography.Geometry) {
	switch g.Type {
	case geography.GeomPolygon:
		coords, ok := g.Coordinates.([][][]float64)
		if !ok {
			return
		}
		g.Coordinates = ringsToCoords(geography.NormalizeWinding(coordsToRings(coords)))
	case geography.GeomMultiPolygon:
		polys, ok := g.Coordinates.([][][][]float64)
		if !ok {
			return
		}
		out := make([][][][]float64, len(polys))
		for i, p := range polys {
			out[i] = ringsToCoords(geography.NormalizeWinding(coordsToRings(p)))
		}
		g.Coordinates = out
	}
}

func coordsToRings(c [][][]float64) []geography.Ring {
	out := make([]geography.Ring, 0, len(c))
	for _, ring := range c {
		r := make(geography.Ring, 0, len(ring))
		for _, p := range ring {
			if len(p) >= 2 {
				r = append(r, [2]float64{p[0], p[1]})
			}
		}
		out = append(out, r)
	}
	return out
}

func ringsToCoords(rs []geography.Ring) [][][]float64 {
	out := make([][][]float64, 0, len(rs))
	for _, r := range rs {
		ring := make([][]float64, 0, len(r))
		for _, p := range r {
			ring = append(ring, []float64{p[0], p[1]})
		}
		out = append(out, ring)
	}
	return out
}
