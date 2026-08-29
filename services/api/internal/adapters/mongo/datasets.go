package mongo

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
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
