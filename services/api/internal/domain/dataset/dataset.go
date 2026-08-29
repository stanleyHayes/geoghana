// Package dataset models published dataset versions and their downloadable
// artifacts (Spec §7.2, story GEO-8.3).
//
// A download is only listed once it EXISTS on disk with a checksum computed
// from its actual bytes. A catalogue that advertises a file nobody generated
// is worse than an empty one: a consumer builds a pipeline against it and
// discovers the gap in production.
package dataset

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotFound       = errors.New("dataset version not found")
	ErrNotPublished   = errors.New("dataset version is not published")
	ErrUnknownFormat  = errors.New("unknown artifact format")
	ErrBadArtifactRef = errors.New("artifact reference is not safe")
)

// Status is the release lifecycle of a dataset version.
type Status string

const (
	StatusDraft      Status = "draft"
	StatusValidation Status = "validation"
	StatusReview     Status = "review"
	StatusApproved   Status = "approved"
	StatusPublished  Status = "published"
	StatusRolledBack Status = "rolled_back"
)

// Published reports whether this version may be served to the public. Only a
// published version appears in /datasets.
func (s Status) Published() bool { return s == StatusPublished }

type Format string

const (
	FormatGeoJSON Format = "geojson"
	FormatCSV     Format = "csv"
	FormatJSON    Format = "json"
)

var validFormats = map[Format]struct{}{
	FormatGeoJSON: {}, FormatCSV: {}, FormatJSON: {},
}

func ParseFormat(s string) (Format, error) {
	f := Format(strings.ToLower(strings.TrimSpace(s)))
	if _, ok := validFormats[f]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownFormat, s)
	}
	return f, nil
}

// ContentType is what the download endpoint must send. GeoJSON has its own
// media type, and sending it as application/json loses information a mapping
// client uses to decide how to handle the body.
func (f Format) ContentType() string {
	switch f {
	case FormatGeoJSON:
		return "application/geo+json"
	case FormatCSV:
		return "text/csv; charset=utf-8"
	default:
		return "application/json; charset=utf-8"
	}
}

// Artifact is one downloadable file belonging to a version.
type Artifact struct {
	// Entity is what the file contains: "regions", "districts" or "places".
	Entity string
	Format Format
	// Filename is a BASE NAME only — never a path. Download requests resolve
	// against it, so a separator or "…" here would be a traversal.
	Filename    string
	SizeBytes   int64
	SHA256      string
	RecordCount int64
}

// SafeFilename rejects anything that could escape the export directory. It is
// enforced on write as well as read: a malformed row in the database must not
// become a path traversal at download time.
func (a Artifact) SafeFilename() error {
	n := a.Filename
	if n == "" || n == "." || n == ".." ||
		strings.ContainsAny(n, `/\`) || strings.Contains(n, "..") {
		return fmt.Errorf("%w: %q", ErrBadArtifactRef, n)
	}
	return nil
}

// Version is a published snapshot of the canonical dataset.
type Version struct {
	Version     string
	Status      Status
	PublishedAt string
	Changelog   string
	Counts      map[string]int64
	Artifacts   []Artifact
}

// Attribution travels with every dataset response, because CC BY is not
// satisfied by a footer on a website (Spec §7.2).
const Attribution = "Contains data from GeoNames (https://www.geonames.org) and " +
	"geoBoundaries (https://www.geoboundaries.org), licensed CC BY 4.0."

// Licence is the licence the dataset itself is published under.
const Licence = "CC-BY-4.0"

// FindArtifact returns the artifact matching an entity and format.
func (v Version) FindArtifact(entity string, f Format) (Artifact, error) {
	for _, a := range v.Artifacts {
		if a.Entity == entity && a.Format == f {
			return a, nil
		}
	}
	return Artifact{}, ErrNotFound
}
