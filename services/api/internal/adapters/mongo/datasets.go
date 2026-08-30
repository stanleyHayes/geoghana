package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/outbox"
)

// datasetDoc is the persistence shape. Domain types carry no bson tags
// (CLAUDE.md rule 3), so the mapping lives here and the storage shape can
// change without touching the domain.
type datasetDoc struct {
	ID          string           `bson:"_id"`
	Version     string           `bson:"version"`
	Status      string           `bson:"status"`
	PublishedAt string           `bson:"publishedAt,omitempty"`
	Changelog   string           `bson:"changelog,omitempty"`
	Checksum    string           `bson:"checksum,omitempty"`
	Counts      map[string]int64 `bson:"counts,omitempty"`
	Artifacts   []artifactDoc    `bson:"artifacts,omitempty"`
}

type artifactDoc struct {
	Entity      string `bson:"entity"`
	Format      string `bson:"format"`
	Filename    string `bson:"filename"`
	SizeBytes   int64  `bson:"sizeBytes"`
	SHA256      string `bson:"sha256"`
	RecordCount int64  `bson:"recordCount"`
}

func (d datasetDoc) toDomain() dataset.Version {
	arts := make([]dataset.Artifact, 0, len(d.Artifacts))
	for _, a := range d.Artifacts {
		arts = append(arts, dataset.Artifact{
			Entity:      a.Entity,
			Format:      dataset.Format(a.Format),
			Filename:    a.Filename,
			SizeBytes:   a.SizeBytes,
			SHA256:      a.SHA256,
			RecordCount: a.RecordCount,
		})
	}
	return dataset.Version{
		Version:     d.Version,
		Status:      dataset.Status(d.Status),
		PublishedAt: d.PublishedAt,
		Changelog:   d.Changelog,
		Counts:      d.Counts,
		Artifacts:   arts,
	}
}

func fromDomain(v dataset.Version) datasetDoc {
	arts := make([]artifactDoc, 0, len(v.Artifacts))
	for _, a := range v.Artifacts {
		arts = append(arts, artifactDoc{
			Entity:      a.Entity,
			Format:      string(a.Format),
			Filename:    a.Filename,
			SizeBytes:   a.SizeBytes,
			SHA256:      a.SHA256,
			RecordCount: a.RecordCount,
		})
	}
	return datasetDoc{
		ID:          v.Version,
		Version:     v.Version,
		Status:      string(v.Status),
		PublishedAt: v.PublishedAt,
		Changelog:   v.Changelog,
		Counts:      v.Counts,
		Artifacts:   arts,
	}
}

// DatasetRepo reads and writes published dataset versions.
type DatasetRepo struct{ s *Store }

func NewDatasetRepo(s *Store) *DatasetRepo { return &DatasetRepo{s: s} }

func (r *DatasetRepo) col() *mongo.Collection {
	return r.s.db.Collection(ColDatasetVersion)
}

// ListPublished returns published versions, newest first.
//
// Only published versions are ever returned: a draft is working state, and
// leaking one would let a consumer pin to a version that may still change or
// be abandoned.
func (r *DatasetRepo) ListPublished(ctx context.Context) ([]dataset.Version, error) {
	cur, err := r.col().Find(ctx,
		bson.M{"status": string(dataset.StatusPublished)},
		options.Find().SetSort(bson.D{{Key: "publishedAt", Value: -1}, {Key: "_id", Value: -1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("list dataset versions: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []dataset.Version
	for cur.Next(ctx) {
		var d datasetDoc
		if err := cur.Decode(&d); err != nil {
			return nil, fmt.Errorf("decode dataset version: %w", err)
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

// ListAll returns every version, newest first, whatever its status. Used by
// the operator history view and by rollback, which needs to see versions that
// are not currently live.
func (r *DatasetRepo) ListAll(ctx context.Context) ([]dataset.Version, error) {
	cur, err := r.col().Find(ctx, bson.M{},
		options.Find().SetSort(bson.D{{Key: "publishedAt", Value: -1}, {Key: "_id", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list dataset versions: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	var out []dataset.Version
	for cur.Next(ctx) {
		var d datasetDoc
		if err := cur.Decode(&d); err != nil {
			return nil, fmt.Errorf("decode dataset version: %w", err)
		}
		out = append(out, d.toDomain())
	}
	return out, cur.Err()
}

// Get returns one version regardless of status; the caller decides whether a
// non-published version may be served.
func (r *DatasetRepo) Get(ctx context.Context, version string) (*dataset.Version, error) {
	var d datasetDoc
	err := r.col().FindOne(ctx, bson.M{"_id": version}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, dataset.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get dataset version %s: %w", version, err)
	}
	v := d.toDomain()
	return &v, nil
}

// Upsert records a version and its artifacts. Idempotent, so a rebuild
// replaces the artifact list rather than appending a second copy.
func (r *DatasetRepo) Upsert(ctx context.Context, v dataset.Version) error {
	for _, a := range v.Artifacts {
		// Enforced on WRITE as well as read: a bad filename must never reach
		// the database, where it would become a traversal at download time.
		if err := a.SafeFilename(); err != nil {
			return err
		}
	}
	doc := fromDomain(v)
	_, err := r.col().ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("upsert dataset version %s: %w", v.Version, err)
	}
	return nil
}

// UpdateAudited compare-and-swaps one unpublished release and appends its
// immutable audit evidence in the same transaction.
func (r *DatasetRepo) UpdateAudited(ctx context.Context, before, after dataset.Version, evidence audit.Entry) error {
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	for _, artifact := range after.Artifacts {
		if err := artifact.SafeFilename(); err != nil {
			return err
		}
	}
	session, err := r.s.client.StartSession()
	if err != nil {
		return fmt.Errorf("start dataset update transaction: %w", err)
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		doc := fromDomain(after)
		filter := bson.M{"_id": before.Version, "status": string(before.Status)}
		if before.Changelog == "" {
			filter["changelog"] = bson.M{"$in": bson.A{"", nil}}
		} else {
			filter["changelog"] = before.Changelog
		}
		result, replaceErr := r.col().ReplaceOne(tx, filter, doc)
		if replaceErr != nil {
			return nil, replaceErr
		}
		if result.MatchedCount != 1 {
			return nil, fmt.Errorf("dataset release compare-and-swap conflict")
		}
		if _, auditErr := NewAuditRepo(r.s).append(tx, evidence); auditErr != nil {
			return nil, auditErr
		}
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("update dataset version %s: %w", after.Version, err)
	}
	return nil
}

// Activate atomically demotes the previous release, promotes the next one and
// records durable work for the background worker. Consumers can therefore
// never observe a published version whose search rebuild event was lost.
func (r *DatasetRepo) Activate(ctx context.Context, previous []dataset.Version, next dataset.Version) error {
	return r.activate(ctx, previous, next, nil)
}

// ActivateAudited commits the catalogue transition, reindex outbox event and
// immutable audit evidence in one transaction. None can exist without all.
func (r *DatasetRepo) ActivateAudited(ctx context.Context, previous []dataset.Version, next dataset.Version, evidence audit.Entry) error {
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	return r.activate(ctx, previous, next, &evidence)
}

func (r *DatasetRepo) activate(ctx context.Context, previous []dataset.Version, next dataset.Version, evidence *audit.Entry) error {
	for _, version := range append(append([]dataset.Version{}, previous...), next) {
		for _, artifact := range version.Artifacts {
			if err := artifact.SafeFilename(); err != nil {
				return err
			}
		}
	}
	session, err := r.s.client.StartSession()
	if err != nil {
		return fmt.Errorf("start dataset activation transaction: %w", err)
	}
	defer session.EndSession(ctx)
	now := time.Now().UTC()
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		for _, version := range previous {
			doc := fromDomain(version)
			if _, replaceErr := r.col().ReplaceOne(tx, bson.M{"_id": doc.ID}, doc); replaceErr != nil {
				return nil, fmt.Errorf("demote dataset version %s: %w", version.Version, replaceErr)
			}
		}
		doc := fromDomain(next)
		if _, replaceErr := r.col().ReplaceOne(tx, bson.M{"_id": doc.ID}, doc); replaceErr != nil {
			return nil, fmt.Errorf("activate dataset version %s: %w", next.Version, replaceErr)
		}
		if enqueueErr := enqueueOutbox(tx, r.s.db, outbox.TopicDatasetPublished, map[string]any{
			"version": next.Version, "publishedAt": next.PublishedAt,
		}, now); enqueueErr != nil {
			return nil, enqueueErr
		}
		if evidence != nil {
			if _, auditErr := NewAuditRepo(r.s).append(tx, *evidence); auditErr != nil {
				return nil, auditErr
			}
		}
		return nil, nil
	})
	if err != nil {
		return fmt.Errorf("activate dataset version %s: %w", next.Version, err)
	}
	return nil
}
