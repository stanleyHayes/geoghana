package mongo

import (
	"context"
	"errors"
	"fmt"

	app "github.com/ghanageo/ghanageo/services/api/internal/app/changerequest"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/changerequest"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type changeTargetDoc struct {
	Kind string `bson:"kind"`
	ID   string `bson:"id"`
}
type changeEvidenceDoc struct {
	SourceID                  string   `bson:"sourceId,omitempty"`
	SourceRecordIDs           []string `bson:"sourceRecordIds,omitempty"`
	DuplicateCandidateIDs     []string `bson:"duplicateCandidateIds,omitempty"`
	ReconciliationConflictIDs []string `bson:"reconciliationConflictIds,omitempty"`
	References                []string `bson:"references,omitempty"`
}
type changeSnapshotDoc struct {
	BeforeJSON string `bson:"beforeJson"`
	AfterJSON  string `bson:"afterJson"`
	Digest     string `bson:"digest"`
}
type changeCommentDoc struct {
	ID        string        `bson:"id"`
	AuthorID  string        `bson:"authorId"`
	Body      string        `bson:"body"`
	RequestID string        `bson:"requestId,omitempty"`
	CreatedAt bson.DateTime `bson:"createdAt"`
}
type changeHistoryDoc struct {
	ID        string        `bson:"id"`
	From      string        `bson:"from,omitempty"`
	To        string        `bson:"to"`
	ActorID   string        `bson:"actorId"`
	Comment   string        `bson:"comment,omitempty"`
	RequestID string        `bson:"requestId,omitempty"`
	CreatedAt bson.DateTime `bson:"createdAt"`
}
type changeRevisionDoc struct {
	Number        int64             `bson:"number"`
	ProposedPatch map[string]any    `bson:"proposedPatch"`
	Snapshot      changeSnapshotDoc `bson:"snapshot"`
	Evidence      changeEvidenceDoc `bson:"evidence"`
	ActorID       string            `bson:"actorId"`
	CreatedAt     bson.DateTime     `bson:"createdAt"`
}
type changeRequestDoc struct {
	ID            string              `bson:"_id"`
	Target        changeTargetDoc     `bson:"target"`
	ProposedPatch map[string]any      `bson:"proposedPatch"`
	Snapshot      changeSnapshotDoc   `bson:"snapshot"`
	Evidence      changeEvidenceDoc   `bson:"evidence"`
	SubmitterID   string              `bson:"submitterId"`
	ReviewerID    string              `bson:"reviewerId,omitempty"`
	State         string              `bson:"state"`
	Comments      []changeCommentDoc  `bson:"comments"`
	History       []changeHistoryDoc  `bson:"history"`
	Revisions     []changeRevisionDoc `bson:"revisions"`
	Version       int64               `bson:"version"`
	RequestID     string              `bson:"requestId,omitempty"`
	CreatedAt     bson.DateTime       `bson:"createdAt"`
	UpdatedAt     bson.DateTime       `bson:"updatedAt"`
}

func changeRequestToDoc(c domain.ChangeRequest, requestID string) changeRequestDoc {
	d := changeRequestDoc{ID: c.ID, Target: changeTargetDoc{c.Target.Kind, c.Target.ID}, ProposedPatch: c.ProposedPatch,
		Snapshot: changeSnapshotDoc{c.Snapshot.BeforeJSON, c.Snapshot.AfterJSON, c.Snapshot.Digest}, Evidence: changeEvidenceDoc{c.Evidence.SourceID, c.Evidence.SourceRecordIDs, c.Evidence.DuplicateCandidateIDs, c.Evidence.ReconciliationConflictIDs, c.Evidence.References},
		SubmitterID: c.SubmitterID, ReviewerID: c.ReviewerID, State: string(c.State), Version: c.Version, RequestID: requestID, CreatedAt: bson.NewDateTimeFromTime(c.CreatedAt), UpdatedAt: bson.NewDateTimeFromTime(c.UpdatedAt)}
	for _, x := range c.Comments {
		d.Comments = append(d.Comments, changeCommentDoc{x.ID, x.AuthorID, x.Body, x.RequestID, bson.NewDateTimeFromTime(x.CreatedAt)})
	}
	for _, x := range c.History {
		d.History = append(d.History, changeHistoryDoc{x.ID, string(x.From), string(x.To), x.ActorID, x.Comment, x.RequestID, bson.NewDateTimeFromTime(x.CreatedAt)})
	}
	for _, x := range c.Revisions {
		d.Revisions = append(d.Revisions, changeRevisionDoc{x.Number, x.ProposedPatch, changeSnapshotDoc{x.Snapshot.BeforeJSON, x.Snapshot.AfterJSON, x.Snapshot.Digest}, changeEvidenceDoc{x.Evidence.SourceID, x.Evidence.SourceRecordIDs, x.Evidence.DuplicateCandidateIDs, x.Evidence.ReconciliationConflictIDs, x.Evidence.References}, x.ActorID, bson.NewDateTimeFromTime(x.CreatedAt)})
	}
	if d.Comments == nil {
		d.Comments = []changeCommentDoc{}
	}
	if d.History == nil {
		d.History = []changeHistoryDoc{}
	}
	if d.Revisions == nil {
		d.Revisions = []changeRevisionDoc{}
	}
	return d
}
func (d changeRequestDoc) domain() domain.ChangeRequest {
	c := domain.ChangeRequest{ID: d.ID, Target: domain.Target{Kind: d.Target.Kind, ID: d.Target.ID}, ProposedPatch: d.ProposedPatch, Snapshot: domain.Snapshot{BeforeJSON: d.Snapshot.BeforeJSON, AfterJSON: d.Snapshot.AfterJSON, Digest: d.Snapshot.Digest},
		Evidence: domain.Evidence{SourceID: d.Evidence.SourceID, SourceRecordIDs: d.Evidence.SourceRecordIDs, DuplicateCandidateIDs: d.Evidence.DuplicateCandidateIDs, ReconciliationConflictIDs: d.Evidence.ReconciliationConflictIDs, References: d.Evidence.References}, SubmitterID: d.SubmitterID, ReviewerID: d.ReviewerID, State: domain.State(d.State), Version: d.Version, CreatedAt: d.CreatedAt.Time(), UpdatedAt: d.UpdatedAt.Time()}
	for _, x := range d.Comments {
		c.Comments = append(c.Comments, domain.Comment{ID: x.ID, AuthorID: x.AuthorID, Body: x.Body, RequestID: x.RequestID, CreatedAt: x.CreatedAt.Time()})
	}
	for _, x := range d.History {
		c.History = append(c.History, domain.HistoryEvent{ID: x.ID, From: domain.State(x.From), To: domain.State(x.To), ActorID: x.ActorID, Comment: x.Comment, RequestID: x.RequestID, CreatedAt: x.CreatedAt.Time()})
	}
	for _, x := range d.Revisions {
		c.Revisions = append(c.Revisions, domain.Revision{Number: x.Number, ProposedPatch: x.ProposedPatch, Snapshot: domain.Snapshot{BeforeJSON: x.Snapshot.BeforeJSON, AfterJSON: x.Snapshot.AfterJSON, Digest: x.Snapshot.Digest}, Evidence: domain.Evidence{SourceID: x.Evidence.SourceID, SourceRecordIDs: x.Evidence.SourceRecordIDs, DuplicateCandidateIDs: x.Evidence.DuplicateCandidateIDs, ReconciliationConflictIDs: x.Evidence.ReconciliationConflictIDs, References: x.Evidence.References}, ActorID: x.ActorID, CreatedAt: x.CreatedAt.Time()})
	}
	return c
}

type ChangeRequestRepo struct{ s *Store }

func NewChangeRequestRepo(s *Store) *ChangeRequestRepo { return &ChangeRequestRepo{s: s} }
func (r *ChangeRequestRepo) col() *mongo.Collection    { return r.s.db.Collection(ColChangeRequests) }

func (r *ChangeRequestRepo) Create(ctx context.Context, c domain.ChangeRequest, requestID string, evidence audit.Entry) (domain.ChangeRequest, bool, error) {
	if requestID != "" {
		var existing changeRequestDoc
		err := r.col().FindOne(ctx, bson.M{"requestId": requestID}).Decode(&existing)
		if err == nil {
			return existing.domain(), false, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return domain.ChangeRequest{}, false, err
		}
	}
	doc := changeRequestToDoc(c, requestID)
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	session, err := r.s.client.StartSession()
	if err != nil {
		return domain.ChangeRequest{}, false, err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		if _, e := r.col().InsertOne(tx, doc); e != nil {
			return nil, e
		}
		if _, e := NewAuditRepo(r.s).append(tx, evidence); e != nil {
			return nil, e
		}
		return nil, nil
	})
	if err != nil && mongo.IsDuplicateKeyError(err) && requestID != "" {
		var existing changeRequestDoc
		if e := r.col().FindOne(ctx, bson.M{"requestId": requestID}).Decode(&existing); e == nil {
			return existing.domain(), false, nil
		}
	}
	if err != nil {
		return domain.ChangeRequest{}, false, fmt.Errorf("create change request: %w", err)
	}
	return c, true, nil
}
func (r *ChangeRequestRepo) Get(ctx context.Context, id string) (domain.ChangeRequest, error) {
	var d changeRequestDoc
	err := r.col().FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ChangeRequest{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ChangeRequest{}, err
	}
	return d.domain(), nil
}

func (r *ChangeRequestRepo) List(ctx context.Context, f app.Filter) (app.Page, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	q := bson.M{}
	after, err := decodeCursor(f.Cursor)
	if err != nil {
		return app.Page{}, err
	}
	if after != "" {
		q["_id"] = bson.M{"$gt": after}
	}
	if f.State != "" {
		q["state"] = string(f.State)
	}
	if f.TargetKind != "" {
		q["target.kind"] = f.TargetKind
	}
	if f.TargetID != "" {
		q["target.id"] = f.TargetID
	}
	if f.SubmitterID != "" {
		q["submitterId"] = f.SubmitterID
	}
	if f.ReviewerID != "" {
		q["reviewerId"] = f.ReviewerID
	}
	cur, err := r.col().Find(ctx, q, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return app.Page{}, err
	}
	defer cur.Close(ctx)
	var docs []changeRequestDoc
	if err := cur.All(ctx, &docs); err != nil {
		return app.Page{}, err
	}
	page := app.Page{Data: make([]domain.ChangeRequest, 0, limit)}
	for i, d := range docs {
		if i == limit {
			break
		}
		page.Data = append(page.Data, d.domain())
	}
	if len(docs) > limit {
		page.NextCursor = encodeCursor(docs[limit-1].ID)
	}
	return page, nil
}

func (r *ChangeRequestRepo) Transition(ctx context.Context, next domain.ChangeRequest, expected int64, e audit.Entry) error {
	h := changeRequestToDoc(next, "").History[len(next.History)-1]
	set := bson.M{"state": string(next.State), "reviewerId": next.ReviewerID, "version": next.Version, "updatedAt": bson.NewDateTimeFromTime(next.UpdatedAt)}
	return r.atomicAppend(ctx, next.ID, expected, bson.M{"$set": set, "$push": bson.M{"history": h}}, e)
}
func (r *ChangeRequestRepo) Revise(ctx context.Context, next domain.ChangeRequest, expected int64, e audit.Entry) error {
	d := changeRequestToDoc(next, "")
	h := d.History[len(d.History)-1]
	revision := d.Revisions[len(d.Revisions)-1]
	set := bson.M{"proposedPatch": d.ProposedPatch, "snapshot": d.Snapshot, "evidence": d.Evidence, "state": string(next.State), "reviewerId": "", "version": next.Version, "updatedAt": d.UpdatedAt}
	return r.atomicAppend(ctx, next.ID, expected, bson.M{"$set": set, "$push": bson.M{"history": h, "revisions": revision}}, e)
}
func (r *ChangeRequestRepo) AddComment(ctx context.Context, next domain.ChangeRequest, expected int64, e audit.Entry) error {
	c := changeRequestToDoc(next, "").Comments[len(next.Comments)-1]
	return r.atomicAppend(ctx, next.ID, expected, bson.M{"$set": bson.M{"version": next.Version, "updatedAt": bson.NewDateTimeFromTime(next.UpdatedAt)}, "$push": bson.M{"comments": c}}, e)
}
func (r *ChangeRequestRepo) atomicAppend(ctx context.Context, id string, expected int64, update bson.M, e audit.Entry) error {
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	session, err := r.s.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		res, x := r.col().UpdateOne(tx, bson.M{"_id": id, "version": expected}, update)
		if x != nil {
			return nil, x
		}
		if res.MatchedCount == 0 {
			return nil, domain.ErrConflict
		}
		if _, x = NewAuditRepo(r.s).append(tx, e); x != nil {
			return nil, x
		}
		return nil, nil
	})
	return err
}
