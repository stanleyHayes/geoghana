// Package geonames ingests the GeoNames Ghana gazetteer.
//
// LICENCE: CC BY 4.0. Attribution is mandatory and is attached to every record
// this adapter produces, so it survives into API responses and downloads
// rather than living only in a footer (plan rule R4).
//
// GeoNames is a REFERENCE source, not an authority on Ghanaian administrative
// geography. Records land as REFERENCE and are never auto-promoted to
// canonical (rule R5); a steward reconciles them against GNHR/GSS.
package geonames

import (
	"bufio"
	"compress/flate"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/app/ingest"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

var _ = flate.BestSpeed // keep the import list honest if compression is added

// Column indexes in the GeoNames "geoname" table dump.
const (
	colGeonameID   = 0
	colName        = 1
	colASCIIName   = 2
	colAlternates  = 3
	colLatitude    = 4
	colLongitude   = 5
	colFeatureCls  = 6
	colFeatureCode = 7
	colCountry     = 8
	colAdmin1      = 10
	colAdmin2      = 11
	colPopulation  = 14
	minColumns     = 15
)

// admin1ToRegion maps GeoNames first-order codes to Ghana's 16 regions.
//
// Ghana reorganised from 10 regions to 16 in 2018. GeoNames carries the modern
// set (codes 12–18 are the new regions), and codes 03 and 07 are retired
// Brong Ahafo splits. An unmapped code is REJECTED rather than guessed: a
// place filed under the wrong region is worse than a place we did not import.
var admin1ToRegion = map[string]string{
	"01": "Greater Accra",
	"02": "Ashanti",
	"04": "Central",
	"05": "Eastern",
	"06": "Northern",
	"08": "Volta",
	"09": "Western",
	"10": "Upper East",
	"11": "Upper West",
	"12": "Ahafo",
	"13": "Bono",
	"14": "Bono East",
	"15": "North East",
	"16": "Oti",
	"17": "Savannah",
	"18": "Western North",
}

// featureToPlaceType maps GeoNames feature codes onto our human geography.
// Codes not listed are skipped, because guessing a type is a data-quality
// failure that then propagates into search ranking.
var featureToPlaceType = map[string]geography.PlaceType{
	"PPLC":  geography.PlaceCity,            // national capital
	"PPLA":  geography.PlaceRegionalCapital, // seat of a first-order division
	"PPLA2": geography.PlaceTown,            // seat of a second-order division
	"PPL":   geography.PlaceVillage,         // refined by population below
	"PPLX":  geography.PlaceSuburb,          // section of a populated place
	"PPLL":  geography.PlaceLocality,
}

// Abandoned and destroyed places are deliberately excluded: PPLQ and PPLW.
var excludedFeatureCodes = map[string]bool{"PPLQ": true, "PPLW": true}

type Adapter struct {
	// Path is the GeoNames country dump, e.g. GH.txt.
	Path string
	// MaxRecords caps a run. Zero means no cap.
	MaxRecords int
	// RetrievedAt identifies the exact local source snapshot consumed.
	RetrievedAt string
}

func New(path string) *Adapter {
	retrievedAt := time.Now().UTC().Format(time.RFC3339)
	if info, err := os.Stat(path); err == nil {
		retrievedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	return &Adapter{Path: path, RetrievedAt: retrievedAt}
}

func (a *Adapter) Name() string { return "geonames" }

func (a *Adapter) Licence() ingest.Licence {
	return ingest.Licence{
		SPDX:        "CC-BY-4.0",
		Name:        "Creative Commons Attribution 4.0",
		URL:         "https://creativecommons.org/licenses/by/4.0/",
		Attribution: "Contains data from GeoNames (https://www.geonames.org), licensed CC BY 4.0.",
		// CC BY permits redistribution provided attribution travels with it.
		Redistributable: true,
	}
}

var ErrNoFile = errors.New("geonames dump not found")

// Fetch streams the dump, emitting one normalized record per usable row.
// It never loads the whole file into memory: the Ghana dump is ~16k rows today
// but allCountries is 12 million, and this should not need rewriting then.
func (a *Adapter) Fetch(ctx context.Context, emit func(ingest.SourceRecord) error) error {
	f, err := os.Open(a.Path)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNoFile, a.Path)
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReaderSize(f, 1<<20))
	r.Comma = '\t'
	// GeoNames alternate names legitimately contain quotes; the dump is not
	// quoted CSV, so quoting must be disabled or rows silently merge.
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	count := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		row, rerr := r.Read()
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			// A single malformed row must not abort a 16k-row import.
			continue
		}
		if len(row) < minColumns {
			continue
		}
		rec, ok := a.toRecord(row)
		if !ok {
			continue
		}
		if err := emit(rec); err != nil {
			return err
		}
		count++
		if a.MaxRecords > 0 && count >= a.MaxRecords {
			return nil
		}
	}
}

func (a *Adapter) toRecord(row []string) (ingest.SourceRecord, bool) {
	if row[colCountry] != "GH" {
		return ingest.SourceRecord{}, false
	}
	if row[colFeatureCls] != "P" {
		return ingest.SourceRecord{}, false
	}
	code := row[colFeatureCode]
	if excludedFeatureCodes[code] {
		return ingest.SourceRecord{}, false
	}
	placeType, known := featureToPlaceType[code]
	if !known {
		return ingest.SourceRecord{}, false
	}

	region, mapped := admin1ToRegion[row[colAdmin1]]
	if !mapped {
		return ingest.SourceRecord{}, false
	}

	lat, err1 := strconv.ParseFloat(row[colLatitude], 64)
	lng, err2 := strconv.ParseFloat(row[colLongitude], 64)
	if err1 != nil || err2 != nil {
		return ingest.SourceRecord{}, false
	}
	coord := geography.Coordinate{Latitude: lat, Longitude: lng}
	// Reject anything outside Ghana's bounding box. GeoNames is generally
	// reliable, but a mis-signed longitude would place a town in the Gulf of
	// Guinea and silently corrupt reverse geocoding.
	if err := coord.Validate(); err != nil {
		return ingest.SourceRecord{}, false
	}

	name := strings.TrimSpace(row[colName])
	if name == "" {
		name = strings.TrimSpace(row[colASCIIName])
	}
	if name == "" {
		return ingest.SourceRecord{}, false
	}

	var population *int64
	if p, err := strconv.ParseInt(row[colPopulation], 10, 64); err == nil && p > 0 {
		population = &p
		// A "village" with a real population is a town; the feature code alone
		// under-describes larger settlements.
		if placeType == geography.PlaceVillage && p >= 20000 {
			placeType = geography.PlaceTown
		}
	}

	payloadHash := sha256.Sum256([]byte(strings.Join(row, "\t")))
	retrievedAt := a.RetrievedAt
	if retrievedAt == "" {
		retrievedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return ingest.SourceRecord{
		ExternalID: row[colGeonameID],
		Name:       name,
		Aliases:    parseAliases(row[colAlternates], name),
		Type:       placeType,
		Coordinate: &coord,
		Population: population,
		RegionHint: region,
		Provenance: geography.Provenance{
			SourceID: "geonames", ExternalID: row[colGeonameID],
			SourceURL:   "https://www.geonames.org/" + row[colGeonameID],
			RetrievedAt: retrievedAt, SourcePayloadHash: hex.EncodeToString(payloadHash[:]),
			Notes: "CC BY 4.0 — GeoNames",
		},
	}, true
}

// parseAliases splits the comma-separated alternate-name field, dropping
// duplicates of the canonical name and anything empty. These are what let a
// user find "Kwabɛnya" by typing "Kwabenya", so they matter.
func parseAliases(raw, canonical string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	seen := map[string]bool{strings.ToLower(canonical): true}
	var out []string
	for _, a := range strings.Split(raw, ",") {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		key := strings.ToLower(a)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, a)
		// Cap the alias list: a handful of GeoNames entries carry dozens of
		// transliterations, which bloat the search index for no gain.
		if len(out) >= 8 {
			break
		}
	}
	return out
}
