package mongo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

// ErrNotFound is returned when an id resolves to nothing.
var ErrNotFound = errors.New("not found")

// Cursor pagination uses the _id as the sort key: stable, unique and
// monotonic for ULIDs, so results are deterministic (plan GEO-7.5).
func encodeCursor(id string) string { return base64.RawURLEncoding.EncodeToString([]byte(id)) }

func decodeCursor(c string) (string, error) {
	if c == "" {
		return "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return "", fmt.Errorf("invalid cursor: %w", err)
	}
	return string(b), nil
}

func cursorFilter(base bson.M, cursor string) (bson.M, error) {
	after, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	if after != "" {
		base["_id"] = bson.M{"$gt": after}
	}
	return base, nil
}

// paginate runs a find with limit+1 to detect a next page without a count.
func paginate[D any, T any](
	ctx context.Context, col *mongo.Collection, filter bson.M, limit int,
	conv func(D) T, idOf func(D) string,
) (ports.Page[T], error) {
	opts := options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit + 1))
	cur, err := col.Find(ctx, filter, opts)
	if err != nil {
		return ports.Page[T]{}, err
	}
	defer cur.Close(ctx)

	var docs []D
	if err := cur.All(ctx, &docs); err != nil {
		return ports.Page[T]{}, err
	}

	page := ports.Page[T]{Data: make([]T, 0, limit)}
	for i, d := range docs {
		if i == limit {
			// The extra row only tells us a next page exists; it is not returned.
			break
		}
		page.Data = append(page.Data, conv(d))
	}
	if len(docs) > limit {
		page.NextCursor = encodeCursor(idOf(docs[limit-1]))
	}
	return page, nil
}

// ---- Regions ----

type RegionRepo struct{ col *mongo.Collection }

func NewRegionRepo(s *Store) *RegionRepo { return &RegionRepo{col: s.db.Collection(ColRegions)} }

func (r *RegionRepo) List(ctx context.Context, p ports.ListParams) (ports.Page[geography.Region], error) {
	p = p.Normalize()
	f, err := cursorFilter(bson.M{"status": string(geography.StatusActive)}, p.Cursor)
	if err != nil {
		return ports.Page[geography.Region]{}, err
	}
	return paginate(ctx, r.col, f, p.Limit, fromRegionDoc, func(d regionDoc) string { return d.ID })
}

func (r *RegionRepo) Get(ctx context.Context, id string) (*geography.Region, error) {
	var d regionDoc
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := fromRegionDoc(d)
	return &out, nil
}

func (r *RegionRepo) Upsert(ctx context.Context, in geography.Region) (bool, error) {
	if err := in.Validate(); err != nil {
		return false, err
	}
	doc := toRegionDoc(in)
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return false, err
	}
	return res.UpsertedCount > 0, nil
}

func (r *RegionRepo) Count(ctx context.Context) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{})
}

// ---- Districts ----

type DistrictRepo struct{ col *mongo.Collection }

func NewDistrictRepo(s *Store) *DistrictRepo {
	return &DistrictRepo{col: s.db.Collection(ColDistricts)}
}

func (r *DistrictRepo) List(ctx context.Context, f ports.DistrictFilter) (ports.Page[geography.District], error) {
	f.ListParams = f.ListParams.Normalize()
	q := bson.M{"status": string(geography.StatusActive)}
	if f.RegionID != "" {
		q["regionId"] = f.RegionID
	}
	if f.Query != "" {
		q["normalizedName"] = bson.M{"$regex": "^" + normalize.Name(f.Query)}
	}
	q, err := cursorFilter(q, f.Cursor)
	if err != nil {
		return ports.Page[geography.District]{}, err
	}
	return paginate(ctx, r.col, q, f.Limit, fromDistrictDoc, func(d districtDoc) string { return d.ID })
}

func (r *DistrictRepo) Get(ctx context.Context, id string) (*geography.District, error) {
	var d districtDoc
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := fromDistrictDoc(d)
	return &out, nil
}

func (r *DistrictRepo) Upsert(ctx context.Context, in geography.District) (bool, error) {
	if err := in.Validate(); err != nil {
		return false, err
	}
	doc := toDistrictDoc(in)
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return false, err
	}
	return res.UpsertedCount > 0, nil
}

func (r *DistrictRepo) Count(ctx context.Context) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{})
}

// CountOrphans backs the acceptance test that every active district references
// an active region (Spec 22.1).
func (r *DistrictRepo) CountOrphans(ctx context.Context) (int64, error) {
	cur, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": "ACTIVE"}}},
		{{Key: "$lookup", Value: bson.M{
			"from": ColRegions, "localField": "regionId", "foreignField": "_id", "as": "region",
		}}},
		{{Key: "$match", Value: bson.M{"region": bson.M{"$size": 0}}}},
		{{Key: "$count", Value: "orphans"}},
	})
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)
	var out []struct {
		Orphans int64 `bson:"orphans"`
	}
	if err := cur.All(ctx, &out); err != nil {
		return 0, err
	}
	if len(out) == 0 {
		return 0, nil
	}
	return out[0].Orphans, nil
}

// ---- Places ----

type PlaceRepo struct{ col *mongo.Collection }

func NewPlaceRepo(s *Store) *PlaceRepo { return &PlaceRepo{col: s.db.Collection(ColPlaces)} }

func (r *PlaceRepo) List(ctx context.Context, f ports.PlaceFilter) (ports.Page[geography.Place], error) {
	f.ListParams = f.ListParams.Normalize()
	q := bson.M{"status": string(geography.StatusActive)}
	if f.DistrictID != "" {
		q["districtId"] = f.DistrictID
	}
	if f.RegionID != "" {
		q["regionId"] = f.RegionID
	}
	if f.Type != "" {
		q["type"] = f.Type
	}
	if f.Query != "" {
		q["normalizedName"] = bson.M{"$regex": "^" + normalize.Name(f.Query)}
	}
	q, err := cursorFilter(q, f.Cursor)
	if err != nil {
		return ports.Page[geography.Place]{}, err
	}
	return paginate(ctx, r.col, q, f.Limit, fromPlaceDoc, func(d placeDoc) string { return d.ID })
}

func (r *PlaceRepo) Get(ctx context.Context, id string) (*geography.Place, error) {
	var d placeDoc
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := fromPlaceDoc(d)
	return &out, nil
}

func (r *PlaceRepo) Upsert(ctx context.Context, in geography.Place) (bool, error) {
	if err := in.Validate(); err != nil {
		return false, err
	}
	doc := toPlaceDoc(in)
	res, err := r.col.ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc, options.Replace().SetUpsert(true))
	if err != nil {
		return false, err
	}
	return res.UpsertedCount > 0, nil
}

func (r *PlaceRepo) Count(ctx context.Context) (int64, error) {
	return r.col.CountDocuments(ctx, bson.M{})
}

// Nearby uses $nearSphere against the 2dsphere centroid index. This is the
// MongoDB equivalent of PostGIS geography-distance proximity (Spec 10).
func (r *PlaceRepo) Nearby(
	ctx context.Context, c geography.Coordinate, radiusMeters, limit int,
) ([]geography.Place, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	filter := bson.M{
		"status": string(geography.StatusActive),
		"centroid": bson.M{
			"$nearSphere": bson.M{
				"$geometry":    bson.M{"type": "Point", "coordinates": []float64{c.Longitude, c.Latitude}},
				"$maxDistance": radiusMeters,
			},
		},
	}
	cur, err := r.col.Find(ctx, filter, options.Find().SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []placeDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := make([]geography.Place, 0, len(docs))
	for _, d := range docs {
		out = append(out, fromPlaceDoc(d))
	}
	return out, nil
}

// ---- Redirects ----

type RedirectRepo struct{ col *mongo.Collection }

func NewRedirectRepo(s *Store) *RedirectRepo {
	return &RedirectRepo{col: s.db.Collection(ColRedirects)}
}

func (r *RedirectRepo) Resolve(ctx context.Context, oldID string) (*geography.Redirect, error) {
	var d struct {
		ID       string `bson:"_id"`
		NewID    string `bson:"newId"`
		Reason   string `bson:"reason"`
		MergedAt string `bson:"mergedAt"`
	}
	if err := r.col.FindOne(ctx, bson.M{"_id": oldID}).Decode(&d); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &geography.Redirect{OldID: d.ID, NewID: d.NewID, Reason: d.Reason, MergedAt: d.MergedAt}, nil
}

func (r *RedirectRepo) Put(ctx context.Context, in geography.Redirect) error {
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": in.OldID}, bson.M{
		"_id": in.OldID, "newId": in.NewID, "reason": in.Reason, "mergedAt": in.MergedAt,
	}, options.Replace().SetUpsert(true))
	return err
}

// SetGeometry attaches a validated boundary polygon to a region.
//
// Validation happens BEFORE this call, in the ingestion layer: MongoDB accepts
// a self-intersecting polygon and then returns silently wrong $geoIntersects
// results, so a bad polygon here would corrupt every containment query rather
// than raising an error.
func (r *RegionRepo) SetGeometry(ctx context.Context, id string, g *geography.Geometry) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id},
		bson.M{"$set": bson.M{"geometry": geomOf(g)}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// SetGeometry attaches a validated boundary polygon to a district.
func (r *DistrictRepo) SetGeometry(ctx context.Context, id string, g *geography.Geometry) error {
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id},
		bson.M{"$set": bson.M{"geometry": geomOf(g)}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// AssignDistrictsByContainment fills in each place's district using PostGIS-style
// point-in-polygon containment, via MongoDB's $geoIntersects against the
// district boundaries.
//
// This is what turns a coordinate into an administrative answer, and it is why
// boundary geometry had to be validated first: a self-intersecting polygon
// would assign places to the wrong district with no error anywhere.
//
// Returns (assigned, unassigned).
func (r *PlaceRepo) AssignDistrictsByContainment(
	ctx context.Context, districts *DistrictRepo, batchLog func(string),
) (int, int, error) {
	cur, err := districts.col.Find(ctx,
		bson.M{"geometry": bson.M{"$ne": nil}},
		options.Find().SetProjection(bson.M{"_id": 1, "name": 1, "regionId": 1, "regionName": 1}))
	if err != nil {
		return 0, 0, err
	}
	var dists []struct {
		ID         string `bson:"_id"`
		Name       string `bson:"name"`
		RegionID   string `bson:"regionId"`
		RegionName string `bson:"regionName"`
	}
	if err := cur.All(ctx, &dists); err != nil {
		return 0, 0, err
	}
	if len(dists) == 0 {
		return 0, 0, errors.New("no district has a boundary; import boundaries first")
	}

	assigned := 0
	for i, d := range dists {
		// One bulk update per district rather than one query per place: 260
		// spatial queries instead of 15,925.
		var full struct {
			Geometry any `bson:"geometry"`
		}
		if err := districts.col.FindOne(ctx, bson.M{"_id": d.ID},
			options.FindOne().SetProjection(bson.M{"geometry": 1})).Decode(&full); err != nil {
			continue
		}
		ur, err := r.col.UpdateMany(ctx,
			bson.M{
				"districtId": bson.M{"$in": bson.A{nil, ""}},
				"centroid":   bson.M{"$geoWithin": bson.M{"$geometry": full.Geometry}},
			},
			bson.M{"$set": bson.M{
				"districtId":   d.ID,
				"districtName": d.Name,
			}},
		)
		if err != nil {
			return assigned, 0, fmt.Errorf("assign %s: %w", d.Name, err)
		}
		assigned += int(ur.ModifiedCount)
		if batchLog != nil && (i+1)%50 == 0 {
			batchLog(fmt.Sprintf("  %d/%d districts processed, %d places assigned so far", i+1, len(dists), assigned))
		}
	}

	unassigned, err := r.col.CountDocuments(ctx, bson.M{
		"centroid":   bson.M{"$ne": nil},
		"districtId": bson.M{"$in": bson.A{nil, ""}},
	})
	if err != nil {
		return assigned, 0, err
	}
	return assigned, int(unassigned), nil
}
