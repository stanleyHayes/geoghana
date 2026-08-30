package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	ingestdomain "github.com/ghanageo/ghanageo/services/api/internal/domain/ingest"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type importRunDoc struct {
	ID                      string              `bson:"_id"`
	SourceID                string              `bson:"sourceId"`
	Status                  ingestdomain.Status `bson:"status"`
	PayloadHash             string              `bson:"payloadHash,omitempty"`
	RequestID               string              `bson:"requestId,omitempty"`
	QueuedAt                time.Time           `bson:"queuedAt"`
	StartedAt               *time.Time          `bson:"startedAt,omitempty"`
	FinishedAt              *time.Time          `bson:"finishedAt,omitempty"`
	DurationMS              int64               `bson:"durationMs"`
	RecordsProcessed        int64               `bson:"recordsProcessed"`
	Errors                  []importRunErrorDoc `bson:"errors"`
	ReconciliationConflicts int64               `bson:"reconciliationConflicts"`
	DuplicateCandidates     int64               `bson:"duplicateCandidates"`
	ExpiresAt               time.Time           `bson:"expiresAt"`
}

type importRunErrorDoc struct {
	Code      string    `bson:"code"`
	Message   string    `bson:"message"`
	RecordRef string    `bson:"recordRef,omitempty"`
	At        time.Time `bson:"at"`
}

func toImportRunErrorDoc(e ingestdomain.Error) importRunErrorDoc {
	return importRunErrorDoc{Code: e.Code, Message: e.Message, RecordRef: e.RecordRef, At: e.At}
}

func (d importRunDoc) domain() ingestdomain.Run {
	errorsOut := make([]ingestdomain.Error, 0, len(d.Errors))
	for _, e := range d.Errors {
		errorsOut = append(errorsOut, ingestdomain.Error{Code: e.Code, Message: e.Message, RecordRef: e.RecordRef, At: e.At})
	}
	return ingestdomain.Run{ID: d.ID, SourceID: d.SourceID, Status: d.Status,
		PayloadHash: d.PayloadHash, RequestID: d.RequestID, QueuedAt: d.QueuedAt,
		StartedAt: d.StartedAt, FinishedAt: d.FinishedAt, DurationMS: d.DurationMS,
		RecordsProcessed: d.RecordsProcessed, Errors: errorsOut,
		ReconciliationConflicts: d.ReconciliationConflicts,
		DuplicateCandidates:     d.DuplicateCandidates, ExpiresAt: d.ExpiresAt}
}

type ImportRunRepo struct{ s *Store }

func NewImportRunRepo(s *Store) *ImportRunRepo { return &ImportRunRepo{s: s} }

func (r *ImportRunRepo) Queue(ctx context.Context, run ingestdomain.Run) (ingestdomain.Run, bool, error) {
	doc := importRunDoc{ID: run.ID, SourceID: run.SourceID, Status: run.Status,
		PayloadHash: run.PayloadHash, RequestID: run.RequestID, QueuedAt: run.QueuedAt,
		DurationMS: 0, RecordsProcessed: 0, Errors: []importRunErrorDoc{},
		ReconciliationConflicts: 0, DuplicateCandidates: 0, ExpiresAt: run.ExpiresAt}
	res, err := r.s.db.Collection(ColSourceRuns).UpdateOne(ctx, bson.M{"_id": run.ID},
		bson.M{"$setOnInsert": doc}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return ingestdomain.Run{}, false, fmt.Errorf("queue import run: %w", err)
	}
	if res.UpsertedCount > 0 {
		return run, true, nil
	}
	var existing importRunDoc
	if err := r.s.db.Collection(ColSourceRuns).FindOne(ctx, bson.M{"_id": run.ID}).Decode(&existing); err != nil {
		return ingestdomain.Run{}, false, fmt.Errorf("read existing import run: %w", err)
	}
	return existing.domain(), false, nil
}

func (r *ImportRunRepo) Start(ctx context.Context, id string, at time.Time) error {
	res, err := r.s.db.Collection(ColSourceRuns).UpdateOne(ctx,
		bson.M{"_id": id, "status": ingestdomain.StatusQueued},
		bson.M{"$set": bson.M{"status": ingestdomain.StatusRunning, "startedAt": at.UTC()}})
	if err != nil {
		return fmt.Errorf("start import run: %w", err)
	}
	if res.MatchedCount == 0 {
		var d importRunDoc
		if err := r.s.db.Collection(ColSourceRuns).FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return ErrNotFound
			}
			return err
		}
		if d.Status == ingestdomain.StatusRunning || d.Status == ingestdomain.StatusSucceeded {
			return nil
		}
		return fmt.Errorf("cannot start import run in %s state", d.Status)
	}
	return nil
}

func (r *ImportRunRepo) CommitRecord(ctx context.Context, record ingestdomain.RawRecord, place *geography.Place) (bool, error) {
	session, err := r.s.client.StartSession()
	if err != nil {
		return false, fmt.Errorf("start import record transaction: %w", err)
	}
	defer session.EndSession(ctx)
	var created bool
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		err := r.s.db.Collection(ColSourceRecords).FindOne(tx, bson.M{"_id": record.ID}).Err()
		if err == nil {
			return nil, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("check source record reference: %w", err)
		}
		if place != nil {
			created, err = NewPlaceRepo(r.s).Upsert(tx, *place)
			if err != nil {
				return nil, fmt.Errorf("persist imported place: %w", err)
			}
			if created {
				record.Outcome = ingestdomain.RecordCreated
			} else {
				record.Outcome = ingestdomain.RecordUpdated
			}
		}
		insert := bson.M{"_id": record.ID, "runId": record.RunID, "sourceId": record.SourceID,
			"externalRef": record.ExternalRef, "payloadHash": record.PayloadHash,
			"outcome": record.Outcome, "reasonCode": record.ReasonCode,
			"processedAt": record.ProcessedAt, "expiresAt": record.ExpiresAt}
		_, err = r.s.db.Collection(ColSourceRecords).InsertOne(tx, insert)
		if err != nil {
			return nil, fmt.Errorf("persist source record reference: %w", err)
		}
		update := bson.M{"$inc": bson.M{"recordsProcessed": 1}}
		if record.Outcome == ingestdomain.RecordRejected {
			update["$push"] = bson.M{"errors": bson.M{"$each": bson.A{importRunErrorDoc{
				Code: record.ReasonCode, Message: record.ReasonCode, RecordRef: record.ExternalRef, At: record.ProcessedAt}}, "$slice": -ingestdomain.MaxRecordedErrors}}
		}
		runRes, err := r.s.db.Collection(ColSourceRuns).UpdateOne(tx,
			bson.M{"_id": record.RunID, "status": ingestdomain.StatusRunning}, update)
		if err != nil {
			return nil, fmt.Errorf("advance import run: %w", err)
		}
		if runRes.MatchedCount != 1 {
			return nil, errors.New("import run is not running")
		}
		return nil, nil
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

func (r *ImportRunRepo) Complete(ctx context.Context, id string, at time.Time, conflicts, duplicates int64) error {
	var d importRunDoc
	if err := r.s.db.Collection(ColSourceRuns).FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		return err
	}
	if d.Status == ingestdomain.StatusSucceeded {
		return nil
	}
	if d.Status != ingestdomain.StatusRunning || d.StartedAt == nil {
		return fmt.Errorf("cannot complete import run in %s state", d.Status)
	}
	duration := at.UTC().Sub(d.StartedAt.UTC()).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	res, err := r.s.db.Collection(ColSourceRuns).UpdateOne(ctx,
		bson.M{"_id": id, "status": ingestdomain.StatusRunning}, bson.M{"$set": bson.M{
			"status": ingestdomain.StatusSucceeded, "finishedAt": at.UTC(), "durationMs": duration,
			"reconciliationConflicts": conflicts, "duplicateCandidates": duplicates}})
	if err != nil {
		return err
	}
	if res.MatchedCount != 1 {
		return errors.New("import run completion lost a concurrent transition")
	}
	return nil
}

func (r *ImportRunRepo) Fail(ctx context.Context, id string, failure ingestdomain.Error, at time.Time) error {
	var d importRunDoc
	if err := r.s.db.Collection(ColSourceRuns).FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		return err
	}
	duration := int64(0)
	if d.StartedAt != nil {
		duration = at.UTC().Sub(d.StartedAt.UTC()).Milliseconds()
		if duration < 0 {
			duration = 0
		}
	}
	res, err := r.s.db.Collection(ColSourceRuns).UpdateOne(ctx,
		bson.M{"_id": id, "status": bson.M{"$in": bson.A{ingestdomain.StatusQueued, ingestdomain.StatusRunning}}},
		bson.M{"$set": bson.M{"status": ingestdomain.StatusFailed, "finishedAt": at.UTC(), "durationMs": duration},
			"$push": bson.M{"errors": bson.M{"$each": bson.A{toImportRunErrorDoc(failure)}, "$slice": -ingestdomain.MaxRecordedErrors}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 && d.Status != ingestdomain.StatusFailed {
		return errors.New("import run failure lost a concurrent transition")
	}
	return nil
}
