// Package adminops defines read-only operational projections for the steward API.
package adminops

import (
	"context"
	"errors"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
)

var ErrNotFound = errors.New("admin operation record not found")

type Page[T any] struct {
	Data       []T    `json:"data"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type Counts struct {
	Regions       int64 `json:"regions"`
	Districts     int64 `json:"districts"`
	Places        int64 `json:"places"`
	Roads         int64 `json:"roads"`
	POIs          int64 `json:"pois"`
	PendingOutbox int64 `json:"pendingOutbox"`
	DeadOutbox    int64 `json:"deadOutbox"`
	AuditEntries  int64 `json:"auditEntries"`
}

type Dashboard struct {
	Counts         Counts
	CurrentRelease *dataset.Version
	RecentActivity []audit.Entry
}

type Dependency struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Detail    string    `json:"detail"`
	LatencyMS int64     `json:"latencyMs"`
	CheckedAt time.Time `json:"checkedAt"`
}

// ProbeResult is the deliberately small, browser-safe result returned by a
// dependency probe. It must never contain connection strings, hostnames,
// credentials, raw server responses, or unbounded label sets.
type ProbeResult struct {
	Status        string
	Detail        string
	LatencyMS     int64
	HitRate       *float64
	DocumentCount *int64
}

type DependencyProbe interface {
	Probe(context.Context) ProbeResult
}

type RuntimeMetrics struct {
	RequestCount int64
	ErrorCount   int64
	CacheHits    int64
	CacheMisses  int64
}

type RuntimeMetricsReader interface {
	AdminMetrics() RuntimeMetrics
}

type Metric struct {
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Status    string  `json:"status"`
	Threshold string  `json:"threshold"`
	Detail    string  `json:"detail,omitempty"`
}

type Health struct {
	Status       string            `json:"status"`
	Dependencies []Dependency      `json:"dependencies"`
	Queue        map[string]int64  `json:"queue"`
	ETLFreshness *time.Time        `json:"etlFreshness,omitempty"`
	IndexStatus  string            `json:"indexStatus"`
	Metrics      map[string]Metric `json:"metrics"`
}

type SourceRun struct {
	ID                  string     `json:"id"`
	SourceID            string     `json:"sourceId"`
	Status              string     `json:"status"`
	PayloadHash         string     `json:"payloadHash,omitempty"`
	RequestID           string     `json:"requestId,omitempty"`
	Error               string     `json:"error,omitempty"`
	StartedAt           time.Time  `json:"startedAt"`
	QueuedAt            time.Time  `json:"queuedAt,omitempty"`
	FinishedAt          *time.Time `json:"finishedAt,omitempty"`
	DurationMS          int64      `json:"durationMs"`
	RecordsProcessed    int64      `json:"recordsProcessed"`
	Errors              []string   `json:"errors,omitempty"`
	Conflicts           int64      `json:"conflicts"`
	DuplicateCandidates int64      `json:"duplicateCandidates"`
	// DetailAvailability distinguishes durable imports from historical audit
	// summaries, whose per-record detail was never retained.
	DetailAvailability string `json:"detailAvailability"`
}

const (
	DetailDurable        = "durable"
	DetailHistoricalOnly = "historical_summary_only"
)

type SourceRecord struct {
	ID          string    `json:"id"`
	RunID       string    `json:"runId"`
	SourceID    string    `json:"sourceId"`
	ExternalRef string    `json:"externalRef"`
	PayloadHash string    `json:"payloadHash"`
	Outcome     string    `json:"outcome"`
	ReasonCode  string    `json:"reasonCode,omitempty"`
	ProcessedAt time.Time `json:"processedAt"`
}

type SourceRecordPage struct {
	Data               []SourceRecord `json:"data"`
	NextCursor         string         `json:"nextCursor,omitempty"`
	DetailAvailability string         `json:"detailAvailability"`
}
