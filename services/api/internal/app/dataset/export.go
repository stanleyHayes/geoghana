package dataset

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	geo "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// pageSize is the read batch when walking a collection for export. The whole
// dataset is small, but streaming keeps memory flat if it stops being small.
const pageSize = 500

// Builder generates the downloadable artifacts for a dataset version.
//
// Checksums and sizes are computed from the bytes actually written, not
// predicted, so the catalogue cannot drift from the files. Nothing is
// recorded in the database until every file is on disk.
type Builder struct {
	regions   ports.RegionRepository
	districts ports.DistrictRepository
	places    ports.PlaceRepository
	repo      Repository
	exportDir string

	// Optional: OSM-derived collections. Absent, the export simply omits
	// them rather than shipping empty files that imply the data is gone.
	roads RoadSource
	pois  POISource
}

// RoadSource and POISource read the OSM-derived collections for export.
// Interfaces rather than concrete repos so the builder stays testable.
type RoadSource interface {
	All(ctx context.Context) ([]geo.Road, error)
}
type POISource interface {
	All(ctx context.Context) ([]geo.POI, error)
}

// WithOSM attaches the OpenStreetMap-derived collections.
func (b *Builder) WithOSM(roads RoadSource, pois POISource) *Builder {
	b.roads, b.pois = roads, pois
	return b
}

func NewBuilder(
	regions ports.RegionRepository, districts ports.DistrictRepository,
	places ports.PlaceRepository, repo Repository, exportDir string,
) *Builder {
	return &Builder{
		regions: regions, districts: districts, places: places,
		repo: repo, exportDir: exportDir,
	}
}

// feature is a GeoJSON Feature. Geometry is `any` because it is either a
// boundary polygon or a centroid point depending on what has been ingested.
type feature struct {
	Type       string         `json:"type"`
	ID         string         `json:"id"`
	Geometry   any            `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
	// Attribution rides INSIDE the file. A download outlives the page it came
	// from, and CC BY has to travel with the data.
	// Attribution rides INSIDE the file. A download outlives the page it came
	// from, and ODbL is share-alike — a notice in a website footer does not
	// travel with a GeoJSON someone saved six months ago.
	Attribution    string `json:"attribution"`
	Licence        string `json:"license"`
	DatasetVersion string `json:"datasetVersion"`
	Generated      string `json:"generated"`
}

type pointGeometry struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

// geometryOf prefers a real boundary and falls back to the centroid. A record
// with neither yields null geometry, which GeoJSON permits — better than
// inventing a location.
func geometryOf(g *geo.Geometry, c *geo.Coordinate) any {
	if g != nil && g.Coordinates != nil {
		return map[string]any{"type": string(g.Type), "coordinates": g.Coordinates}
	}
	if c != nil {
		// GeoJSON position order is [longitude, latitude].
		return pointGeometry{Type: "Point", Coordinates: [2]float64{c.Longitude, c.Latitude}}
	}
	return nil
}

// Build writes every artifact for the version and records them.
func (b *Builder) Build(ctx context.Context, version, generatedAt string) (domain.Version, error) {
	return b.BuildWithChangelog(ctx, version, generatedAt, "")
}

// BuildWithChangelog writes every artifact and records release notes alongside
// the immutable catalogue entry. Build remains for callers that do not need to
// attach notes, while release tooling should use this method.
func (b *Builder) BuildWithChangelog(ctx context.Context, version, generatedAt, changelog string) (domain.Version, error) {
	dir := filepath.Join(b.exportDir, version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return domain.Version{}, fmt.Errorf("create export dir: %w", err)
	}

	regions, err := b.allRegions(ctx)
	if err != nil {
		return domain.Version{}, err
	}
	districts, err := b.allDistricts(ctx)
	if err != nil {
		return domain.Version{}, err
	}
	places, err := b.allPlaces(ctx)
	if err != nil {
		return domain.Version{}, err
	}

	var arts []domain.Artifact
	add := func(a domain.Artifact, err error) error {
		if err != nil {
			return err
		}
		arts = append(arts, a)
		return nil
	}

	// --- GeoJSON ---
	if err := add(b.writeGeoJSON(dir, version, generatedAt, "regions",
		regionFeatures(regions))); err != nil {
		return domain.Version{}, err
	}
	if err := add(b.writeGeoJSON(dir, version, generatedAt, "districts",
		districtFeatures(districts))); err != nil {
		return domain.Version{}, err
	}
	if err := add(b.writeGeoJSON(dir, version, generatedAt, "places",
		placeFeatures(places))); err != nil {
		return domain.Version{}, err
	}

	// --- CSV ---
	if err := add(b.writeCSV(dir, "regions",
		[]string{"id", "name", "capital", "code", "status", "verificationStatus", "source"},
		regionRows(regions))); err != nil {
		return domain.Version{}, err
	}
	if err := add(b.writeCSV(dir, "districts",
		[]string{"id", "name", "regionId", "regionName", "type", "code", "capital", "status", "verificationStatus", "source"},
		districtRows(districts))); err != nil {
		return domain.Version{}, err
	}
	if err := add(b.writeCSV(dir, "places",
		[]string{"id", "name", "type", "regionId", "regionName", "districtId", "districtName", "latitude", "longitude", "population", "status", "verificationStatus", "source"},
		placeRows(places))); err != nil {
		return domain.Version{}, err
	}

	// OSM-derived artifacts. ODbL is share-alike, so these files carry the
	// notice inside them like every other download.
	if b.roads != nil {
		roads, rerr := b.roads.All(ctx)
		if rerr != nil {
			return domain.Version{}, rerr
		}
		if len(roads) > 0 {
			if err := add(b.writeGeoJSON(dir, version, generatedAt, "roads",
				roadFeatures(roads))); err != nil {
				return domain.Version{}, err
			}
			if err := add(b.writeCSV(dir, "roads",
				[]string{"id", "name", "ref", "class", "regionId", "districtId", "source", "attribution"},
				roadRows(roads))); err != nil {
				return domain.Version{}, err
			}
		}
	}
	if b.pois != nil {
		pois, perr := b.pois.All(ctx)
		if perr != nil {
			return domain.Version{}, perr
		}
		if len(pois) > 0 {
			if err := add(b.writeGeoJSON(dir, version, generatedAt, "pois",
				poiFeatures(pois))); err != nil {
				return domain.Version{}, err
			}
			if err := add(b.writeCSV(dir, "pois",
				[]string{"id", "name", "class", "category", "regionId", "districtId", "latitude", "longitude", "source", "attribution"},
				poiRows(pois))); err != nil {
				return domain.Version{}, err
			}
		}
	}

	v := domain.Version{
		Version: version,
		// Built, not live. Publishing is a SEPARATE, audited decision:
		// generating files and declaring them the canonical dataset are
		// different acts, and conflating them meant every export silently
		// became the published version — which is how the catalogue ended up
		// with two live versions at once.
		Status:      domain.StatusApproved,
		PublishedAt: generatedAt,
		Changelog:   changelog,
		Counts: map[string]int64{
			"regions": int64(len(regions)), "districts": int64(len(districts)),
			"places": int64(len(places)),
		},
		Artifacts: arts,
	}
	if err := b.repo.Upsert(ctx, v); err != nil {
		return domain.Version{}, err
	}
	return v, nil
}

// writeGeoJSON writes a FeatureCollection and returns its recorded artifact.
func (b *Builder) writeGeoJSON(
	dir, version, generatedAt, entity string, feats []feature,
) (domain.Artifact, error) {
	name := entity + ".geojson"
	fc := featureCollection{
		Type: "FeatureCollection", Features: feats,
		Attribution: domain.Attribution, Licence: domain.Licence,
		DatasetVersion: version, Generated: generatedAt,
	}
	body, err := json.MarshalIndent(fc, "", " ")
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("encode %s: %w", name, err)
	}
	return b.writeFile(dir, entity, domain.FormatGeoJSON, name, body, int64(len(feats)))
}

func (b *Builder) writeCSV(
	dir, entity string, header []string, rows [][]string,
) (domain.Artifact, error) {
	name := entity + ".csv"
	var buf writeCounter
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return domain.Artifact{}, fmt.Errorf("write %s header: %w", name, err)
	}
	if err := w.WriteAll(rows); err != nil {
		return domain.Artifact{}, fmt.Errorf("write %s rows: %w", name, err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return domain.Artifact{}, fmt.Errorf("flush %s: %w", name, err)
	}
	return b.writeFile(dir, entity, domain.FormatCSV, name, buf.buf, int64(len(rows)))
}

// writeFile writes bytes and derives size and checksum FROM THOSE BYTES, so
// the catalogue can never describe a file that was not written.
func (b *Builder) writeFile(
	dir, entity string, format domain.Format, name string, body []byte, count int64,
) (domain.Artifact, error) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return domain.Artifact{}, fmt.Errorf("write %s: %w", path, err)
	}
	sum := sha256.Sum256(body)
	a := domain.Artifact{
		Entity: entity, Format: format, Filename: name,
		SizeBytes: int64(len(body)), SHA256: hex.EncodeToString(sum[:]),
		RecordCount: count,
	}
	if err := a.SafeFilename(); err != nil {
		return domain.Artifact{}, err
	}
	return a, nil
}

// writeCounter collects CSV output in memory so the checksum covers exactly
// what lands on disk.
type writeCounter struct{ buf []byte }

func (w *writeCounter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

var _ io.Writer = (*writeCounter)(nil)

// ---- record collection ----

func (b *Builder) allRegions(ctx context.Context) ([]geo.Region, error) {
	var out []geo.Region
	cursor := ""
	for {
		// The exporter is the one caller that genuinely wants polygons: a
		// boundary file without geometry is the bug this replaced.
		page, err := b.regions.List(ctx, ports.ListParams{
			Cursor: cursor, Limit: pageSize, IncludeGeometry: true,
		})
		if err != nil {
			return nil, fmt.Errorf("list regions: %w", err)
		}
		out = append(out, page.Data...)
		if page.NextCursor == "" {
			return out, nil
		}
		cursor = page.NextCursor
	}
}

func (b *Builder) allDistricts(ctx context.Context) ([]geo.District, error) {
	var out []geo.District
	cursor := ""
	for {
		page, err := b.districts.List(ctx, ports.DistrictFilter{
			ListParams: ports.ListParams{Cursor: cursor, Limit: pageSize, IncludeGeometry: true},
		})
		if err != nil {
			return nil, fmt.Errorf("list districts: %w", err)
		}
		out = append(out, page.Data...)
		if page.NextCursor == "" {
			return out, nil
		}
		cursor = page.NextCursor
	}
}

func (b *Builder) allPlaces(ctx context.Context) ([]geo.Place, error) {
	var out []geo.Place
	cursor := ""
	for {
		page, err := b.places.List(ctx, ports.PlaceFilter{
			ListParams: ports.ListParams{Cursor: cursor, Limit: pageSize, IncludeGeometry: true},
		})
		if err != nil {
			return nil, fmt.Errorf("list places: %w", err)
		}
		out = append(out, page.Data...)
		if page.NextCursor == "" {
			return out, nil
		}
		cursor = page.NextCursor
	}
}

// ---- feature and row projections ----

func regionFeatures(rs []geo.Region) []feature {
	out := make([]feature, 0, len(rs))
	for _, r := range rs {
		out = append(out, feature{
			Type: "Feature", ID: r.ID,
			Geometry: geometryOf(r.Geometry, r.Centroid),
			Properties: map[string]any{
				"name": r.Name, "capital": r.Capital, "code": r.OfficialCode,
				"countryCode": r.CountryCode, "status": string(r.Status),
				"verificationStatus": string(r.VerificationStatus),
				"source":             r.Provenance.SourceID, "sourceUrl": r.Provenance.SourceURL,
			},
		})
	}
	return out
}

func districtFeatures(ds []geo.District) []feature {
	out := make([]feature, 0, len(ds))
	for _, d := range ds {
		out = append(out, feature{
			Type: "Feature", ID: d.ID,
			Geometry: geometryOf(d.Geometry, d.Centroid),
			Properties: map[string]any{
				"name": d.Name, "regionId": d.RegionID, "regionName": d.RegionName,
				"type": d.DistrictType, "code": d.OfficialCode, "capital": d.Capital,
				"status": string(d.Status), "verificationStatus": string(d.VerificationStatus),
				"source": d.Provenance.SourceID, "sourceUrl": d.Provenance.SourceURL,
			},
		})
	}
	return out
}

func placeFeatures(ps []geo.Place) []feature {
	out := make([]feature, 0, len(ps))
	for _, p := range ps {
		aliases := make([]string, 0, len(p.Aliases))
		for _, a := range p.Aliases {
			// The true orthography, not the folded matching form.
			aliases = append(aliases, a.Value)
		}
		props := map[string]any{
			"name": p.Name, "type": string(p.Type),
			"regionId": p.RegionID, "regionName": p.RegionName,
			"districtId": p.DistrictID, "districtName": p.DistrictName,
			"aliases": aliases, "status": string(p.Status),
			"verificationStatus": string(p.VerificationStatus),
			"source":             p.Provenance.SourceID, "sourceUrl": p.Provenance.SourceURL,
		}
		if p.Population != nil {
			props["population"] = *p.Population
		}
		out = append(out, feature{
			Type: "Feature", ID: p.ID,
			Geometry:   geometryOf(p.Geometry, p.Centroid),
			Properties: props,
		})
	}
	return out
}

func roadFeatures(rs []geo.Road) []feature {
	out := make([]feature, 0, len(rs))
	for _, r := range rs {
		out = append(out, feature{
			Type: "Feature", ID: r.ID,
			Geometry: geometryOf(r.Geometry, nil),
			Properties: map[string]any{
				"name": r.Name, "ref": r.Ref, "class": string(r.Class),
				"regionId": r.RegionID, "districtId": r.DistrictID,
				"source": r.Provenance.SourceID, "sourceUrl": r.Provenance.SourceURL,
				// Per-feature as well as per-file: a consumer that splits the
				// collection keeps the licence with each record.
				"attribution": r.Attribution,
			},
		})
	}
	return out
}

func poiFeatures(ps []geo.POI) []feature {
	out := make([]feature, 0, len(ps))
	for _, p := range ps {
		out = append(out, feature{
			Type: "Feature", ID: p.ID,
			Geometry: geometryOf(nil, p.Centroid),
			Properties: map[string]any{
				"name": p.Name, "class": string(p.Class), "category": p.Category,
				"regionId": p.RegionID, "districtId": p.DistrictID,
				"source": p.Provenance.SourceID, "sourceUrl": p.Provenance.SourceURL,
				"attribution": p.Attribution,
			},
		})
	}
	return out
}

func roadRows(rs []geo.Road) [][]string {
	out := make([][]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, []string{
			r.ID, r.Name, r.Ref, string(r.Class), r.RegionID, r.DistrictID,
			r.Provenance.SourceID, r.Attribution,
		})
	}
	return out
}

func poiRows(ps []geo.POI) [][]string {
	out := make([][]string, 0, len(ps))
	for _, p := range ps {
		lat, lng := "", ""
		if p.Centroid != nil {
			lat = strconv.FormatFloat(p.Centroid.Latitude, 'f', -1, 64)
			lng = strconv.FormatFloat(p.Centroid.Longitude, 'f', -1, 64)
		}
		out = append(out, []string{
			p.ID, p.Name, string(p.Class), p.Category, p.RegionID, p.DistrictID,
			lat, lng, p.Provenance.SourceID, p.Attribution,
		})
	}
	return out
}

func regionRows(rs []geo.Region) [][]string {
	out := make([][]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, []string{
			r.ID, r.Name, r.Capital, r.OfficialCode,
			string(r.Status), string(r.VerificationStatus), r.Provenance.SourceID,
		})
	}
	return out
}

func districtRows(ds []geo.District) [][]string {
	out := make([][]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, []string{
			d.ID, d.Name, d.RegionID, d.RegionName, d.DistrictType,
			d.OfficialCode, d.Capital, string(d.Status),
			string(d.VerificationStatus), d.Provenance.SourceID,
		})
	}
	return out
}

func placeRows(ps []geo.Place) [][]string {
	out := make([][]string, 0, len(ps))
	for _, p := range ps {
		lat, lng := "", ""
		if p.Centroid != nil {
			lat = strconv.FormatFloat(p.Centroid.Latitude, 'f', -1, 64)
			lng = strconv.FormatFloat(p.Centroid.Longitude, 'f', -1, 64)
		}
		pop := ""
		if p.Population != nil {
			pop = strconv.FormatInt(*p.Population, 10)
		}
		out = append(out, []string{
			p.ID, p.Name, string(p.Type), p.RegionID, p.RegionName,
			p.DistrictID, p.DistrictName, lat, lng, pop,
			string(p.Status), string(p.VerificationStatus), p.Provenance.SourceID,
		})
	}
	return out
}
