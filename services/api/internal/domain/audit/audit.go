// Package audit models the immutable record of privileged actions
// (Spec §12.4 and §17, story GEO-9.7).
//
// The central property is that tampering is DETECTABLE. That is a deliberately
// weaker and more honest claim than "impossible": anyone holding the database
// credentials can edit a collection, and no application-level control changes
// that. A direct `db.audit_log.updateOne(...)` succeeds, and pretending
// otherwise would be the dangerous kind of security theatre.
//
// So the log defends in depth:
//   - Nothing here mutates an Entry; every builder returns a copy.
//   - The repository exposes Append and List, and no Update or Delete.
//   - A JSON Schema validator rejects a row missing actor, action or target.
//   - Each entry carries the hash of the one before it, so ANY later edit,
//     insertion or deletion breaks the chain and `audit verify` reports where.
//
// The chain is what makes the first three meaningful. Without it an operator
// with database access could rewrite history silently; with it they cannot do
// so without leaving evidence.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrNoActor  = errors.New("audit entry needs an actor")
	ErrNoAction = errors.New("audit entry needs an action")
	ErrNoTarget = errors.New("audit entry needs a target")
)

// ActorKind distinguishes who performed an action. An operator running the CLI
// and a request arriving with an API key are different accountability stories,
// and collapsing them would make the log useless for a security review.
type ActorKind string

const (
	// ActorOperator is a human running the admin CLI on a trusted machine.
	ActorOperator ActorKind = "operator"
	// ActorAdmin is an authenticated admin console user.
	ActorAdmin ActorKind = "admin"
	// ActorAPIKey is an authenticated API caller.
	ActorAPIKey ActorKind = "api_key"
	// ActorSystem is a scheduled or automated process with no human behind it.
	ActorSystem ActorKind = "system"
)

// Actor identifies who did something.
type Actor struct {
	Kind ActorKind
	// ID is the key id, user id or process name. Never a secret: an audit row
	// is read far more widely than the credential it describes.
	ID string
	// Label is a human-readable name, denormalized so the log stays readable
	// after the referenced record is renamed or deleted.
	Label string
	IP    string
}

// Action is what was done. Values are stable strings because they are queried
// and alerted on; renaming one silently breaks every saved search.
type Action string

const (
	ActionDatasetPublished Action = "dataset.published"
	ActionDatasetExported  Action = "dataset.exported"
	ActionDatasetRolledBk  Action = "dataset.rolled_back"
	ActionKeyCreated       Action = "key.created"
	ActionKeyRevoked       Action = "key.revoked"
	ActionSeedImported     Action = "data.seed_imported"
	ActionSourceImported   Action = "data.source_imported"
	ActionBoundariesLoaded Action = "data.boundaries_loaded"
	ActionPlacesDeduped    Action = "data.deduped"
	ActionDistrictsAssign  Action = "data.districts_assigned"
	ActionReindexed        Action = "search.reindexed"
	ActionRecordUpdated    Action = "record.updated"
	ActionRecordDeprecated Action = "record.deprecated"
)

// Target is what the action was performed on.
type Target struct {
	// Kind is the entity type: "dataset", "api_key", "place", "district"…
	Kind string
	ID   string
	// Label is denormalized for the same reason as Actor.Label.
	Label string
}

// Entry is one immutable audit row.
//
// Fields are exported for construction and reading, but nothing in this
// package mutates an Entry after New returns it, and the repository never
// offers a way to replace one.
type Entry struct {
	ID     string
	At     time.Time
	Actor  Actor
	Action Action
	Target Target
	// RequestID ties the row to the access log for the same request. Empty for
	// CLI actions, which have no HTTP request behind them.
	RequestID string
	// Before and After capture the change. Both nil for an action that creates
	// or reads nothing; Before nil for a creation; After nil for a removal.
	//
	// BeforeJSON and AfterJSON are the CANONICAL serialisations, and they —
	// not the maps — are what the hash covers and what the database stores.
	//
	// The maps cannot be hashed directly: Go values do not round-trip through
	// BSON unchanged ([]string returns as an array of any, int as int32), so a
	// digest taken over the map before writing never matches one taken after
	// reading, and every verification would report tampering on an untouched
	// log. One serialised representation removes the drift entirely.
	Before     map[string]any
	After      map[string]any
	BeforeJSON string
	AfterJSON  string
	// Reason is required by policy for actions a steward takes by discretion,
	// such as raising a limit on documented need (§24 F4).
	Reason string
	// Outcome records whether the action succeeded. A FAILED privileged action
	// is often the more interesting row — a rejected revocation attempt is a
	// security signal, and dropping it would hide exactly what matters.
	Outcome Outcome
	Error   string

	// PrevHash is the Hash of the entry written immediately before this one,
	// and Hash covers this entry's own content plus PrevHash. Together they
	// chain the log: editing, inserting or removing any row breaks every hash
	// after it, which is what makes tampering detectable rather than silent.
	PrevHash string
	Hash     string
}

// GenesisHash starts the chain. The first entry has no predecessor, and using
// a fixed sentinel rather than an empty string means a truncation that removes
// entry #1 is still detectable — the new first row would not carry it.
const GenesisHash = "genesis"

// Canonical renders the entry as deterministic bytes for hashing.
//
// Field order and encoding are FIXED here rather than delegated to a struct
// marshaller: Go map iteration is randomised, so encoding/json over Before and
// After would produce a different digest each run and every verification would
// fail. Nested maps are rendered with sorted keys for the same reason.
func (e Entry) Canonical() []byte {
	var b strings.Builder
	b.WriteString(e.ID)
	b.WriteByte(0x1f)
	b.WriteString(e.At.UTC().Format(time.RFC3339Nano))
	b.WriteByte(0x1f)
	b.WriteString(string(e.Actor.Kind) + "|" + e.Actor.ID + "|" + e.Actor.Label + "|" + e.Actor.IP)
	b.WriteByte(0x1f)
	b.WriteString(string(e.Action))
	b.WriteByte(0x1f)
	b.WriteString(e.Target.Kind + "|" + e.Target.ID + "|" + e.Target.Label)
	b.WriteByte(0x1f)
	b.WriteString(e.RequestID)
	b.WriteByte(0x1f)
	b.WriteString(e.BeforeJSON)
	b.WriteByte(0x1f)
	b.WriteString(e.AfterJSON)
	b.WriteByte(0x1f)
	b.WriteString(e.Reason)
	b.WriteByte(0x1f)
	b.WriteString(string(e.Outcome) + "|" + e.Error)
	b.WriteByte(0x1f)
	b.WriteString(e.PrevHash)
	return []byte(b.String())
}

// CanonicalJSON renders a map with sorted keys so the same content always
// produces the same bytes. encoding/json already sorts map keys, but going
// through it explicitly also normalises Go numeric types to JSON numbers,
// which is what makes the value survive a BSON round trip unchanged.
func CanonicalJSON(m map[string]any) (string, error) {
	if len(m) == 0 {
		return "", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("canonicalise audit payload: %w", err)
	}
	return string(b), nil
}

// ParsePayload decodes a stored canonical payload back into a map for display.
func ParsePayload(s string) map[string]any {
	if s == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// ComputeHash returns the digest for this entry given its predecessor.
func (e Entry) ComputeHash() string {
	sum := sha256.Sum256(e.Canonical())
	return hex.EncodeToString(sum[:])
}

// VerifyChain walks entries in CHRONOLOGICAL order and reports the first that
// does not verify, along with why.
//
// It reports the index rather than stopping at a boolean so an operator can
// see exactly where history diverges — the row before the break is the last
// one that can be trusted.
func VerifyChain(entries []Entry) error {
	prev := GenesisHash
	for i, e := range entries {
		if e.PrevHash != prev {
			return fmt.Errorf(
				"audit chain broken at entry %d (%s, %s): expected previous hash %s, found %s "+
					"— a row was edited, inserted or removed at or before this point",
				i, e.ID, e.Action, prev, e.PrevHash)
		}
		if want := e.ComputeHash(); want != e.Hash {
			return fmt.Errorf(
				"audit entry %d (%s, %s) was modified after it was written: "+
					"content hashes to %s but the row records %s",
				i, e.ID, e.Action, want, e.Hash)
		}
		prev = e.Hash
	}
	return nil
}

type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
)

// New builds a valid entry, or returns an error explaining what is missing.
//
// It refuses an incomplete row rather than writing a partial one: an audit
// entry that cannot say who did what to which thing is not evidence.
func New(a Actor, action Action, t Target) (Entry, error) {
	if a.Kind == "" || a.ID == "" {
		return Entry{}, fmt.Errorf("%w: kind=%q id=%q", ErrNoActor, a.Kind, a.ID)
	}
	if strings.TrimSpace(string(action)) == "" {
		return Entry{}, ErrNoAction
	}
	if t.Kind == "" || t.ID == "" {
		return Entry{}, fmt.Errorf("%w: kind=%q id=%q", ErrNoTarget, t.Kind, t.ID)
	}
	return Entry{
		Actor: a, Action: action, Target: t, Outcome: OutcomeSucceeded,
	}, nil
}

// WithChange records the before and after state.
func (e Entry) WithChange(before, after map[string]any) Entry {
	e.Before, e.After = before, after
	return e
}

// WithRequest ties the entry to an HTTP request.
func (e Entry) WithRequest(requestID string) Entry {
	e.RequestID = requestID
	return e
}

// WithReason records why a discretionary action was taken.
func (e Entry) WithReason(r string) Entry {
	e.Reason = r
	return e
}

// Failed marks the action as attempted and rejected.
func (e Entry) Failed(err error) Entry {
	e.Outcome = OutcomeFailed
	if err != nil {
		e.Error = err.Error()
	}
	return e
}

// Redact removes values that must never reach an audit row.
//
// Audit entries are read by more people than the records they describe, and a
// before/after snapshot of a credential would put a secret in front of every
// one of them. Redaction happens on the way IN, so the secret is never
// written rather than merely hidden on the way out.
func Redact(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	const mask = "[redacted]"
	sensitive := []string{"secret", "password", "token", "hash", "key", "credential", "authorization"}
	out := make(map[string]any, len(m))
	for k, v := range m {
		lower := strings.ToLower(k)
		masked := false
		for _, s := range sensitive {
			if strings.Contains(lower, s) {
				out[k] = mask
				masked = true
				break
			}
		}
		if masked {
			continue
		}
		// Nested maps are redacted too; a secret one level down is still a secret.
		if nested, ok := v.(map[string]any); ok {
			out[k] = Redact(nested)
			continue
		}
		out[k] = v
	}
	return out
}
