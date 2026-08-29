package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
)

type usageDocument struct {
	ID             string    `bson:"_id"`
	RequestID      string    `bson:"requestId"`
	OrganizationID string    `bson:"organizationId"`
	ApplicationID  string    `bson:"applicationId"`
	KeyID          string    `bson:"keyId"`
	Protocol       string    `bson:"protocol"`
	Operation      string    `bson:"operation"`
	Status         string    `bson:"status"`
	Success        bool      `bson:"success"`
	LatencyMS      int64     `bson:"latencyMs"`
	QuotaCost      int       `bson:"quotaCost"`
	QuotaLimit     int       `bson:"quotaLimit"`
	QuotaRemaining int       `bson:"quotaRemaining"`
	Geography      string    `bson:"geography"`
	At             time.Time `bson:"at"`
}

type UsageRepo struct{ col *mongo.Collection }

func NewUsageRepo(s *Store) *UsageRepo { return &UsageRepo{col: s.db.Collection(ColUsageEvents)} }

func (r *UsageRepo) Record(ctx context.Context, event usage.Event) error {
	doc := usageDocument{ID: event.ID, RequestID: event.RequestID, OrganizationID: event.OrganizationID, ApplicationID: event.ApplicationID, KeyID: event.KeyID, Protocol: event.Protocol, Operation: event.Operation, Status: event.Status, Success: event.Success, LatencyMS: event.LatencyMS, QuotaCost: event.QuotaCost, QuotaLimit: event.QuotaLimit, QuotaRemaining: event.QuotaRemaining, Geography: event.Geography, At: event.At}
	_, err := r.col.InsertOne(ctx, doc)
	return err
}

type aggregateRow struct {
	Label        string  `bson:"_id"`
	Requests     int64   `bson:"requests"`
	Errors       int64   `bson:"errors"`
	QuotaCost    int64   `bson:"quotaCost"`
	AvgLatencyMS float64 `bson:"avgLatencyMs"`
}

type aggregateResult struct {
	Totals      []aggregateRow `bson:"totals"`
	Protocols   []aggregateRow `bson:"protocols"`
	Endpoints   []aggregateRow `bson:"endpoints"`
	Geographies []aggregateRow `bson:"geographies"`
}

func groupBy(field string, limit int) bson.A {
	stages := bson.A{
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: field},
			{Key: "requests", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "errors", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$cond", Value: bson.A{"$success", 0, 1}}}}}},
			{Key: "quotaCost", Value: bson.D{{Key: "$sum", Value: "$quotaCost"}}},
			{Key: "avgLatencyMs", Value: bson.D{{Key: "$avg", Value: "$latencyMs"}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "requests", Value: -1}, {Key: "_id", Value: 1}}}},
	}
	if limit > 0 {
		stages = append(stages, bson.D{{Key: "$limit", Value: limit}})
	}
	return stages
}

func (r *UsageRepo) Summary(ctx context.Context, organizationID, applicationID string, since time.Time) (usage.Summary, error) {
	match := bson.D{{Key: "organizationId", Value: organizationID}, {Key: "applicationId", Value: applicationID}, {Key: "at", Value: bson.D{{Key: "$gte", Value: since}}}}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$facet", Value: bson.D{
			{Key: "totals", Value: groupBy("all", 1)},
			{Key: "protocols", Value: groupBy("$protocol", 10)},
			{Key: "endpoints", Value: groupBy("$operation", 12)},
			{Key: "geographies", Value: bson.A{bson.D{{Key: "$match", Value: bson.D{{Key: "geography", Value: bson.D{{Key: "$ne", Value: ""}}}}}}, groupBy("$geography", 12)[0], groupBy("$geography", 12)[1], groupBy("$geography", 12)[2]}},
		}}},
	}
	cursor, err := r.col.Aggregate(ctx, pipeline)
	if err != nil {
		return usage.Summary{}, fmt.Errorf("aggregate usage: %w", err)
	}
	defer cursor.Close(ctx)
	var rows []aggregateResult
	if err := cursor.All(ctx, &rows); err != nil {
		return usage.Summary{}, fmt.Errorf("decode usage aggregate: %w", err)
	}
	out := usage.Summary{Since: since}
	if len(rows) == 0 {
		return out, nil
	}
	convert := func(in []aggregateRow) []usage.Breakdown {
		result := make([]usage.Breakdown, 0, len(in))
		for _, row := range in {
			result = append(result, usage.Breakdown{Label: row.Label, Requests: row.Requests, Errors: row.Errors, QuotaCost: row.QuotaCost, AvgLatencyMS: row.AvgLatencyMS})
		}
		return result
	}
	if len(rows[0].Totals) > 0 {
		t := rows[0].Totals[0]
		out.Requests, out.Errors, out.QuotaCost, out.AvgLatencyMS = t.Requests, t.Errors, t.QuotaCost, t.AvgLatencyMS
	}
	out.ByProtocol = convert(rows[0].Protocols)
	out.ByEndpoint = convert(rows[0].Endpoints)
	out.ByGeography = convert(rows[0].Geographies)
	return out, nil
}

func (r *UsageRepo) List(ctx context.Context, organizationID, applicationID string, before *time.Time, limit int) (usage.Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	filter := bson.D{{Key: "organizationId", Value: organizationID}, {Key: "applicationId", Value: applicationID}}
	if before != nil {
		filter = append(filter, bson.E{Key: "at", Value: bson.D{{Key: "$lt", Value: *before}}})
	}
	cursor, err := r.col.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "at", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return usage.Page{}, fmt.Errorf("list usage: %w", err)
	}
	defer cursor.Close(ctx)
	var docs []usageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return usage.Page{}, fmt.Errorf("decode usage: %w", err)
	}
	page := usage.Page{Data: make([]usage.Event, 0, min(limit, len(docs)))}
	for index, doc := range docs {
		if index == limit {
			next := docs[limit-1].At
			page.NextBefore = &next
			break
		}
		page.Data = append(page.Data, usage.Event{ID: doc.ID, RequestID: doc.RequestID, OrganizationID: doc.OrganizationID, ApplicationID: doc.ApplicationID, KeyID: doc.KeyID, Protocol: doc.Protocol, Operation: doc.Operation, Status: doc.Status, Success: doc.Success, LatencyMS: doc.LatencyMS, QuotaCost: doc.QuotaCost, QuotaLimit: doc.QuotaLimit, QuotaRemaining: doc.QuotaRemaining, Geography: doc.Geography, At: doc.At})
	}
	return page, nil
}
