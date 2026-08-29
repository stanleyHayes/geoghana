// Package osm reads an OpenStreetMap PBF extract (story GEO-4.7).
//
// Everything derived here carries ODbL attribution. ODbL is share-alike, so
// that is a licence obligation rather than a courtesy, and the domain types
// refuse to validate without it.
package osm

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/paulmach/osm"
	"github.com/paulmach/osm/osmpbf"

	ingest "github.com/ghanageo/ghanageo/services/api/internal/app/ingest"
	geo "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// Licence describes the source for the licensing register.
func Licence() ingest.Licence {
	return ingest.Licence{
		Name:        "OpenStreetMap (Geofabrik Ghana extract)",
		URL:         "https://download.geofabrik.de/africa/ghana.html",
		SPDX:        "ODbL-1.0",
		Attribution: geo.ODbLAttribution,
		// Redistributable, but only under the same terms — which is what
		// ShareAlike records.
		Redistributable: true,
		// ODbL is share-alike: a derived database carries the same licence.
		// Recording that here means the register cannot describe it as
		// permissive by omission.
		ShareAlike: true,
	}
}

// Counts reports what a scan found.
type Counts struct {
	NodesScanned, WaysScanned int64
	Places, Roads, POIs       int
	SkippedUnnamed            int
	SkippedOutsideGhana       int
}

func (c Counts) String() string {
	return fmt.Sprintf(
		"scanned %d nodes / %d ways · %d places · %d roads · %d POIs · skipped %d unnamed, %d outside Ghana",
		c.NodesScanned, c.WaysScanned, c.Places, c.Roads, c.POIs,
		c.SkippedUnnamed, c.SkippedOutsideGhana)
}

// Extract is what one pass over a PBF yields.
type Extract struct {
	Places []geo.Place
	Roads  []geo.Road
	POIs   []geo.POI
	Counts Counts
}

// poiClassFor maps OSM tags onto a published class.
//
// Returns false for anything not worth publishing. The list is deliberately
// short: a location API is asked about hospitals and schools, not about
// individual benches, and importing everything would bury the useful records.
func poiClassFor(tags osm.Tags) (geo.POIClass, string, bool) {
	get := func(k string) string { return strings.ToLower(strings.TrimSpace(tags.Find(k))) }

	if v := get("amenity"); v != "" {
		switch v {
		case "school", "college", "university", "kindergarten", "library":
			return geo.POIEducation, v, true
		case "hospital", "clinic", "doctors", "pharmacy", "dentist", "health_post":
			return geo.POIHealth, v, true
		case "townhall", "courthouse", "police", "fire_station", "post_office", "embassy":
			return geo.POIGovernment, v, true
		case "bank", "atm", "bureau_de_change":
			return geo.POIFinance, v, true
		case "bus_station", "ferry_terminal", "taxi", "fuel":
			return geo.POITransport, v, true
		case "place_of_worship":
			return geo.POIWorship, v, true
		case "marketplace", "restaurant", "cafe", "bar", "fast_food":
			return geo.POICommerce, v, true
		}
	}
	if v := get("shop"); v != "" {
		return geo.POICommerce, "shop:" + v, true
	}
	if v := get("tourism"); v != "" {
		switch v {
		case "hotel", "guest_house", "hostel", "motel":
			return geo.POIHospitality, v, true
		case "museum", "attraction", "viewpoint":
			return geo.POILandmark, v, true
		}
	}
	if v := get("aeroway"); v == "aerodrome" {
		return geo.POITransport, "airport", true
	}
	return "", "", false
}

// placeTypeFor maps OSM place=* onto our human-geography types.
//
// Ghana is deliberately not modelled as country/state/city, so the mapping is
// to the domain's own vocabulary rather than OSM's.
func placeTypeFor(v string) (geo.PlaceType, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "city":
		return geo.PlaceCity, true
	case "town":
		return geo.PlaceTown, true
	case "village":
		return geo.PlaceVillage, true
	case "hamlet":
		return geo.PlaceHamlet, true
	case "suburb", "quarter":
		return geo.PlaceSuburb, true
	case "neighbourhood":
		return geo.PlaceNeighbourhood, true
	case "isolated_dwelling", "farm":
		return geo.PlaceSettlement, true
	case "locality":
		return geo.PlaceLocality, true
	}
	return "", false
}

// Scan reads a PBF and returns everything worth importing.
//
// TWO passes, not one. A PBF lists every node before any way, so a single pass
// cannot know which node coordinates a road will need and would have to retain
// all of them — Ghana has several million, which is hundreds of megabytes held
// to build a few thousand roads. Pass one reads ways only and records which
// nodes matter; pass two reads nodes and keeps just those, plus the named
// places and POIs. Decompression is paid twice; memory stays bounded.
func Scan(ctx context.Context, path, datasetVersion string) (Extract, error) {
	out := Extract{}

	// ---- pass 1: ways worth keeping, and the nodes they reference ----
	ways, wanted, waysScanned, err := scanWays(ctx, path)
	if err != nil {
		return Extract{}, err
	}
	out.Counts.WaysScanned = waysScanned

	// ---- pass 2: coordinates for those nodes, plus places and POIs ----
	nodeLoc, err := scanNodes(ctx, path, wanted, datasetVersion, &out)
	if err != nil {
		return Extract{}, err
	}

	for _, w := range ways {
		if r, ok := roadFrom(w, nodeLoc, datasetVersion); ok {
			out.Roads = append(out.Roads, r)
		}
	}
	out.Counts.Roads = len(out.Roads)
	return out, nil
}

func scanWays(ctx context.Context, path string) ([]*osm.Way, map[osm.NodeID]struct{}, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("open extract: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := osmpbf.New(ctx, f, 4)
	scanner.SkipNodes = true
	scanner.SkipRelations = true
	defer func() { _ = scanner.Close() }()

	var (
		kept    []*osm.Way
		wanted  = map[osm.NodeID]struct{}{}
		scanned int64
	)
	for scanner.Scan() {
		w, ok := scanner.Object().(*osm.Way)
		if !ok {
			continue
		}
		scanned++
		if keepWay(w) == nil {
			continue
		}
		kept = append(kept, w)
		for _, n := range w.Nodes {
			wanted[n.ID] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, nil, 0, fmt.Errorf("scan ways: %w", err)
	}
	return kept, wanted, scanned, nil
}

func scanNodes(
	ctx context.Context, path string, wanted map[osm.NodeID]struct{},
	version string, out *Extract,
) (map[osm.NodeID][2]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open extract: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := osmpbf.New(ctx, f, 4)
	scanner.SkipWays = true
	scanner.SkipRelations = true
	defer func() { _ = scanner.Close() }()

	loc := make(map[osm.NodeID][2]float64, len(wanted))
	for scanner.Scan() {
		n, ok := scanner.Object().(*osm.Node)
		if !ok {
			continue
		}
		out.Counts.NodesScanned++
		if _, need := wanted[n.ID]; need {
			loc[n.ID] = [2]float64{n.Lon, n.Lat}
		}
		collectNode(n, version, out)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("scan nodes: %w", err)
	}
	return loc, nil
}

func collectNode(n *osm.Node, version string, out *Extract) {
	name := strings.TrimSpace(n.Tags.Find("name"))
	if name == "" {
		// An unnamed node cannot be looked up by name, which is what this
		// dataset is for.
		if n.Tags.Find("place") != "" || n.Tags.Find("amenity") != "" {
			out.Counts.SkippedUnnamed++
		}
		return
	}
	c := &geo.Coordinate{Latitude: n.Lat, Longitude: n.Lon}
	if err := c.Validate(); err != nil {
		out.Counts.SkippedOutsideGhana++
		return
	}

	if t, ok := placeTypeFor(n.Tags.Find("place")); ok {
		out.Places = append(out.Places, geo.Place{
			Name: name, Type: t, Centroid: c,
			Status: geo.StatusActive,
			// From a source, unreviewed. Never CANONICAL automatically (R5).
			VerificationStatus: geo.VerificationReference,
			Provenance: geo.Provenance{
				SourceID:   "openstreetmap",
				ExternalID: fmt.Sprintf("node/%d", n.ID),
				SourceURL:  fmt.Sprintf("https://www.openstreetmap.org/node/%d", n.ID),
			},
			DatasetVersion: version,
		})
		out.Counts.Places++
		return
	}

	if class, category, ok := poiClassFor(n.Tags); ok {
		out.POIs = append(out.POIs, geo.POI{
			Name: name, Class: class, Category: category, Centroid: c,
			Status: geo.StatusActive, VerificationStatus: geo.VerificationReference,
			Provenance: geo.Provenance{
				SourceID:   "openstreetmap",
				ExternalID: fmt.Sprintf("node/%d", n.ID),
				SourceURL:  fmt.Sprintf("https://www.openstreetmap.org/node/%d", n.ID),
			},
			Attribution:    geo.ODbLAttribution,
			DatasetVersion: version,
		})
		out.Counts.POIs++
	}
}

// keepWay decides whether a way is worth carrying to the second pass.
func keepWay(w *osm.Way) *osm.Way {
	if _, ok := geo.ParseRoadClass(w.Tags.Find("highway")); !ok {
		return nil
	}
	if strings.TrimSpace(w.Tags.Find("name")) == "" &&
		strings.TrimSpace(w.Tags.Find("ref")) == "" {
		return nil
	}
	return w
}

func roadFrom(w *osm.Way, loc map[osm.NodeID][2]float64, version string) (geo.Road, bool) {
	class, ok := geo.ParseRoadClass(w.Tags.Find("highway"))
	if !ok {
		return geo.Road{}, false
	}
	coords := make([][]float64, 0, len(w.Nodes))
	for _, n := range w.Nodes {
		if p, ok := loc[n.ID]; ok {
			coords = append(coords, []float64{p[0], p[1]})
		}
	}
	// A LineString needs two positions. A way whose nodes fell outside the
	// extract cannot be drawn, and a one-point "line" is not geometry.
	if len(coords) < 2 {
		return geo.Road{}, false
	}
	return geo.Road{
		Name:  strings.TrimSpace(w.Tags.Find("name")),
		Ref:   strings.TrimSpace(w.Tags.Find("ref")),
		Class: class,
		Geometry: &geo.Geometry{
			Type: geo.GeomLineString, Coordinates: coords,
		},
		Status: geo.StatusActive, VerificationStatus: geo.VerificationReference,
		Provenance: geo.Provenance{
			SourceID:   "openstreetmap",
			ExternalID: fmt.Sprintf("way/%d", w.ID),
			SourceURL:  fmt.Sprintf("https://www.openstreetmap.org/way/%d", w.ID),
		},
		Attribution:    geo.ODbLAttribution,
		DatasetVersion: version,
	}, true
}
