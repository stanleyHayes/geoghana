package geography

import (
	"errors"
	"fmt"
	"math"
)

// Geometry is a GeoJSON geometry in WGS84 (EPSG:4326) — the only CRS MongoDB
// accepts for 2dsphere indexes.
//
// WHY THIS FILE IS LARGER THAN IT LOOKS LIKE IT SHOULD BE:
//
// PostGIS would have given us ST_IsValid and ST_MakeValid. MongoDB has no
// equivalent. It rejects some malformed GeoJSON on insert but performs no
// validity checking and offers no repair, and a self-intersecting polygon that
// Mongo happily stores will silently return WRONG $geoIntersects results —
// meaning a reverse-geocode would place a town in the wrong district with no
// error anywhere. Validation therefore lives here and is a BLOCKING gate before
// any write to a canonical collection (plan GEO-5.3, risk RK-17).
type Geometry struct {
	Type        GeometryType
	Coordinates any // []float64 | [][]float64 | [][][]float64 | [][][][]float64
}

type GeometryType string

const (
	GeomPoint           GeometryType = "Point"
	GeomLineString      GeometryType = "LineString"
	GeomPolygon         GeometryType = "Polygon"
	GeomMultiPolygon    GeometryType = "MultiPolygon"
	GeomMultiPoint      GeometryType = "MultiPoint"
	GeomMultiLineString GeometryType = "MultiLineString"
)

var (
	ErrGeomUnknownType   = errors.New("unknown geometry type")
	ErrGeomRingTooShort  = errors.New("linear ring needs at least 4 positions")
	ErrGeomRingNotClosed = errors.New("linear ring is not closed (first position must equal last)")
	ErrGeomSelfIntersect = errors.New("polygon ring is self-intersecting")
	ErrGeomNoRings       = errors.New("polygon needs at least one ring")
	ErrGeomBadPosition   = errors.New("position must be [longitude, latitude]")
	ErrGeomEmpty         = errors.New("geometry has no coordinates")
)

// Ring is a closed sequence of positions. Position order is GeoJSON's
// [longitude, latitude] — the reverse of how humans say it, and a classic
// source of points landing in the Gulf of Guinea.
type Ring [][2]float64

// Validate runs every check that PostGIS would have run for us.
func (g *Geometry) Validate() error {
	if g == nil {
		return nil
	}
	switch g.Type {
	case GeomPoint:
		p, err := asPosition(g.Coordinates)
		if err != nil {
			return err
		}
		return validatePosition(p)
	case GeomLineString, GeomMultiPoint:
		line, err := asRing(g.Coordinates)
		if err != nil {
			return err
		}
		if len(line) < 2 {
			return fmt.Errorf("%s: %w", g.Type, ErrGeomEmpty)
		}
		return validateRingPositions(line)
	case GeomPolygon:
		rings, err := asRings(g.Coordinates)
		if err != nil {
			return err
		}
		return validatePolygon(rings)
	case GeomMultiPolygon:
		polys, err := asPolygons(g.Coordinates)
		if err != nil {
			return err
		}
		if len(polys) == 0 {
			return ErrGeomEmpty
		}
		for i, rings := range polys {
			if err := validatePolygon(rings); err != nil {
				return fmt.Errorf("polygon %d: %w", i, err)
			}
		}
		return nil
	case GeomMultiLineString:
		lines, err := asRings(g.Coordinates)
		if err != nil {
			return err
		}
		for i, l := range lines {
			if len(l) < 2 {
				return fmt.Errorf("line %d: %w", i, ErrGeomEmpty)
			}
			if err := validateRingPositions(l); err != nil {
				return fmt.Errorf("line %d: %w", i, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrGeomUnknownType, g.Type)
	}
}

func validatePolygon(rings []Ring) error {
	if len(rings) == 0 {
		return ErrGeomNoRings
	}
	for i, r := range rings {
		if len(r) < 4 {
			return fmt.Errorf("ring %d: %w (has %d)", i, ErrGeomRingTooShort, len(r))
		}
		if r[0] != r[len(r)-1] {
			return fmt.Errorf("ring %d: %w", i, ErrGeomRingNotClosed)
		}
		if err := validateRingPositions(r); err != nil {
			return fmt.Errorf("ring %d: %w", i, err)
		}
		if selfIntersects(r) {
			return fmt.Errorf("ring %d: %w", i, ErrGeomSelfIntersect)
		}
	}
	return nil
}

func validateRingPositions(r Ring) error {
	for _, p := range r {
		if err := validatePosition(p); err != nil {
			return err
		}
	}
	return nil
}

func validatePosition(p [2]float64) error {
	lng, lat := p[0], p[1]
	if math.IsNaN(lng) || math.IsNaN(lat) || math.IsInf(lng, 0) || math.IsInf(lat, 0) {
		return fmt.Errorf("%w: non-finite", ErrGeomBadPosition)
	}
	if lat < -90 || lat > 90 {
		return fmt.Errorf("%w: %v", ErrLatOutOfRange, lat)
	}
	if lng < -180 || lng > 180 {
		return fmt.Errorf("%w: %v", ErrLngOutOfRange, lng)
	}
	return nil
}

// selfIntersects reports whether any two non-adjacent segments of a closed ring
// cross. O(n^2); boundary rings in this dataset are small enough that the
// simplicity is worth more than the speed, and this runs at ingest, not per request.
func selfIntersects(r Ring) bool {
	n := len(r) - 1 // last position repeats the first
	if n < 4 {
		return false
	}
	for i := 0; i < n; i++ {
		a1, a2 := r[i], r[i+1]
		for j := i + 1; j < n; j++ {
			// Skip adjacent segments and the wrap-around pair, which legitimately touch.
			if j == i || j == i+1 || (i == 0 && j == n-1) {
				continue
			}
			b1, b2 := r[j], r[j+1]
			if segmentsCross(a1, a2, b1, b2) {
				return true
			}
		}
	}
	return false
}

func segmentsCross(p1, p2, q1, q2 [2]float64) bool {
	d1 := cross(q1, q2, p1)
	d2 := cross(q1, q2, p2)
	d3 := cross(p1, p2, q1)
	d4 := cross(p1, p2, q2)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	return false
}

func cross(a, b, p [2]float64) float64 {
	return (b[0]-a[0])*(p[1]-a[1]) - (b[1]-a[1])*(p[0]-a[0])
}

// ContainsGeometry reports whether every exterior point of child lies inside
// this polygon or multipolygon. It is the blocking district-within-region gate
// used before canonical boundary writes.
func (g *Geometry) ContainsGeometry(child *Geometry) bool {
	if g == nil || child == nil {
		return false
	}
	var parents [][]Ring
	switch g.Type {
	case GeomPolygon:
		r, e := asRings(g.Coordinates)
		if e != nil {
			return false
		}
		parents = [][]Ring{r}
	case GeomMultiPolygon:
		p, e := asPolygons(g.Coordinates)
		if e != nil {
			return false
		}
		parents = p
	default:
		return false
	}
	var children [][]Ring
	switch child.Type {
	case GeomPolygon:
		r, e := asRings(child.Coordinates)
		if e != nil {
			return false
		}
		children = [][]Ring{r}
	case GeomMultiPolygon:
		p, e := asPolygons(child.Coordinates)
		if e != nil {
			return false
		}
		children = p
	default:
		return false
	}
	for _, poly := range children {
		if len(poly) == 0 {
			return false
		}
		for _, point := range poly[0] {
			contained := false
			for _, parent := range parents {
				if pointInPolygon(point, parent) {
					contained = true
					break
				}
			}
			if !contained {
				return false
			}
		}
	}
	return true
}

func pointInPolygon(p [2]float64, rings []Ring) bool {
	if len(rings) == 0 || !pointInRing(p, rings[0]) {
		return false
	}
	for _, hole := range rings[1:] {
		if pointInRing(p, hole) {
			return false
		}
	}
	return true
}

func pointInRing(p [2]float64, ring Ring) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]
		if ((yi > p[1]) != (yj > p[1])) && p[0] < (xj-xi)*(p[1]-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

// SignedArea returns twice the signed area of a ring. Positive is
// counter-clockwise. MongoDB interprets a polygon larger than a hemisphere by
// its winding order, so getting this wrong can invert a district into
// "everywhere on Earth except this district".
func SignedArea(r Ring) float64 {
	var sum float64
	for i := 0; i < len(r)-1; i++ {
		sum += (r[i][0] * r[i+1][1]) - (r[i+1][0] * r[i][1])
	}
	return sum
}

// NormalizeWinding enforces the right-hand rule: exterior rings counter-clockwise,
// interior (hole) rings clockwise. Applied on ingest so stored geometry is
// consistent regardless of what a source provided.
func NormalizeWinding(rings []Ring) []Ring {
	out := make([]Ring, len(rings))
	for i, r := range rings {
		area := SignedArea(r)
		wantCCW := i == 0 // exterior ring
		isCCW := area > 0
		if isCCW != wantCCW {
			out[i] = reverseRing(r)
		} else {
			out[i] = r
		}
	}
	return out
}

func reverseRing(r Ring) Ring {
	out := make(Ring, len(r))
	for i := range r {
		out[i] = r[len(r)-1-i]
	}
	return out
}

// ---- coordinate shape coercion ----

func asPosition(v any) ([2]float64, error) {
	s, ok := v.([]float64)
	if !ok || len(s) < 2 {
		return [2]float64{}, ErrGeomBadPosition
	}
	return [2]float64{s[0], s[1]}, nil
}

func asRing(v any) (Ring, error) {
	s, ok := v.([][]float64)
	if !ok {
		return nil, ErrGeomBadPosition
	}
	out := make(Ring, 0, len(s))
	for _, p := range s {
		if len(p) < 2 {
			return nil, ErrGeomBadPosition
		}
		out = append(out, [2]float64{p[0], p[1]})
	}
	return out, nil
}

func asRings(v any) ([]Ring, error) {
	s, ok := v.([][][]float64)
	if !ok {
		return nil, ErrGeomBadPosition
	}
	out := make([]Ring, 0, len(s))
	for _, r := range s {
		ring, err := asRing(anySlice(r))
		if err != nil {
			return nil, err
		}
		out = append(out, ring)
	}
	return out, nil
}

func asPolygons(v any) ([][]Ring, error) {
	s, ok := v.([][][][]float64)
	if !ok {
		return nil, ErrGeomBadPosition
	}
	out := make([][]Ring, 0, len(s))
	for _, p := range s {
		rings, err := asRings(anyRings(p))
		if err != nil {
			return nil, err
		}
		out = append(out, rings)
	}
	return out, nil
}

func anySlice(r [][]float64) any   { return r }
func anyRings(p [][][]float64) any { return p }
