package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
)

type roadDoc struct {
	ID                 string        `bson:"_id"`
	Name               string        `bson:"name"`
	NormalizedName     string        `bson:"normalizedName"`
	Ref                string        `bson:"ref,omitempty"`
	Class              string        `bson:"class"`
	RegionID           string        `bson:"regionId,omitempty"`
	RegionName         string        `bson:"regionName,omitempty"`
	DistrictID         string        `bson:"districtId,omitempty"`
	Geometry           *geoJSON      `bson:"geometry,omitempty"`
	Status             string        `bson:"status"`
	VerificationStatus string        `bson:"verificationStatus"`
	Provenance         provenanceDoc `bson:"provenance"`
	// Stored per record, not looked up at read time: a road outlives the
	// import that created it, and the licence has to travel with the row.
	Attribution    string `bson:"attribution"`
	DatasetVersion string `bson:"datasetVersion,omitempty"`
}

type poiDoc struct {
	ID                 string        `bson:"_id"`
	Name               string        `bson:"name"`
	NormalizedName     string        `bson:"normalizedName"`
	Class              string        `bson:"class"`
	Category           string        `bson:"category,omitempty"`
	RegionID           string        `bson:"regionId,omitempty"`
	RegionName         string        `bson:"regionName,omitempty"`
	DistrictID         string        `bson:"districtId,omitempty"`
	Centroid           *geoJSON      `bson:"centroid,omitempty"`
	Status             string        `bson:"status"`
	VerificationStatus string        `bson:"verificationStatus"`
	Provenance         provenanceDoc `bson:"provenance"`
	Attribution        string        `bson:"attribution"`
	DatasetVersion     string        `bson:"datasetVersion,omitempty"`
}

// RoadRepo stores the road network.
type RoadRepo struct{ s *Store }

func NewRoadRepo(s *Store) *RoadRepo       { return &RoadRepo{s: s} }
func (r *RoadRepo) col() *mongo.Collection { return r.s.db.Collection(ColRoads) }

// UpsertMany writes a batch. Bulk rather than one call per record: an OSM
// import is tens of thousands of rows, and a round trip each would take
// minutes instead of seconds.
func (r *RoadRepo) UpsertMany(ctx context.Context, roads []geography.Road) (int, error) {
	if len(roads) == 0 {
		return 0, nil
	}
	models := make([]mongo.WriteModel, 0, len(roads))
	for _, x := range roads {
		d := roadDoc{
			ID: x.ID, Name: x.Name, NormalizedName: normalize.Name(x.Name),
			Ref: x.Ref, Class: string(x.Class),
			RegionID: x.RegionID, RegionName: x.RegionName, DistrictID: x.DistrictID,
			Geometry: geomOf(x.Geometry), Status: string(x.Status),
			VerificationStatus: string(x.VerificationStatus),
			Provenance:         provOf(x.Provenance),
			Attribution:        x.Attribution, DatasetVersion: x.DatasetVersion,
		}
		models = append(models, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": d.ID}).SetReplacement(d).SetUpsert(true))
	}
	res, err := r.col().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, fmt.Errorf("upsert roads: %w", err)
	}
	return int(res.UpsertedCount + res.ModifiedCount), nil
}

func (r *RoadRepo) Count(ctx context.Context) (int64, error) {
	return r.col().CountDocuments(ctx, bson.M{})
}

// POIRepo stores points of interest.
type POIRepo struct{ s *Store }

func NewPOIRepo(s *Store) *POIRepo        { return &POIRepo{s: s} }
func (r *POIRepo) col() *mongo.Collection { return r.s.db.Collection(ColPOIs) }

func (r *POIRepo) UpsertMany(ctx context.Context, pois []geography.POI) (int, error) {
	if len(pois) == 0 {
		return 0, nil
	}
	models := make([]mongo.WriteModel, 0, len(pois))
	for _, x := range pois {
		d := poiDoc{
			ID: x.ID, Name: x.Name, NormalizedName: normalize.Name(x.Name),
			Class: string(x.Class), Category: x.Category,
			RegionID: x.RegionID, RegionName: x.RegionName, DistrictID: x.DistrictID,
			Centroid: pointOf(x.Centroid), Status: string(x.Status),
			VerificationStatus: string(x.VerificationStatus),
			Provenance:         provOf(x.Provenance),
			Attribution:        x.Attribution, DatasetVersion: x.DatasetVersion,
		}
		models = append(models, mongo.NewReplaceOneModel().
			SetFilter(bson.M{"_id": d.ID}).SetReplacement(d).SetUpsert(true))
	}
	res, err := r.col().BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	if err != nil {
		return 0, fmt.Errorf("upsert pois: %w", err)
	}
	return int(res.UpsertedCount + res.ModifiedCount), nil
}

func (r *POIRepo) Count(ctx context.Context) (int64, error) {
	return r.col().CountDocuments(ctx, bson.M{})
}

// AssignRegions fills in the region and district a POI falls inside, by
// point-in-polygon against stored boundaries.
//
// Containment rather than nearest: a POI 200m from a district line belongs to
// exactly one of them, and "nearest boundary" would put it on the wrong side
// often enough to matter.
func (r *POIRepo) AssignRegions(ctx context.Context, db *mongo.Database) (int, error) {
	assigned := 0
	cur, err := db.Collection(ColDistricts).Find(ctx,
		bson.M{"geometry": bson.M{"$ne": nil}},
		options.Find().SetProjection(bson.M{"_id": 1, "regionId": 1, "regionName": 1}))
	if err != nil {
		return 0, fmt.Errorf("list districts: %w", err)
	}
	defer func() { _ = cur.Close(ctx) }()

	for cur.Next(ctx) {
		var d struct {
			ID         string `bson:"_id"`
			RegionID   string `bson:"regionId"`
			RegionName string `bson:"regionName"`
		}
		if err := cur.Decode(&d); err != nil {
			return assigned, err
		}
		var full struct {
			Geometry any `bson:"geometry"`
		}
		if err := db.Collection(ColDistricts).FindOne(ctx, bson.M{"_id": d.ID},
			options.FindOne().SetProjection(bson.M{"geometry": 1})).Decode(&full); err != nil {
			continue
		}
		res, err := r.col().UpdateMany(ctx,
			bson.M{
				"districtId": bson.M{"$in": bson.A{nil, ""}},
				"centroid":   bson.M{"$geoWithin": bson.M{"$geometry": full.Geometry}},
			},
			bson.M{"$set": bson.M{
				"districtId": d.ID, "regionId": d.RegionID, "regionName": d.RegionName,
				"updatedAt": time.Now().UTC(),
			}})
		if err != nil {
			return assigned, fmt.Errorf("assign pois in %s: %w", d.ID, err)
		}
		assigned += int(res.ModifiedCount)
	}
	return assigned, cur.Err()
}
