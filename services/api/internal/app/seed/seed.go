// Package seed imports the bootstrap dataset from version-controlled CSVs.
//
// The import is IDEMPOTENT (plan rule R8): running it twice produces no net
// change. Seed rows keep the verification status the CSV declares and are never
// promoted to canonical by this path (rule R5).
package seed

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
	"strings"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// Expected counts, asserted by CI (Spec 31.1).
const (
	ExpectedRegions   = 16
	ExpectedDistricts = 261
	ExpectedPlaces    = 16
)

// Manifest describes a reproducible seed import.
type Manifest struct {
	DatasetVersion string            `json:"datasetVersion"`
	GeneratedAt    string            `json:"generatedAt"`
	Files          map[string]string `json:"files"`     // logical name -> relative path
	Checksums      map[string]string `json:"checksums"` // relative path -> sha256
	SourceRevision string            `json:"sourceRevision"`
}

type Result struct {
	RegionsCreated, RegionsUpdated     int
	DistrictsCreated, DistrictsUpdated int
	PlacesCreated, PlacesUpdated       int
	Skipped                            []string
	// Protected records were left alone because a steward has already
	// reviewed them. See stewardOwned.
	Protected []string
}

func (r Result) Changed() int {
	return r.RegionsCreated + r.DistrictsCreated + r.PlacesCreated
}

func (r Result) String() string {
	return fmt.Sprintf(
		"regions %d created / %d updated · districts %d created / %d updated · places %d created / %d updated · %d skipped · %d steward-owned, left alone",
		r.RegionsCreated, r.RegionsUpdated, r.DistrictsCreated, r.DistrictsUpdated,
		r.PlacesCreated, r.PlacesUpdated, len(r.Skipped), len(r.Protected))
}

// stewardOwned reports whether a record has been advanced past the seed's
// authority and must not be overwritten by a re-import.
//
// Seeding is an UPSERT, so re-running it used to rewrite every field of every
// record — reverting a steward's correction and demoting their verification
// status back to the CSV's, silently and with no audit row. Rule R5 says a
// seed row is never promoted to canonical automatically; the same reasoning
// forbids the reverse, which is worse: it destroys work a human did on
// purpose.
//
// REVIEWED and CANONICAL are exactly the states only a steward can set, so
// they are the boundary.
func stewardOwned(v geography.VerificationStatus) bool {
	return v.PromotableToCanonical()
}

// Importer wires the repositories the seed needs.
type Importer struct {
	Regions   ports.RegionRepository
	Districts ports.DistrictRepository
	Places    ports.PlaceRepository
}

// LoadManifest reads and verifies a manifest, checking every file checksum so a
// release is reproducible (plan GEO-4.1).
func LoadManifest(path string) (*Manifest, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, "", fmt.Errorf("parse manifest: %w", err)
	}
	base := filepath.Dir(path)
	for rel, want := range m.Checksums {
		got, err := fileSHA256(filepath.Join(base, rel))
		if err != nil {
			return nil, "", err
		}
		if got != want {
			return nil, "", fmt.Errorf(
				"checksum mismatch for %s:\n  manifest: %s\n  actual:   %s\nthe seed files changed without the manifest being regenerated", rel, want, got)
		}
	}
	return &m, base, nil
}

func fileSHA256(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", p, err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Run imports every file in the manifest.
func (im *Importer) Run(ctx context.Context, m *Manifest, base string) (Result, error) {
	var res Result

	if p, ok := m.Files["regions"]; ok {
		if err := im.importRegions(ctx, filepath.Join(base, p), m.DatasetVersion, &res); err != nil {
			return res, err
		}
	}
	if p, ok := m.Files["districts"]; ok {
		if err := im.importDistricts(ctx, filepath.Join(base, p), m.DatasetVersion, &res); err != nil {
			return res, err
		}
	}
	if p, ok := m.Files["places"]; ok {
		if err := im.importPlaces(ctx, filepath.Join(base, p), m.DatasetVersion, &res); err != nil {
			return res, err
		}
	}
	return res, nil
}

// readCSV returns rows keyed by header name, so column order can change safely.
func readCSV(path string) ([]map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read header of %s: %w", path, err)
	}
	for i := range header {
		header[i] = strings.TrimSpace(strings.TrimPrefix(header[i], "\ufeff"))
	}

	var rows []map[string]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		m := make(map[string]string, len(header))
		for i, h := range header {
			if i < len(rec) {
				m[h] = strings.TrimSpace(rec[i])
			}
		}
		rows = append(rows, m)
	}
	return rows, nil
}

func provenanceFrom(row map[string]string, sourceID string) geography.Provenance {
	payload, _ := json.Marshal(row) // encoding/json sorts string map keys.
	sum := sha256.Sum256(payload)
	externalID := row["id"]
	return geography.Provenance{
		SourceID: sourceID, ExternalID: externalID,
		SourceURL: row["source_url"], RetrievedAt: row["retrieved_at"],
		SourcePayloadHash: hex.EncodeToString(sum[:]), Notes: row["notes"],
	}
}

func seedID(entity, legacyID string) (string, error) {
	return geography.StableID(entity, "seed-bootstrap", legacyID)
}

func statusOf(row map[string]string) geography.Status {
	if s := row["status"]; s != "" {
		return geography.Status(s)
	}
	return geography.StatusActive
}

func verificationOf(row map[string]string) geography.VerificationStatus {
	if v := row["verification_status"]; v != "" {
		return geography.VerificationStatus(v)
	}
	return geography.VerificationReference
}

func (im *Importer) importRegions(ctx context.Context, path, version string, res *Result) error {
	rows, err := readCSV(path)
	if err != nil {
		return err
	}
	for _, row := range rows {
		id, idErr := seedID("region", row["id"])
		if idErr != nil {
			return fmt.Errorf("region %s id: %w", row["id"], idErr)
		}
		r := geography.Region{
			ID:                 id,
			CountryCode:        row["country_code"],
			Name:               row["name"],
			Capital:            row["capital"],
			OfficialCode:       row["official_code"],
			Status:             statusOf(row),
			VerificationStatus: verificationOf(row),
			Provenance:         provenanceFrom(row, "seed-bootstrap"),
			DatasetVersion:     version,
		}
		if existing, gerr := im.Regions.Get(ctx, r.ID); gerr == nil &&
			existing != nil && stewardOwned(existing.VerificationStatus) {
			res.Protected = append(res.Protected, r.ID)
			continue
		}
		created, err := im.Regions.Upsert(ctx, r)
		if err != nil {
			return fmt.Errorf("region %s: %w", r.ID, err)
		}
		if created {
			res.RegionsCreated++
		} else {
			res.RegionsUpdated++
		}
	}
	return nil
}

func (im *Importer) importDistricts(ctx context.Context, path, version string, res *Result) error {
	rows, err := readCSV(path)
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		legacyID := row["id"]
		if _, dup := seen[legacyID]; dup {
			res.Skipped = append(res.Skipped, "duplicate district id: "+legacyID)
			continue
		}
		seen[legacyID] = struct{}{}
		id, idErr := seedID("district", legacyID)
		if idErr != nil {
			return fmt.Errorf("district %s id: %w", legacyID, idErr)
		}
		regionID, idErr := seedID("region", row["region_id"])
		if idErr != nil {
			return fmt.Errorf("district %s region id: %w", legacyID, idErr)
		}

		d := geography.District{
			ID:                 id,
			RegionID:           regionID,
			RegionName:         row["region_name"],
			Name:               row["name"],
			DistrictType:       row["classification_hint"],
			OfficialCode:       row["official_code"],
			Capital:            row["capital"],
			Status:             statusOf(row),
			VerificationStatus: verificationOf(row),
			Provenance:         provenanceFrom(row, "seed-bootstrap"),
			DatasetVersion:     version,
		}
		if existing, gerr := im.Districts.Get(ctx, d.ID); gerr == nil &&
			existing != nil && stewardOwned(existing.VerificationStatus) {
			res.Protected = append(res.Protected, d.ID)
			continue
		}
		created, err := im.Districts.Upsert(ctx, d)
		if err != nil {
			return fmt.Errorf("district %s: %w", d.ID, err)
		}
		if created {
			res.DistrictsCreated++
		} else {
			res.DistrictsUpdated++
		}
	}
	return nil
}

func (im *Importer) importPlaces(ctx context.Context, path, version string, res *Result) error {
	rows, err := readCSV(path)
	if err != nil {
		return err
	}
	for _, row := range rows {
		pt, err := geography.ParsePlaceType(row["place_type"])
		if err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("place %s: %v", row["id"], err))
			continue
		}
		id, idErr := seedID("place", row["id"])
		if idErr != nil {
			return fmt.Errorf("place %s id: %w", row["id"], idErr)
		}
		regionID := ""
		if row["region_id"] != "" {
			regionID, idErr = seedID("region", row["region_id"])
			if idErr != nil {
				return fmt.Errorf("place %s region id: %w", row["id"], idErr)
			}
		}
		districtID := ""
		if row["district_id"] != "" {
			districtID, idErr = seedID("district", row["district_id"])
			if idErr != nil {
				return fmt.Errorf("place %s district id: %w", row["id"], idErr)
			}
		}
		p := geography.Place{
			ID:                 id,
			Name:               row["name"],
			Type:               pt,
			RegionID:           regionID,
			RegionName:         row["region_name"],
			DistrictID:         districtID,
			DistrictName:       row["district_name"],
			Status:             statusOf(row),
			VerificationStatus: verificationOf(row),
			Provenance:         provenanceFrom(row, "seed-bootstrap"),
			DatasetVersion:     version,
		}
		// Coordinates are intentionally blank in the bootstrap seed; only set a
		// centroid when the CSV actually carries one, and validate it if so.
		if lat, lng := row["latitude"], row["longitude"]; lat != "" && lng != "" {
			c, err := parseCoord(lat, lng)
			if err != nil {
				res.Skipped = append(res.Skipped, fmt.Sprintf("place %s: %v", p.ID, err))
				continue
			}
			p.Centroid = c
		}
		if existing, gerr := im.Places.Get(ctx, p.ID); gerr == nil &&
			existing != nil && stewardOwned(existing.VerificationStatus) {
			res.Protected = append(res.Protected, p.ID)
			continue
		}
		created, err := im.Places.Upsert(ctx, p)
		if err != nil {
			return fmt.Errorf("place %s: %w", p.ID, err)
		}
		if created {
			res.PlacesCreated++
		} else {
			res.PlacesUpdated++
		}
	}
	return nil
}

func parseCoord(lat, lng string) (*geography.Coordinate, error) {
	var la, lo float64
	if _, err := fmt.Sscanf(lat, "%g", &la); err != nil {
		return nil, fmt.Errorf("bad latitude %q", lat)
	}
	if _, err := fmt.Sscanf(lng, "%g", &lo); err != nil {
		return nil, fmt.Errorf("bad longitude %q", lng)
	}
	c := geography.Coordinate{Latitude: la, Longitude: lo}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}
