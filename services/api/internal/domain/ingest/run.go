// Package ingest models durable source ingestion state. It deliberately keeps
// raw provider payloads out of the operational database: only an external
// reference and a one-way payload digest are retained.
package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type RecordOutcome string

const (
	RecordCreated  RecordOutcome = "created"
	RecordUpdated  RecordOutcome = "updated"
	RecordSkipped  RecordOutcome = "skipped"
	RecordRejected RecordOutcome = "rejected"
)

const (
	RunRetention       = 400 * 24 * time.Hour
	RawRecordRetention = 90 * 24 * time.Hour
	MaxRecordedErrors  = 100
)

var sha256Pattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type Error struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	RecordRef string    `json:"recordRef,omitempty"`
	At        time.Time `json:"at"`
}

type Run struct {
	ID                      string
	SourceID                string
	Status                  Status
	PayloadHash             string
	RequestID               string
	QueuedAt                time.Time
	StartedAt               *time.Time
	FinishedAt              *time.Time
	DurationMS              int64
	RecordsProcessed        int64
	Errors                  []Error
	ReconciliationConflicts int64
	DuplicateCandidates     int64
	ExpiresAt               time.Time
}

type RawRecord struct {
	ID          string
	RunID       string
	SourceID    string
	ExternalRef string
	PayloadHash string
	Outcome     RecordOutcome
	ReasonCode  string
	ProcessedAt time.Time
	ExpiresAt   time.Time
}

func NewRun(sourceID, payloadHash, requestID string, now time.Time) (Run, error) {
	sourceID, payloadHash, requestID = strings.TrimSpace(sourceID), strings.ToLower(strings.TrimSpace(payloadHash)), strings.TrimSpace(requestID)
	if sourceID == "" {
		return Run{}, errors.New("source id is required")
	}
	if payloadHash != "" && !sha256Pattern.MatchString(payloadHash) {
		return Run{}, errors.New("payload hash must be a lowercase SHA-256 digest")
	}
	idSeed := sourceID + "\x00" + payloadHash
	if payloadHash == "" {
		idSeed += "\x00" + requestID
	}
	if payloadHash == "" && requestID == "" {
		idSeed += "\x00" + now.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(idSeed))
	return Run{ID: "run_" + hex.EncodeToString(sum[:12]), SourceID: sourceID,
		Status: StatusQueued, PayloadHash: payloadHash, RequestID: requestID,
		QueuedAt: now.UTC(), ExpiresAt: now.UTC().Add(RunRetention)}, nil
}

func NewRawRecord(run Run, externalRef, payloadHash string, outcome RecordOutcome, reason string, now time.Time) (RawRecord, error) {
	externalRef, payloadHash = strings.TrimSpace(externalRef), strings.ToLower(strings.TrimSpace(payloadHash))
	if run.ID == "" || run.SourceID == "" || externalRef == "" {
		return RawRecord{}, errors.New("run, source and external record reference are required")
	}
	if !sha256Pattern.MatchString(payloadHash) {
		return RawRecord{}, errors.New("record payload hash must be a lowercase SHA-256 digest")
	}
	if outcome != RecordCreated && outcome != RecordUpdated && outcome != RecordSkipped && outcome != RecordRejected {
		return RawRecord{}, fmt.Errorf("invalid record outcome %q", outcome)
	}
	sum := sha256.Sum256([]byte(run.ID + "\x00" + externalRef))
	return RawRecord{ID: "src_" + hex.EncodeToString(sum[:16]), RunID: run.ID, SourceID: run.SourceID,
		ExternalRef: externalRef, PayloadHash: payloadHash, Outcome: outcome,
		ReasonCode: sanitizeReason(reason), ProcessedAt: now.UTC(), ExpiresAt: now.UTC().Add(RawRecordRetention)}, nil
}

// sanitizeReason keeps operational classifications useful without retaining
// provider values, email addresses or other source payload fragments.
func sanitizeReason(reason string) string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	switch {
	case reason == "":
		return ""
	case strings.HasPrefix(reason, "unresolved region"):
		return "unresolved_region"
	case strings.HasPrefix(reason, "write failed"):
		return "write_failed"
	case strings.HasPrefix(reason, "missing external id"):
		return "missing_external_id"
	case strings.HasPrefix(reason, "stable id"):
		return "invalid_external_id"
	default:
		return "validation_failed"
	}
}
