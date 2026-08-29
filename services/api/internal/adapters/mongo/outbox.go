package mongo

import (
	"context"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/outbox"
)

type outboxDoc struct {
	ID          string         `bson:"_id"`
	Topic       string         `bson:"topic"`
	Payload     map[string]any `bson:"payload"`
	Status      string         `bson:"status"`
	Attempts    int            `bson:"attempts"`
	AvailableAt time.Time      `bson:"availableAt"`
	CreatedAt   time.Time      `bson:"createdAt"`
	LockedAt    *time.Time     `bson:"lockedAt,omitempty"`
	LockedBy    string         `bson:"lockedBy,omitempty"`
	LastError   string         `bson:"lastError,omitempty"`
	CompletedAt *time.Time     `bson:"completedAt,omitempty"`
}

func (d outboxDoc) domain() outbox.Event {
	return outbox.Event{
		ID: d.ID, Topic: d.Topic, Payload: d.Payload, Status: outbox.Status(d.Status),
		Attempts: d.Attempts, AvailableAt: d.AvailableAt, CreatedAt: d.CreatedAt,
		LockedAt: d.LockedAt, LockedBy: d.LockedBy, LastError: d.LastError,
		CompletedAt: d.CompletedAt,
	}
}

// OutboxRepo implements an at-least-once queue over the transactional Mongo
// store. Claims are leases: abandoned processing rows become available again.
type OutboxRepo struct{ s *Store }

func NewOutboxRepo(s *Store) *OutboxRepo { return &OutboxRepo{s: s} }

func enqueueOutbox(ctx context.Context, db *mongo.Database, topic string, payload map[string]any, now time.Time) error {
	doc := outboxDoc{
		ID: ulid.Make().String(), Topic: topic, Payload: payload,
		Status: string(outbox.StatusPending), Attempts: 0,
		AvailableAt: now, CreatedAt: now,
	}
	if _, err := db.Collection(ColOutbox).InsertOne(ctx, doc); err != nil {
		return fmt.Errorf("enqueue %s: %w", topic, err)
	}
	return nil
}

func (r *OutboxRepo) Claim(ctx context.Context, workerID string, now time.Time, lease time.Duration) (*outbox.Event, error) {
	staleBefore := now.Add(-lease)
	filter := bson.M{
		"availableAt": bson.M{"$lte": now},
		"$or": bson.A{
			bson.M{"status": string(outbox.StatusPending)},
			bson.M{"status": string(outbox.StatusProcessing), "lockedAt": bson.M{"$lte": staleBefore}},
		},
	}
	update := bson.M{
		"$set": bson.M{"status": string(outbox.StatusProcessing), "lockedAt": now, "lockedBy": workerID},
		"$inc": bson.M{"attempts": 1},
	}
	opts := options.FindOneAndUpdate().
		SetSort(bson.D{{Key: "availableAt", Value: 1}, {Key: "createdAt", Value: 1}}).
		SetReturnDocument(options.After)
	var doc outboxDoc
	err := r.s.db.Collection(ColOutbox).FindOneAndUpdate(ctx, filter, update, opts).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim outbox event: %w", err)
	}
	event := doc.domain()
	return &event, nil
}

func (r *OutboxRepo) Complete(ctx context.Context, eventID, workerID string, now time.Time) error {
	result, err := r.s.db.Collection(ColOutbox).UpdateOne(ctx,
		bson.M{"_id": eventID, "status": string(outbox.StatusProcessing), "lockedBy": workerID},
		bson.M{"$set": bson.M{"status": string(outbox.StatusCompleted), "completedAt": now}, "$unset": bson.M{"lockedAt": "", "lockedBy": "", "lastError": ""}},
	)
	if err != nil {
		return fmt.Errorf("complete outbox event: %w", err)
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("complete outbox event: lease lost")
	}
	return nil
}

func (r *OutboxRepo) Fail(ctx context.Context, event outbox.Event, workerID, message string, now time.Time, maxAttempts int, retryDelay time.Duration) error {
	status := outbox.StatusPending
	if event.Attempts >= maxAttempts {
		status = outbox.StatusDead
	}
	set := bson.M{"status": string(status), "lastError": message, "availableAt": now.Add(retryDelay)}
	result, err := r.s.db.Collection(ColOutbox).UpdateOne(ctx,
		bson.M{"_id": event.ID, "status": string(outbox.StatusProcessing), "lockedBy": workerID},
		bson.M{"$set": set, "$unset": bson.M{"lockedAt": "", "lockedBy": ""}},
	)
	if err != nil {
		return fmt.Errorf("fail outbox event: %w", err)
	}
	if result.MatchedCount != 1 {
		return fmt.Errorf("fail outbox event: lease lost")
	}
	return nil
}

func (r *OutboxRepo) Depth(ctx context.Context) (outbox.Depth, error) {
	counts := outbox.Depth{}
	for state, target := range map[outbox.Status]*int64{
		outbox.StatusPending: &counts.Pending, outbox.StatusProcessing: &counts.Processing, outbox.StatusDead: &counts.Dead,
	} {
		count, err := r.s.db.Collection(ColOutbox).CountDocuments(ctx, bson.M{"status": string(state)})
		if err != nil {
			return outbox.Depth{}, fmt.Errorf("count outbox %s: %w", state, err)
		}
		*target = count
	}
	return counts, nil
}
