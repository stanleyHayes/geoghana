package mongo

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
)

// ErrAuditImmutable is returned by every mutating path on the audit
// collection. It exists so an attempt to rewrite history fails loudly at the
// adapter, not silently at a code review that never happened.
var ErrAuditImmutable = errors.New("audit log is append-only: entries cannot be modified or deleted")

type auditDoc struct {
	ID        string    `bson:"_id"`
	At        time.Time `bson:"at"`
	ActorKind string    `bson:"actorKind"`
	ActorID   string    `bson:"actorId"`
	ActorName string    `bson:"actorLabel,omitempty"`
	ActorIP   string    `bson:"actorIp,omitempty"`
	Action    string    `bson:"action"`
	TargetKnd string    `bson:"targetKind"`
	TargetID  string    `bson:"targetId"`
	TargetLbl string    `bson:"targetLabel,omitempty"`
	RequestID string    `bson:"requestId,omitempty"`
	// Stored as canonical JSON strings, which is exactly what the hash covers.
	Before   string `bson:"before,omitempty"`
	After    string `bson:"after,omitempty"`
	Reason   string `bson:"reason,omitempty"`
	Outcome  string `bson:"outcome"`
	Error    string `bson:"error,omitempty"`
	PrevHash string `bson:"prevHash"`
	Hash     string `bson:"hash"`
}

func (d auditDoc) toDomain() audit.Entry {
	return audit.Entry{
		ID: d.ID, At: d.At,
		Actor: audit.Actor{
			Kind: audit.ActorKind(d.ActorKind), ID: d.ActorID,
			Label: d.ActorName, IP: d.ActorIP,
		},
		Action:    audit.Action(d.Action),
		Target:    audit.Target{Kind: d.TargetKnd, ID: d.TargetID, Label: d.TargetLbl},
		RequestID: d.RequestID,
		Before:    audit.ParsePayload(d.Before), After: audit.ParsePayload(d.After),
		BeforeJSON: d.Before, AfterJSON: d.After,
		Reason: d.Reason, Outcome: audit.Outcome(d.Outcome), Error: d.Error,
		PrevHash: d.PrevHash, Hash: d.Hash,
	}
}

// AuditRepo is the append-only audit store.
//
// It deliberately exposes no Update and no Delete. The only write is Append,
// and it uses InsertOne so a duplicate _id is an error rather than an
// overwrite — ReplaceOne with upsert would have made rewriting a row trivial.
type AuditRepo struct {
	s *Store
}

var auditAppendMu sync.Mutex

func NewAuditRepo(s *Store) *AuditRepo { return &AuditRepo{s: s} }

func (r *AuditRepo) col() *mongo.Collection { return r.s.db.Collection(ColAuditLog) }

// Append writes one entry, linked to the previous one by hash.
//
// The id and timestamp are assigned here so a caller cannot backdate a row or
// collide with an existing one, and the hash is computed AFTER redaction so
// the digest covers exactly the bytes stored — hashing the unredacted form
// would make every verification fail.
//
// Appends are serialised by a mutex. Two concurrent writers could otherwise
// read the same tail hash and produce a fork that verification would report
// as tampering. This is a single-writer log by nature; the lock makes that
// explicit rather than leaving it to luck.
func (r *AuditRepo) Append(ctx context.Context, e audit.Entry) (audit.Entry, error) {
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	return r.append(ctx, e)
}

// append writes under an existing auditAppendMu critical section. Transactional
// collaborators use this so their state change and its evidence commit together.
func (r *AuditRepo) append(ctx context.Context, e audit.Entry) (audit.Entry, error) {

	// Truncated to milliseconds because that is BSON's datetime resolution.
	// Hashing a nanosecond-precision timestamp would produce a digest the
	// stored row can never reproduce, so every verification would report
	// tampering on a log nobody had touched.
	e.At = time.Now().UTC().Truncate(time.Millisecond)
	e.ID = newAuditID(e.At)
	// Redaction on the way IN: a secret must never be written, not merely
	// hidden when read back.
	e.Before, e.After = audit.Redact(e.Before), audit.Redact(e.After)

	// Serialised BEFORE hashing, so the digest covers precisely the bytes the
	// database stores.
	var err error
	if e.BeforeJSON, err = audit.CanonicalJSON(e.Before); err != nil {
		return audit.Entry{}, err
	}
	if e.AfterJSON, err = audit.CanonicalJSON(e.After); err != nil {
		return audit.Entry{}, err
	}

	prev, err := r.tailHash(ctx)
	if err != nil {
		return audit.Entry{}, err
	}
	e.PrevHash = prev
	e.Hash = e.ComputeHash()

	d := auditDoc{
		ID: e.ID, At: e.At,
		ActorKind: string(e.Actor.Kind), ActorID: e.Actor.ID,
		ActorName: e.Actor.Label, ActorIP: e.Actor.IP,
		Action:    string(e.Action),
		TargetKnd: e.Target.Kind, TargetID: e.Target.ID, TargetLbl: e.Target.Label,
		RequestID: e.RequestID, Before: e.BeforeJSON, After: e.AfterJSON,
		Reason: e.Reason, Outcome: string(e.Outcome), Error: e.Error,
		PrevHash: e.PrevHash, Hash: e.Hash,
	}
	if _, err := r.col().InsertOne(ctx, d); err != nil {
		return audit.Entry{}, fmt.Errorf("append audit entry: %w", err)
	}
	return e, nil
}

// tailHash returns the hash of the most recent entry, or the genesis sentinel
// when the log is empty.
func (r *AuditRepo) tailHash(ctx context.Context) (string, error) {
	var d auditDoc
	err := r.col().FindOne(ctx, bson.M{},
		options.FindOne().SetSort(bson.D{{Key: "at", Value: -1}, {Key: "_id", Value: -1}}),
	).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return audit.GenesisHash, nil
	}
	if err != nil {
		return "", fmt.Errorf("read audit tail: %w", err)
	}
	return d.Hash, nil
}

// Chain returns every entry in CHRONOLOGICAL order for verification. Unlike
// List it is not filtered or limited: a chain check on a subset would report
// a false break at the first gap.
func (r *AuditRepo) Chain(ctx context.Context) ([]audit.Entry, error) {
	cur, err := r.col().Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "at", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("read audit chain: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []audit.Entry
	for cur.Next(ctx) {
		var d auditDoc
		if err := cur.Decode(&d); err != nil {
			return nil, fmt.Errorf("decode audit entry: %w", err)
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

// AuditFilter narrows a query over the log.
type AuditFilter struct {
	Actor  string
	Action string
	Target string
	Limit  int
}

// List returns entries newest first. Read-only by construction.
func (r *AuditRepo) List(ctx context.Context, f AuditFilter) ([]audit.Entry, error) {
	q := bson.M{}
	if f.Actor != "" {
		q["actorId"] = f.Actor
	}
	if f.Action != "" {
		q["action"] = f.Action
	}
	if f.Target != "" {
		q["targetId"] = f.Target
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	cur, err := r.col().Find(ctx, q,
		options.Find().SetSort(bson.D{{Key: "at", Value: -1}, {Key: "_id", Value: -1}}).
			SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []audit.Entry
	for cur.Next(ctx) {
		var d auditDoc
		if err := cur.Decode(&d); err != nil {
			return nil, fmt.Errorf("decode audit entry: %w", err)
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

// ChangesAfter returns successful geography mutations in chronological order.
// Audit ids are time ordered, so the last delivered id is a durable resume
// cursor without introducing a second mutable event store.
func (r *AuditRepo) ChangesAfter(ctx context.Context, cursor string, limit int) ([]audit.Entry, error) {
	q := bson.M{
		"outcome": string(audit.OutcomeSucceeded),
		"action": bson.M{"$in": bson.A{
			string(audit.ActionRecordUpdated), string(audit.ActionRecordDeprecated),
		}},
	}
	if cursor != "" {
		q["_id"] = bson.M{"$gt": cursor}
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	cur, err := r.col().Find(ctx, q, options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("read audit change feed: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()
	var out []audit.Entry
	for cur.Next(ctx) {
		var d auditDoc
		if err := cur.Decode(&d); err != nil {
			return nil, fmt.Errorf("decode audit change feed: %w", err)
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

// LatestChangeCursor returns the current tail of the public change feed.
func (r *AuditRepo) LatestChangeCursor(ctx context.Context) (string, error) {
	var d auditDoc
	err := r.col().FindOne(ctx, bson.M{
		"outcome": string(audit.OutcomeSucceeded),
		"action": bson.M{"$in": bson.A{
			string(audit.ActionRecordUpdated), string(audit.ActionRecordDeprecated),
		}},
	}, options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read audit change-feed tail: %w", err)
	}
	return d.ID, nil
}

// newAuditID is time-ordered so the natural _id sort matches chronological
// order, and random-suffixed so two entries written in the same nanosecond
// cannot collide — a collision would surface as an InsertOne duplicate-key
// error and lose the second entry.
func newAuditID(at time.Time) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Randomness is unavailable; a monotonic counter still avoids a
		// collision within the process, and losing the row would be worse.
		atomic.AddUint64(&auditSeq, 1)
		return fmt.Sprintf("aud_%d_%016x", at.UnixNano(), atomic.LoadUint64(&auditSeq))
	}
	return fmt.Sprintf("aud_%d_%s", at.UnixNano(), hex.EncodeToString(b[:]))
}

var auditSeq uint64
