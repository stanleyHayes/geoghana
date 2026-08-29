package perf

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TestHotQueriesUseIndexes is the other half of GEO-12.6: a query can meet its
// p95 on a small dataset while doing a collection scan, and then fall over as
// the data grows. Latency alone would not catch that, so every hot query is
// checked to use an IXSCAN and to examine a number of documents close to what
// it returns.
//
// Skipped unless PERF_MONGO_URI is set.
//
//	PERF_MONGO_URI="mongodb://127.0.0.1:27117/ghanageo?replicaSet=rs0&directConnection=true" \
//	  go test ./perf/ -run TestHotQueriesUseIndexes -v
func TestHotQueriesUseIndexes(t *testing.T) {
	uri := os.Getenv("PERF_MONGO_URI")
	if uri == "" {
		t.Skip("PERF_MONGO_URI not set — skipping the index-plan gate")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = client.Disconnect(ctx) }()
	db := client.Database("ghanageo")

	cases := []struct {
		name string
		col  string
		// filter and sort mirror what the repositories actually issue.
		filter bson.M
		sort   bson.D
		limit  int64
		// examineRatio caps totalDocsExamined / nReturned. 1 would be perfect;
		// a little slack allows for a sort or a limit+1 pagination probe.
		examineRatio float64
	}{
		{
			name: "region by id", col: "regions",
			filter: bson.M{"_id": "gh-region-ashanti"}, examineRatio: 1,
		},
		{
			name: "place by id", col: "places",
			filter: bson.M{"_id": "gh-place-accra"}, examineRatio: 1,
		},
		{
			name: "districts page (cursor sort)", col: "districts",
			filter: bson.M{}, sort: bson.D{{Key: "_id", Value: 1}}, limit: 51,
			examineRatio: 1.2,
		},
		{
			name: "districts by region", col: "districts",
			filter: bson.M{"regionId": "gh-region-ashanti"},
			sort:   bson.D{{Key: "_id", Value: 1}}, limit: 51, examineRatio: 2,
		},
		{
			name: "places by district", col: "places",
			filter: bson.M{"districtId": "gh-district-ashanti-kumasi-metropolitan"},
			sort:   bson.D{{Key: "_id", Value: 1}}, limit: 51, examineRatio: 2,
		},
		{
			name: "places near a point (2dsphere)", col: "places",
			filter: bson.M{"centroid": bson.M{"$nearSphere": bson.M{
				"$geometry":    bson.M{"type": "Point", "coordinates": []float64{-0.187, 5.6037}},
				"$maxDistance": 5000,
			}}},
			limit: 20, examineRatio: 3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cmd := bson.D{
				{Key: "find", Value: c.col},
				{Key: "filter", Value: c.filter},
			}
			if c.sort != nil {
				cmd = append(cmd, bson.E{Key: "sort", Value: c.sort})
			}
			if c.limit > 0 {
				cmd = append(cmd, bson.E{Key: "limit", Value: c.limit})
			}

			// bson.Raw rather than bson.M: the nested executionStats document
			// does not reliably decode to bson.M across driver versions, and a
			// failed type assertion silently looked like "no stats".
			raw, err := db.RunCommand(ctx, bson.D{
				{Key: "explain", Value: cmd},
				{Key: "verbosity", Value: "executionStats"},
			}).Raw()
			if err != nil {
				t.Fatalf("explain: %v", err)
			}

			stats, lookupErr := raw.LookupErr("executionStats")
			if lookupErr != nil {
				t.Fatalf("no executionStats in plan: %v", lookupErr)
			}
			returned := rawInt(stats, "nReturned")
			examined := rawInt(stats, "totalDocsExamined")
			keys := rawInt(stats, "totalKeysExamined")

			stage := rawStages(stats)
			t.Logf("%-32s nReturned=%d examined=%d keys=%d stages=%v",
				c.name, returned, examined, keys, stage)

			// MongoDB 8 reports EXPRESS_IXSCAN for a fast-path id lookup; both
			// it and IXSCAN are index scans. Only COLLSCAN is a failure.
			if contains(stage, "COLLSCAN") {
				t.Errorf("%s does a COLLSCAN — it will degrade as the collection grows "+
					"(GEO-12.6 requires an IXSCAN)", c.name)
			}
			if returned > 0 {
				ratio := float64(examined) / float64(returned)
				if ratio > c.examineRatio {
					t.Errorf("%s examined %d documents to return %d (ratio %.1f > %.1f): "+
						"the index is not selective enough", c.name, examined, returned, ratio, c.examineRatio)
				}
			}
		})
	}
}

// rawStages walks the execution tree and collects stage names.
func rawStages(stats bson.RawValue) []string {
	var out []string
	var walk func(bson.RawValue)
	walk = func(v bson.RawValue) {
		doc, ok := v.DocumentOK()
		if !ok {
			return
		}
		if s, err := doc.LookupErr("stage"); err == nil {
			if name, ok := s.StringValueOK(); ok {
				out = append(out, name)
			}
		}
		if child, err := doc.LookupErr("inputStage"); err == nil {
			walk(child)
		}
		if kids, err := doc.LookupErr("inputStages"); err == nil {
			if arr, ok := kids.ArrayOK(); ok {
				vals, _ := arr.Values()
				for _, k := range vals {
					walk(k)
				}
			}
		}
	}
	if es, err := stats.Document().LookupErr("executionStages"); err == nil {
		walk(es)
	}
	return out
}

// rawInt reads a numeric field regardless of whether it came back as int32,
// int64 or double.
func rawInt(stats bson.RawValue, key string) int {
	v, err := stats.Document().LookupErr(key)
	if err != nil {
		return 0
	}
	if n, ok := v.Int32OK(); ok {
		return int(n)
	}
	if n, ok := v.Int64OK(); ok {
		return int(n)
	}
	if n, ok := v.DoubleOK(); ok {
		return int(n)
	}
	return 0
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
