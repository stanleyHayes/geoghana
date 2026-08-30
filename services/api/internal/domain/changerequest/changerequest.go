// Package changerequest models reviewed proposals without mutating canonical data.
package changerequest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	ErrNotFound          = errors.New("change request not found")
	ErrInvalidTransition = errors.New("invalid change-request transition")
	ErrConflict          = errors.New("change request was modified concurrently")
	ErrInvalid           = errors.New("invalid change request")
)

type State string

const (
	StateSubmitted        State = "submitted"
	StateInReview         State = "in_review"
	StateChangesRequested State = "changes_requested"
	StateApproved         State = "approved"
	StateRejected         State = "rejected"
)

func (s State) Valid() bool {
	switch s {
	case StateSubmitted, StateInReview, StateChangesRequested, StateApproved, StateRejected:
		return true
	default:
		return false
	}
}

// CanTransition is the complete lifecycle allow-list. Terminal decisions cannot
// be reopened; a revised proposal returns to submitted for a fresh review.
func CanTransition(from, to State) bool {
	switch from {
	case StateSubmitted:
		return to == StateInReview
	case StateInReview:
		return to == StateChangesRequested || to == StateApproved || to == StateRejected
	case StateChangesRequested:
		return to == StateSubmitted
	default:
		return false
	}
}

type Target struct {
	Kind string
	ID   string
}

// Evidence contains references to durable import/reconciliation evidence. It
// intentionally does not duplicate raw source payloads or canonical records.
type Evidence struct {
	SourceID                  string
	SourceRecordIDs           []string
	DuplicateCandidateIDs     []string
	ReconciliationConflictIDs []string
	References                []string
}

func (e Evidence) Empty() bool {
	return strings.TrimSpace(e.SourceID) == "" && len(e.SourceRecordIDs) == 0 &&
		len(e.DuplicateCandidateIDs) == 0 && len(e.ReconciliationConflictIDs) == 0 && len(e.References) == 0
}

type Snapshot struct {
	BeforeJSON string
	AfterJSON  string
	Digest     string
}

func NewSnapshot(before, after map[string]any) (Snapshot, error) {
	if before == nil || after == nil {
		return Snapshot{}, fmt.Errorf("%w: before and after snapshots are required", ErrInvalid)
	}
	b, err := json.Marshal(before)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: encode before snapshot: %v", ErrInvalid, err)
	}
	a, err := json.Marshal(after)
	if err != nil {
		return Snapshot{}, fmt.Errorf("%w: encode after snapshot: %v", ErrInvalid, err)
	}
	if string(b) == string(a) {
		return Snapshot{}, fmt.Errorf("%w: proposal does not change the target", ErrInvalid)
	}
	sum := sha256.Sum256(append(append(append([]byte{}, b...), 0), a...))
	return Snapshot{BeforeJSON: string(b), AfterJSON: string(a), Digest: hex.EncodeToString(sum[:])}, nil
}

func (s Snapshot) Before() map[string]any { return decodeSnapshot(s.BeforeJSON) }
func (s Snapshot) After() map[string]any  { return decodeSnapshot(s.AfterJSON) }

func decodeSnapshot(raw string) map[string]any {
	var value map[string]any
	_ = json.Unmarshal([]byte(raw), &value)
	return value
}

type Comment struct {
	ID        string
	AuthorID  string
	Body      string
	RequestID string
	CreatedAt time.Time
}

type HistoryEvent struct {
	ID        string
	From      State
	To        State
	ActorID   string
	Comment   string
	RequestID string
	CreatedAt time.Time
}

// Revision is append-only: resubmission never overwrites what a reviewer saw.
type Revision struct {
	Number        int64
	ProposedPatch map[string]any
	Snapshot      Snapshot
	Evidence      Evidence
	ActorID       string
	CreatedAt     time.Time
}

type ChangeRequest struct {
	ID            string
	Target        Target
	ProposedPatch map[string]any
	Snapshot      Snapshot
	Evidence      Evidence
	SubmitterID   string
	ReviewerID    string
	State         State
	Comments      []Comment
	History       []HistoryEvent
	Revisions     []Revision
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func New(target Target, patch map[string]any, before, after map[string]any, evidence Evidence, submitterID string, now time.Time) (ChangeRequest, error) {
	target.Kind, target.ID, submitterID = strings.TrimSpace(target.Kind), strings.TrimSpace(target.ID), strings.TrimSpace(submitterID)
	if target.Kind == "" || target.ID == "" || submitterID == "" || len(patch) == 0 || evidence.Empty() {
		return ChangeRequest{}, fmt.Errorf("%w: target, patch, evidence, and submitter are required", ErrInvalid)
	}
	if !patchMatches(before, patch, after) {
		return ChangeRequest{}, fmt.Errorf("%w: proposed patch does not produce the after snapshot", ErrInvalid)
	}
	snapshot, err := NewSnapshot(before, after)
	if err != nil {
		return ChangeRequest{}, err
	}
	now = now.UTC()
	id := "cr_" + ulid.Make().String()
	event := HistoryEvent{ID: "che_" + ulid.Make().String(), To: StateSubmitted, ActorID: submitterID, CreatedAt: now}
	evidence = cloneEvidence(evidence)
	revision := Revision{Number: 1, ProposedPatch: cloneMap(patch), Snapshot: snapshot, Evidence: evidence, ActorID: submitterID, CreatedAt: now}
	return ChangeRequest{ID: id, Target: target, ProposedPatch: cloneMap(patch), Snapshot: snapshot, Evidence: evidence,
		SubmitterID: submitterID, State: StateSubmitted, History: []HistoryEvent{event}, Revisions: []Revision{revision}, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}

func cloneMap(in map[string]any) map[string]any {
	b, _ := json.Marshal(in)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

// patchMatches applies JSON Merge Patch object semantics (RFC 7396) and
// verifies that the transport-provided after image is not a second, conflicting
// account of the proposed change.
func patchMatches(before, patch, after map[string]any) bool {
	merged := cloneMap(before)
	applyMergePatch(merged, patch)
	want, err1 := json.Marshal(merged)
	got, err2 := json.Marshal(after)
	return err1 == nil && err2 == nil && string(want) == string(got)
}

func applyMergePatch(target, patch map[string]any) {
	for key, value := range patch {
		if value == nil {
			delete(target, key)
			continue
		}
		nested, isObject := value.(map[string]any)
		if !isObject {
			target[key] = value
			continue
		}
		base, _ := target[key].(map[string]any)
		if base == nil {
			base = map[string]any{}
		} else {
			base = cloneMap(base)
		}
		applyMergePatch(base, nested)
		target[key] = base
	}
}

func cloneEvidence(e Evidence) Evidence {
	e.SourceRecordIDs = append([]string{}, e.SourceRecordIDs...)
	e.DuplicateCandidateIDs = append([]string{}, e.DuplicateCandidateIDs...)
	e.ReconciliationConflictIDs = append([]string{}, e.ReconciliationConflictIDs...)
	e.References = append([]string{}, e.References...)
	return e
}

func (c ChangeRequest) Transition(to State, actorID, comment, requestID string, now time.Time) (ChangeRequest, error) {
	actorID = strings.TrimSpace(actorID)
	if to == StateSubmitted && c.State == StateChangesRequested {
		return ChangeRequest{}, fmt.Errorf("%w: resubmission requires an appended revision", ErrInvalidTransition)
	}
	if actorID == "" || !to.Valid() || !CanTransition(c.State, to) {
		return ChangeRequest{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, c.State, to)
	}
	if (to == StateChangesRequested || to == StateRejected) && strings.TrimSpace(comment) == "" {
		return ChangeRequest{}, fmt.Errorf("%w: %s requires a reviewer comment", ErrInvalid, to)
	}
	next := c
	if to == StateInReview || to == StateApproved || to == StateRejected || to == StateChangesRequested {
		if c.SubmitterID == actorID {
			return ChangeRequest{}, fmt.Errorf("%w: submitters cannot review their own proposal", ErrInvalidTransition)
		}
		if c.ReviewerID != "" && c.ReviewerID != actorID {
			return ChangeRequest{}, fmt.Errorf("%w: review is assigned to another reviewer", ErrConflict)
		}
		next.ReviewerID = actorID
	}
	now = now.UTC()
	next.State = to
	next.Version++
	next.UpdatedAt = now
	next.History = append(append([]HistoryEvent{}, c.History...), HistoryEvent{ID: "che_" + ulid.Make().String(), From: c.State, To: to, ActorID: actorID, Comment: strings.TrimSpace(comment), RequestID: strings.TrimSpace(requestID), CreatedAt: now})
	return next, nil
}

func (c ChangeRequest) AddComment(actorID, body, requestID string, now time.Time) (ChangeRequest, error) {
	actorID, body = strings.TrimSpace(actorID), strings.TrimSpace(body)
	if actorID == "" || body == "" {
		return ChangeRequest{}, fmt.Errorf("%w: comment author and body are required", ErrInvalid)
	}
	next := c
	next.Version++
	next.UpdatedAt = now.UTC()
	next.Comments = append(append([]Comment{}, c.Comments...), Comment{ID: "chc_" + ulid.Make().String(), AuthorID: actorID, Body: body, RequestID: strings.TrimSpace(requestID), CreatedAt: now.UTC()})
	return next, nil
}

// Revise resubmits a changes-requested proposal as a new immutable revision.
func (c ChangeRequest) Revise(actorID string, patch, after map[string]any, evidence Evidence, comment, requestID string, now time.Time) (ChangeRequest, error) {
	actorID = strings.TrimSpace(actorID)
	if c.State != StateChangesRequested || actorID == "" || actorID != c.SubmitterID || len(patch) == 0 || evidence.Empty() {
		return ChangeRequest{}, fmt.Errorf("%w: only the submitter may revise a changes-requested proposal", ErrInvalidTransition)
	}
	if !patchMatches(c.Snapshot.Before(), patch, after) {
		return ChangeRequest{}, fmt.Errorf("%w: proposed patch does not produce the after snapshot", ErrInvalid)
	}
	snapshot, err := NewSnapshot(c.Snapshot.Before(), after)
	if err != nil {
		return ChangeRequest{}, err
	}
	now = now.UTC()
	next := c
	next.ProposedPatch = cloneMap(patch)
	next.Snapshot = snapshot
	next.Evidence = cloneEvidence(evidence)
	next.ReviewerID = ""
	next.State = StateSubmitted
	next.Version++
	next.UpdatedAt = now
	next.Revisions = append(append([]Revision{}, c.Revisions...), Revision{Number: int64(len(c.Revisions) + 1), ProposedPatch: cloneMap(patch), Snapshot: snapshot, Evidence: cloneEvidence(evidence), ActorID: actorID, CreatedAt: now})
	next.History = append(append([]HistoryEvent{}, c.History...), HistoryEvent{ID: "che_" + ulid.Make().String(), From: c.State, To: StateSubmitted, ActorID: actorID, Comment: strings.TrimSpace(comment), RequestID: strings.TrimSpace(requestID), CreatedAt: now})
	return next, nil
}
