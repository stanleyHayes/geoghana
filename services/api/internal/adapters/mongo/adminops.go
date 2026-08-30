package mongo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	app "github.com/ghanageo/ghanageo/services/api/internal/app/adminops"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/adminops"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type AdminOpsRepo struct {
	s           *Store
	cacheProbe  domain.DependencyProbe
	searchProbe domain.DependencyProbe
	metrics     domain.RuntimeMetricsReader
}

func NewAdminOpsRepo(s *Store) *AdminOpsRepo { return &AdminOpsRepo{s: s} }

func (r *AdminOpsRepo) WithHealthDependencies(cache, search domain.DependencyProbe, metrics domain.RuntimeMetricsReader) *AdminOpsRepo {
	r.cacheProbe, r.searchProbe, r.metrics = cache, search, metrics
	return r
}

func adminLimit(n int) int {
	if n <= 0 {
		return 25
	}
	if n > 100 {
		return 100
	}
	return n
}
func adminCursor(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", fmt.Errorf("invalid cursor: %w", err)
	}
	return string(b), nil
}
func adminEncode(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func (r *AdminOpsRepo) Dashboard(ctx context.Context) (domain.Dashboard, error) {
	collections := []string{ColRegions, ColDistricts, ColPlaces, ColRoads, ColPOIs, ColAuditLog}
	values := make([]int64, len(collections))
	for i, name := range collections {
		n, err := r.s.db.Collection(name).CountDocuments(ctx, bson.M{})
		if err != nil {
			return domain.Dashboard{}, err
		}
		values[i] = n
	}
	pending, err := r.s.db.Collection(ColOutbox).CountDocuments(ctx, bson.M{"status": "pending"})
	if err != nil {
		return domain.Dashboard{}, err
	}
	dead, err := r.s.db.Collection(ColOutbox).CountDocuments(ctx, bson.M{"status": "dead"})
	if err != nil {
		return domain.Dashboard{}, err
	}
	recentPage, err := r.Audit(ctx, app.AuditQuery{PageQuery: app.PageQuery{Limit: 8}})
	if err != nil {
		return domain.Dashboard{}, err
	}
	var current *dataset.Version
	var live datasetDoc
	err = r.s.db.Collection(ColDatasetVersion).FindOne(ctx, bson.M{"status": string(dataset.StatusPublished)}, options.FindOne().SetSort(bson.D{{Key: "publishedAt", Value: -1}, {Key: "_id", Value: -1}})).Decode(&live)
	if err == nil {
		v := live.toDomain()
		current = &v
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Dashboard{}, err
	}
	return domain.Dashboard{Counts: domain.Counts{Regions: values[0], Districts: values[1], Places: values[2], Roads: values[3], POIs: values[4], AuditEntries: values[5], PendingOutbox: pending, DeadOutbox: dead}, CurrentRelease: current, RecentActivity: recentPage.Data}, nil
}

func (r *AdminOpsRepo) Health(ctx context.Context) (domain.Health, error) {
	checkedAt := time.Now().UTC()
	health := domain.Health{Status: "healthy", Queue: map[string]int64{}, IndexStatus: "unavailable", Metrics: map[string]domain.Metric{}}
	type namedProbe struct {
		name   string
		result domain.ProbeResult
	}
	probeCh := make(chan namedProbe, 2)
	for name, probe := range map[string]domain.DependencyProbe{"cache": r.cacheProbe, "search-index": r.searchProbe} {
		if probe == nil {
			probeCh <- namedProbe{name, domain.ProbeResult{Status: "unavailable", Detail: "probe is not configured"}}
			continue
		}
		go func(name string, probe domain.DependencyProbe) {
			probeCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
			defer cancel()
			resultCh := make(chan domain.ProbeResult, 1)
			go func() { resultCh <- probe.Probe(probeCtx) }()
			select {
			case result := <-resultCh:
				probeCh <- namedProbe{name, result}
			case <-probeCtx.Done():
				probeCh <- namedProbe{name, domain.ProbeResult{Status: "unavailable", Detail: "probe timed out", LatencyMS: 2500}}
			}
		}(name, probe)
	}

	dbCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	start := time.Now()
	err := r.s.client.Ping(dbCtx, nil)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		health.Dependencies = append(health.Dependencies, domain.Dependency{Name: "mongodb", Status: "unavailable", Detail: "database probe failed", LatencyMS: latency, CheckedAt: checkedAt})
		health.Metrics["databaseLatency"] = domain.Metric{Unit: "ms", Status: "unavailable", Threshold: "degraded >=100ms; unhealthy >=500ms"}
		health.Status = "unhealthy"
	} else {
		health.Dependencies = append(health.Dependencies, domain.Dependency{Name: "mongodb", Status: thresholdStatus(float64(latency), 100, 500), Detail: "read-only ping succeeded", LatencyMS: latency, CheckedAt: checkedAt})
		health.Metrics["databaseLatency"] = domain.Metric{Value: float64(latency), Unit: "ms", Status: thresholdStatus(float64(latency), 100, 500), Threshold: "degraded >=100ms; unhealthy >=500ms"}
	}
	if err == nil {
		for _, status := range []string{"pending", "processing", "completed", "dead"} {
			n, e := r.s.db.Collection(ColOutbox).CountDocuments(dbCtx, bson.M{"status": status})
			if e != nil {
				health.Status = "degraded"
				break
			}
			health.Queue[status] = n
		}
	}
	var latest auditDoc
	e := r.s.db.Collection(ColAuditLog).FindOne(dbCtx, bson.M{"action": string(audit.ActionSourceImported)}, options.FindOne().SetSort(bson.D{{Key: "at", Value: -1}})).Decode(&latest)
	if e == nil {
		t := latest.At.UTC()
		health.ETLFreshness = &t
		hours := time.Since(t).Hours()
		health.Metrics["etlFreshness"] = domain.Metric{Value: hours, Unit: "hours", Status: thresholdStatus(hours, 24, 72), Threshold: "degraded >=24h; unhealthy >=72h"}
	} else if !errors.Is(e, mongo.ErrNoDocuments) {
		health.Status = "degraded"
	}
	if _, exists := health.Metrics["etlFreshness"]; !exists {
		health.Metrics["etlFreshness"] = domain.Metric{Unit: "hours", Status: "unavailable", Threshold: "degraded >=24h; unhealthy >=72h", Detail: "no completed import is recorded"}
	}
	depth := health.Queue["pending"] + health.Queue["processing"]
	queueStatus := thresholdStatus(float64(depth), 100, 1000)
	if health.Queue["dead"] > 0 && queueStatus == "healthy" {
		queueStatus = "degraded"
	}
	health.Metrics["queueDepth"] = domain.Metric{Value: float64(depth), Unit: "events", Status: queueStatus, Threshold: "degraded >=100 or dead >0; unhealthy >=1000"}

	var searchResult domain.ProbeResult
	for range 2 {
		probed := <-probeCh
		health.Dependencies = append(health.Dependencies, domain.Dependency{Name: probed.name, Status: probed.result.Status, Detail: probed.result.Detail, LatencyMS: probed.result.LatencyMS, CheckedAt: checkedAt})
		if probed.name == "cache" && probed.result.HitRate != nil {
			health.Metrics["cacheHitRate"] = rateMetric(*probed.result.HitRate)
		}
		if probed.name == "search-index" {
			searchResult = probed.result
			health.IndexStatus = probed.result.Status
		}
	}
	if searchResult.DocumentCount != nil {
		places, countErr := r.s.db.Collection(ColPlaces).CountDocuments(dbCtx, bson.M{})
		if countErr == nil {
			lag := float64(0)
			indexStatus := "healthy"
			if *searchResult.DocumentCount != places {
				indexStatus = "degraded"
				var live datasetDoc
				if releaseErr := r.s.db.Collection(ColDatasetVersion).FindOne(dbCtx, bson.M{"status": string(dataset.StatusPublished)}, options.FindOne().SetSort(bson.D{{Key: "publishedAt", Value: -1}})).Decode(&live); releaseErr == nil {
					if publishedAt, parseErr := time.Parse(time.RFC3339, live.PublishedAt); parseErr == nil {
						lag = max(0, time.Since(publishedAt).Seconds())
					}
				}
			}
			health.IndexStatus = indexStatus
			health.Metrics["searchIndexLag"] = domain.Metric{Value: lag, Unit: "seconds", Status: indexStatus, Threshold: "healthy when indexed and canonical document counts match", Detail: fmt.Sprintf("%d indexed / %d canonical", *searchResult.DocumentCount, places)}
		}
	}
	if _, exists := health.Metrics["searchIndexLag"]; !exists {
		health.Metrics["searchIndexLag"] = domain.Metric{Unit: "seconds", Status: "unavailable", Threshold: "healthy when indexed and canonical document counts match"}
	}
	if r.metrics != nil {
		m := r.metrics.AdminMetrics()
		rate := float64(0)
		if m.RequestCount > 0 {
			rate = float64(m.ErrorCount) / float64(m.RequestCount) * 100
		}
		health.Metrics["errorRate"] = domain.Metric{Value: rate, Unit: "percent", Status: thresholdStatus(rate, 1, 5), Threshold: "degraded >=1%; unhealthy >=5%", Detail: "process lifetime aggregate"}
		if _, exists := health.Metrics["cacheHitRate"]; !exists && m.CacheHits+m.CacheMisses > 0 {
			health.Metrics["cacheHitRate"] = rateMetric(float64(m.CacheHits) / float64(m.CacheHits+m.CacheMisses) * 100)
		}
	}
	if _, exists := health.Metrics["errorRate"]; !exists {
		health.Metrics["errorRate"] = domain.Metric{Unit: "percent", Status: "unavailable", Threshold: "degraded >=1%; unhealthy >=5%"}
	}
	if _, exists := health.Metrics["cacheHitRate"]; !exists {
		health.Metrics["cacheHitRate"] = domain.Metric{Unit: "percent", Status: "unavailable", Threshold: "degraded <80%; unhealthy <60%", Detail: "no cache lookups are recorded"}
	}
	for _, metric := range health.Metrics {
		health.Status = worseStatus(health.Status, metric.Status)
	}
	for _, dep := range health.Dependencies {
		health.Status = worseStatus(health.Status, dep.Status)
	}
	return health, nil
}

func thresholdStatus(value, degraded, unhealthy float64) string {
	if value >= unhealthy {
		return "unhealthy"
	}
	if value >= degraded {
		return "degraded"
	}
	return "healthy"
}
func rateMetric(value float64) domain.Metric {
	status := "healthy"
	if value < 60 {
		status = "unhealthy"
	} else if value < 80 {
		status = "degraded"
	}
	return domain.Metric{Value: value, Unit: "percent", Status: status, Threshold: "degraded <80%; unhealthy <60%"}
}
func worseStatus(current, candidate string) string {
	rank := map[string]int{"healthy": 0, "degraded": 1, "unavailable": 2, "unhealthy": 3}
	if rank[candidate] > rank[current] {
		return candidate
	}
	return current
}

func (r *AdminOpsRepo) Audit(ctx context.Context, q app.AuditQuery) (domain.Page[audit.Entry], error) {
	after, err := adminCursor(q.Cursor)
	if err != nil {
		return domain.Page[audit.Entry]{}, err
	}
	f := bson.M{}
	if after != "" {
		f["_id"] = bson.M{"$lt": after}
	}
	if q.Actor != "" {
		f["actorId"] = q.Actor
	}
	if q.Action != "" {
		f["action"] = q.Action
	}
	if q.Target != "" {
		f["targetId"] = q.Target
	}
	if q.Outcome != "" {
		f["outcome"] = q.Outcome
	}
	limit := adminLimit(q.Limit)
	cur, err := r.s.db.Collection(ColAuditLog).Find(ctx, f, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return domain.Page[audit.Entry]{}, err
	}
	defer cur.Close(ctx)
	var docs []auditDoc
	if err := cur.All(ctx, &docs); err != nil {
		return domain.Page[audit.Entry]{}, err
	}
	out := domain.Page[audit.Entry]{Data: make([]audit.Entry, 0, limit)}
	for i, d := range docs {
		if i == limit {
			break
		}
		out.Data = append(out.Data, d.toDomain())
	}
	if len(docs) > limit {
		out.NextCursor = adminEncode(docs[limit-1].ID)
	}
	return out, nil
}

func (r *AdminOpsRepo) SourceRuns(ctx context.Context, q app.PageQuery) (domain.Page[domain.SourceRun], error) {
	if page, found, err := r.durableSourceRuns(ctx, q); err != nil || found {
		return page, err
	}
	// Compatibility for deployments that have not run the source-runs
	// migration yet: historical audit rows remain visible.
	p, err := r.Audit(ctx, app.AuditQuery{PageQuery: q, Action: string(audit.ActionSourceImported)})
	if err != nil {
		return domain.Page[domain.SourceRun]{}, err
	}
	out := domain.Page[domain.SourceRun]{NextCursor: p.NextCursor, Data: make([]domain.SourceRun, 0, len(p.Data))}
	for _, e := range p.Data {
		run := domain.SourceRun{ID: e.ID, SourceID: e.Target.ID, Status: string(e.Outcome), RequestID: e.RequestID, Error: e.Error, StartedAt: e.At, DetailAvailability: domain.DetailHistoricalOnly}
		payload := e.After
		if payload == nil {
			payload = e.Before
		}
		if v, ok := payload["payloadHash"].(string); ok {
			run.PayloadHash = v
		}
		run.RecordsProcessed = number(payload["recordsProcessed"])
		run.Conflicts = number(payload["conflicts"])
		run.DuplicateCandidates = number(payload["duplicateCandidates"])
		out.Data = append(out.Data, run)
	}
	return out, nil
}

func (r *AdminOpsRepo) SourceRun(ctx context.Context, id string) (domain.SourceRun, error) {
	var durable importRunDoc
	if err := r.s.db.Collection(ColSourceRuns).FindOne(ctx, bson.M{"_id": id}).Decode(&durable); err == nil {
		return sourceRunProjection(durable), nil
	} else if !errors.Is(err, mongo.ErrNoDocuments) && !isNamespaceMissing(err) {
		return domain.SourceRun{}, err
	}
	var d auditDoc
	err := r.s.db.Collection(ColAuditLog).FindOne(ctx, bson.M{"_id": id, "action": string(audit.ActionSourceImported)}).Decode(&d)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return domain.SourceRun{}, domain.ErrNotFound
		}
		return domain.SourceRun{}, err
	}
	e := d.toDomain()
	payload := e.After
	if payload == nil {
		payload = e.Before
	}
	run := domain.SourceRun{ID: e.ID, SourceID: e.Target.ID, Status: string(e.Outcome), RequestID: e.RequestID, Error: e.Error, StartedAt: e.At, DetailAvailability: domain.DetailHistoricalOnly}
	if v, ok := payload["payloadHash"].(string); ok {
		run.PayloadHash = v
	}
	run.RecordsProcessed = number(payload["recordsProcessed"])
	run.Conflicts = number(payload["conflicts"])
	run.DuplicateCandidates = number(payload["duplicateCandidates"])
	return run, nil
}

func (r *AdminOpsRepo) durableSourceRuns(ctx context.Context, q app.PageQuery) (domain.Page[domain.SourceRun], bool, error) {
	after, err := adminCursor(q.Cursor)
	if err != nil {
		return domain.Page[domain.SourceRun]{}, false, err
	}
	f := bson.M{}
	if after != "" {
		parts := strings.SplitN(after, "\x1f", 2)
		if len(parts) == 2 {
			at, parseErr := time.Parse(time.RFC3339Nano, parts[0])
			if parseErr != nil {
				return domain.Page[domain.SourceRun]{}, false, fmt.Errorf("invalid source-run cursor: %w", parseErr)
			}
			f["$or"] = bson.A{bson.M{"queuedAt": bson.M{"$lt": at}}, bson.M{"queuedAt": at, "_id": bson.M{"$lt": parts[1]}}}
		} else {
			f["_id"] = bson.M{"$lt": after}
		}
	}
	limit := adminLimit(q.Limit)
	cur, err := r.s.db.Collection(ColSourceRuns).Find(ctx, f,
		options.Find().SetSort(bson.D{{Key: "queuedAt", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit+1)))
	if err != nil {
		if isNamespaceMissing(err) {
			return domain.Page[domain.SourceRun]{}, false, nil
		}
		return domain.Page[domain.SourceRun]{}, false, err
	}
	defer cur.Close(ctx)
	var docs []importRunDoc
	if err := cur.All(ctx, &docs); err != nil {
		return domain.Page[domain.SourceRun]{}, false, err
	}
	if len(docs) == 0 {
		return domain.Page[domain.SourceRun]{}, false, nil
	}
	out := domain.Page[domain.SourceRun]{Data: make([]domain.SourceRun, 0, limit)}
	for i, d := range docs {
		if i == limit {
			break
		}
		out.Data = append(out.Data, sourceRunProjection(d))
	}
	if len(docs) > limit {
		last := docs[limit-1]
		out.NextCursor = adminEncode(last.QueuedAt.UTC().Format(time.RFC3339Nano) + "\x1f" + last.ID)
	}
	return out, true, nil
}

func sourceRunProjection(d importRunDoc) domain.SourceRun {
	started := d.QueuedAt
	if d.StartedAt != nil {
		started = *d.StartedAt
	}
	errorsOut := make([]string, 0, len(d.Errors))
	for _, e := range d.Errors {
		errorsOut = append(errorsOut, e.Code)
	}
	errorSummary := ""
	if len(errorsOut) > 0 {
		errorSummary = errorsOut[0]
	}
	return domain.SourceRun{ID: d.ID, SourceID: d.SourceID, Status: string(d.Status), Error: errorSummary,
		PayloadHash: d.PayloadHash, RequestID: d.RequestID, StartedAt: started,
		QueuedAt: d.QueuedAt, FinishedAt: d.FinishedAt, DurationMS: d.DurationMS,
		RecordsProcessed: d.RecordsProcessed, Errors: errorsOut,
		Conflicts: d.ReconciliationConflicts, DuplicateCandidates: d.DuplicateCandidates,
		DetailAvailability: domain.DetailDurable}
}

type sourceRecordDoc struct {
	ID          string    `bson:"_id"`
	RunID       string    `bson:"runId"`
	SourceID    string    `bson:"sourceId"`
	ExternalRef string    `bson:"externalRef"`
	PayloadHash string    `bson:"payloadHash"`
	Outcome     string    `bson:"outcome"`
	ReasonCode  string    `bson:"reasonCode"`
	ProcessedAt time.Time `bson:"processedAt"`
}

func (d sourceRecordDoc) projection() domain.SourceRecord {
	return domain.SourceRecord{ID: d.ID, RunID: d.RunID, SourceID: d.SourceID,
		ExternalRef: d.ExternalRef, PayloadHash: d.PayloadHash, Outcome: d.Outcome,
		ReasonCode: d.ReasonCode, ProcessedAt: d.ProcessedAt}
}

func (r *AdminOpsRepo) SourceRecords(ctx context.Context, runID string, q app.SourceRecordQuery) (domain.SourceRecordPage, error) {
	var run importRunDoc
	err := r.s.db.Collection(ColSourceRuns).FindOne(ctx, bson.M{"_id": runID}).Decode(&run)
	if errors.Is(err, mongo.ErrNoDocuments) || isNamespaceMissing(err) {
		var historical auditDoc
		historyErr := r.s.db.Collection(ColAuditLog).FindOne(ctx, bson.M{"_id": runID, "action": string(audit.ActionSourceImported)}).Decode(&historical)
		if errors.Is(historyErr, mongo.ErrNoDocuments) {
			return domain.SourceRecordPage{}, domain.ErrNotFound
		}
		if historyErr != nil {
			return domain.SourceRecordPage{}, historyErr
		}
		return domain.SourceRecordPage{Data: []domain.SourceRecord{}, DetailAvailability: domain.DetailHistoricalOnly}, nil
	}
	if err != nil {
		return domain.SourceRecordPage{}, err
	}
	after, err := adminCursor(q.Cursor)
	if err != nil {
		return domain.SourceRecordPage{}, err
	}
	f := bson.M{"runId": runID}
	if q.ReasonCode != "" {
		f["reasonCode"] = q.ReasonCode
	}
	if after != "" {
		parts := strings.SplitN(after, "\x1f", 2)
		if len(parts) != 2 {
			return domain.SourceRecordPage{}, errors.New("invalid source-record cursor")
		}
		at, parseErr := time.Parse(time.RFC3339Nano, parts[0])
		if parseErr != nil {
			return domain.SourceRecordPage{}, fmt.Errorf("invalid source-record cursor: %w", parseErr)
		}
		f["$or"] = bson.A{bson.M{"processedAt": bson.M{"$lt": at}}, bson.M{"processedAt": at, "_id": bson.M{"$lt": parts[1]}}}
	}
	limit := adminLimit(q.Limit)
	cur, err := r.s.db.Collection(ColSourceRecords).Find(ctx, f, options.Find().SetSort(bson.D{{Key: "processedAt", Value: -1}, {Key: "_id", Value: -1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return domain.SourceRecordPage{}, err
	}
	defer cur.Close(ctx)
	var docs []sourceRecordDoc
	if err := cur.All(ctx, &docs); err != nil {
		return domain.SourceRecordPage{}, err
	}
	out := domain.SourceRecordPage{Data: make([]domain.SourceRecord, 0, limit), DetailAvailability: domain.DetailDurable}
	for i, d := range docs {
		if i == limit {
			break
		}
		out.Data = append(out.Data, d.projection())
	}
	if len(docs) > limit {
		last := docs[limit-1]
		out.NextCursor = adminEncode(last.ProcessedAt.UTC().Format(time.RFC3339Nano) + "\x1f" + last.ID)
	}
	return out, nil
}

func (r *AdminOpsRepo) SourceRecord(ctx context.Context, runID, recordID string) (domain.SourceRecord, error) {
	var d sourceRecordDoc
	err := r.s.db.Collection(ColSourceRecords).FindOne(ctx, bson.M{"_id": recordID, "runId": runID}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) || isNamespaceMissing(err) {
		return domain.SourceRecord{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SourceRecord{}, err
	}
	return d.projection(), nil
}

func isNamespaceMissing(err error) bool {
	var commandErr mongo.CommandError
	return errors.As(err, &commandErr) && commandErr.Code == 26
}
func number(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

func (r *AdminOpsRepo) DatasetReleases(ctx context.Context, q app.PageQuery) (domain.Page[dataset.Version], error) {
	after, err := adminCursor(q.Cursor)
	if err != nil {
		return domain.Page[dataset.Version]{}, err
	}
	f := bson.M{}
	if after != "" {
		f["_id"] = bson.M{"$lt": after}
	}
	limit := adminLimit(q.Limit)
	cur, err := r.s.db.Collection(ColDatasetVersion).Find(ctx, f, options.Find().SetSort(bson.D{{Key: "_id", Value: -1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return domain.Page[dataset.Version]{}, err
	}
	defer cur.Close(ctx)
	var docs []datasetDoc
	if err := cur.All(ctx, &docs); err != nil {
		return domain.Page[dataset.Version]{}, err
	}
	out := domain.Page[dataset.Version]{Data: make([]dataset.Version, 0, limit)}
	for i, d := range docs {
		if i == limit {
			break
		}
		out.Data = append(out.Data, d.toDomain())
	}
	if len(docs) > limit {
		out.NextCursor = adminEncode(docs[limit-1].ID)
	}
	return out, nil
}
