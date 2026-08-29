package osm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmgeojson"
	"github.com/paulmach/osm/osmpbf"

	geo "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// District boundaries from OpenStreetMap administrative relations.
//
// geoBoundaries represents 2019 and Ghana has reorganised since, so 13 of our
// 261 districts have no boundary there — Guan District was inaugurated in
// October 2021, two years after that data was published. OSM carries current
// administrative relations, which is the only openly-licensed source that
// reflects the present structure.
//
// Ghana's districts are admin_level=6.

// AdminBoundary is one administrative area lifted from the extract.
type AdminBoundary struct {
	Name       string
	RelationID int64
	// Geometry is a Polygon or MultiPolygon in GeoJSON order.
	Geometry *geo.Geometry
	// Centroid is a representative interior point, used to decide which
	// region the area belongs to.
	//
	// Region gating must NOT use the full polygon: a district boundary shares
	// edges with its neighbours, so $geoIntersects matches several regions and
	// returns an arbitrary one. Two districts were assigned to the wrong
	// region and then failed to match any candidate there.
	Centroid *geo.Coordinate
}

// centroidOf returns the area-weighted centre of a feature.
func centroidOf(f *geojson.Feature) *geo.Coordinate {
	switch g := f.Geometry.(type) {
	case orb.Polygon:
		c, _ := planar.CentroidArea(g)
		return &geo.Coordinate{Latitude: c.Lat(), Longitude: c.Lon()}
	case orb.MultiPolygon:
		c, _ := planar.CentroidArea(g)
		return &geo.Coordinate{Latitude: c.Lat(), Longitude: c.Lon()}
	}
	return nil
}

// ScanDistrictBoundaries extracts admin_level=6 areas.
//
// THREE passes, for the same reason the road scan takes two: a PBF lists
// nodes, then ways, then relations, so nothing downstream is known when the
// earlier elements go by. Pass one reads relations to learn which ways
// matter, pass two reads those ways to learn which nodes matter, pass three
// reads those nodes. Only the members of the ~300 admin relations are
// retained rather than the extract's 18.7 million nodes.
func ScanDistrictBoundaries(ctx context.Context, path string) ([]AdminBoundary, error) {
	rels, wantWays, err := scanAdminRelations(ctx, path)
	if err != nil {
		return nil, err
	}
	ways, wantNodes, err := scanMemberWays(ctx, path, wantWays)
	if err != nil {
		return nil, err
	}
	nodes, err := scanMemberNodes(ctx, path, wantNodes)
	if err != nil {
		return nil, err
	}

	// osmgeojson assembles member ways into rings, handling inner/outer and
	// direction. Hand-rolling that is where multipolygon bugs live.
	set := &osm.OSM{Nodes: nodes, Ways: ways, Relations: rels}
	fc, err := osmgeojson.Convert(set, osmgeojson.NoMeta(true), osmgeojson.NoRelationMembership(true))
	if err != nil {
		return nil, fmt.Errorf("assemble boundary geometry: %w", err)
	}

	// Convert emits the member ways as LineStrings alongside the assembled
	// areas, so the great majority of features are not boundaries. Only
	// Polygon and MultiPolygon are areas.
	out := make([]AdminBoundary, 0, len(rels))
	for _, f := range fc.Features {
		name := featureName(f)
		if name == "" {
			continue
		}
		g, ok := toDomainGeometry(f)
		if !ok {
			continue
		}
		out = append(out, AdminBoundary{Name: name, Geometry: g, Centroid: centroidOf(f)})
	}
	return out, nil
}

// featureName reads the OSM name.
//
// osmgeojson nests the original tags under a "tags" property rather than
// flattening them, so Properties["name"] is always absent — which is what made
// the first run report zero boundaries from 264 perfectly good polygons.
func featureName(f *geojson.Feature) string {
	// osmgeojson stores tags as map[string]string. Asserting the more usual
	// map[string]interface{} silently fails and every boundary comes back
	// nameless — 264 good polygons reported as zero.
	switch tags := f.Properties["tags"].(type) {
	case map[string]string:
		return strings.TrimSpace(tags["name"])
	case map[string]interface{}:
		if n, ok := tags["name"].(string); ok {
			return strings.TrimSpace(n)
		}
	}
	if n, ok := f.Properties["name"].(string); ok {
		return strings.TrimSpace(n)
	}
	return ""
}

// toDomainGeometry converts a GeoJSON feature into our geometry, keeping only
// the area types a boundary can legitimately be.
func toDomainGeometry(f *geojson.Feature) (*geo.Geometry, bool) {
	// geojson.NewGeometry, not json.Marshal on the field: orb.Geometry is an
	// interface, and marshalling it directly yields the bare coordinates with
	// no "type" — which silently made every boundary unrecognisable.
	raw, err := json.Marshal(geojson.NewGeometry(f.Geometry))
	if err != nil {
		return nil, false
	}
	var shape struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if err := json.Unmarshal(raw, &shape); err != nil {
		return nil, false
	}
	switch shape.Type {
	case "Polygon":
		var c [][][]float64
		if err := json.Unmarshal(shape.Coordinates, &c); err != nil {
			return nil, false
		}
		return &geo.Geometry{Type: geo.GeomPolygon, Coordinates: c}, true
	case "MultiPolygon":
		var c [][][][]float64
		if err := json.Unmarshal(shape.Coordinates, &c); err != nil {
			return nil, false
		}
		return &geo.Geometry{Type: geo.GeomMultiPolygon, Coordinates: c}, true
	}
	// A LineString relation is an unclosed boundary — real in OSM, but not a
	// district shape, so it is skipped rather than coerced.
	return nil, false
}

func scanAdminRelations(ctx context.Context, path string) ([]*osm.Relation, map[osm.WayID]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open extract: %w", err)
	}
	defer func() { _ = f.Close() }()

	s := osmpbf.New(ctx, f, 4)
	s.SkipNodes, s.SkipWays = true, true
	defer func() { _ = s.Close() }()

	var rels []*osm.Relation
	want := map[osm.WayID]struct{}{}
	for s.Scan() {
		r, ok := s.Object().(*osm.Relation)
		if !ok {
			continue
		}
		if r.Tags.Find("boundary") != "administrative" || r.Tags.Find("admin_level") != "6" {
			continue
		}
		rels = append(rels, r)
		for _, m := range r.Members {
			if m.Type == osm.TypeWay {
				want[osm.WayID(m.Ref)] = struct{}{}
			}
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		return nil, nil, fmt.Errorf("scan relations: %w", err)
	}
	return rels, want, nil
}

func scanMemberWays(
	ctx context.Context, path string, want map[osm.WayID]struct{},
) ([]*osm.Way, map[osm.NodeID]struct{}, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open extract: %w", err)
	}
	defer func() { _ = f.Close() }()

	s := osmpbf.New(ctx, f, 4)
	s.SkipNodes, s.SkipRelations = true, true
	defer func() { _ = s.Close() }()

	var ways []*osm.Way
	nodes := map[osm.NodeID]struct{}{}
	for s.Scan() {
		w, ok := s.Object().(*osm.Way)
		if !ok {
			continue
		}
		if _, need := want[w.ID]; !need {
			continue
		}
		ways = append(ways, w)
		for _, n := range w.Nodes {
			nodes[n.ID] = struct{}{}
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		return nil, nil, fmt.Errorf("scan member ways: %w", err)
	}
	return ways, nodes, nil
}

func scanMemberNodes(
	ctx context.Context, path string, want map[osm.NodeID]struct{},
) ([]*osm.Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open extract: %w", err)
	}
	defer func() { _ = f.Close() }()

	s := osmpbf.New(ctx, f, 4)
	s.SkipWays, s.SkipRelations = true, true
	defer func() { _ = s.Close() }()

	out := make([]*osm.Node, 0, len(want))
	for s.Scan() {
		n, ok := s.Object().(*osm.Node)
		if !ok {
			continue
		}
		if _, need := want[n.ID]; need {
			out = append(out, n)
		}
	}
	if err := s.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("scan member nodes: %w", err)
	}
	return out, nil
}
