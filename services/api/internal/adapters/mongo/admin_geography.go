package mongo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/normalize"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

type AdminGeographyRepo struct{ s *Store }

var errIdempotentReplay = errors.New("idempotent replay")

func NewAdminGeographyRepo(s *Store) *AdminGeographyRepo { return &AdminGeographyRepo{s: s} }

func (r *AdminGeographyRepo) atomic(ctx context.Context, evidence audit.Entry, mutate func(context.Context) error) error {
	session, err := r.s.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)
	auditAppendMu.Lock()
	defer auditAppendMu.Unlock()
	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		if x := mutate(tx); errors.Is(x, errIdempotentReplay) {
			return nil, nil
		} else if x != nil {
			return nil, x
		}
		_, x := NewAuditRepo(r.s).append(tx, evidence)
		return nil, x
	})
	return err
}

func (r *AdminGeographyRepo) claim(ctx context.Context, e audit.Entry) (bool, error) {
	if e.RequestID == "" {
		return false, errors.New("idempotency request id is required")
	}
	before, err := audit.CanonicalJSON(e.Before)
	if err != nil {
		return false, err
	}
	after, err := audit.CanonicalJSON(e.After)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256([]byte(string(e.Action) + "\x00" + e.Target.Kind + "\x00" + e.Target.ID + "\x00" + before + "\x00" + after))
	fingerprint := hex.EncodeToString(sum[:])
	doc := bson.M{"_id": e.RequestID, "fingerprint": fingerprint, "targetKind": e.Target.Kind, "targetId": e.Target.ID, "createdAt": time.Now().UTC()}
	res, err := r.s.db.Collection(ColAdminMutationKeys).UpdateOne(ctx, bson.M{"_id": e.RequestID}, bson.M{"$setOnInsert": doc}, options.UpdateOne().SetUpsert(true))
	if err != nil {
		return false, err
	}
	if res.UpsertedCount == 1 {
		return false, nil
	}
	var existing struct {
		Fingerprint string `bson:"fingerprint"`
	}
	if err = r.s.db.Collection(ColAdminMutationKeys).FindOne(ctx, bson.M{"_id": e.RequestID}).Decode(&existing); err != nil {
		return false, err
	}
	if existing.Fingerprint != fingerprint {
		return false, errors.New("idempotency key was already used for a different mutation")
	}
	return true, nil
}

func (r *AdminGeographyRepo) claimed(ctx context.Context, e audit.Entry, fn func(context.Context) error) error {
	replay, err := r.claim(ctx, e)
	if err != nil {
		return err
	}
	if replay {
		return errIdempotentReplay
	}
	return fn(ctx)
}

func insertOne(ctx context.Context, col *mongo.Collection, doc any) error {
	_, err := col.InsertOne(ctx, doc)
	if mongo.IsDuplicateKeyError(err) {
		return fmt.Errorf("record already exists: %w", err)
	}
	return err
}

func (r *AdminGeographyRepo) CreateRegion(ctx context.Context, v geography.Region, e audit.Entry) error {
	return r.atomic(ctx, e, func(tx context.Context) error {
		return r.claimed(tx, e, func(tx context.Context) error { return insertOne(tx, r.s.db.Collection(ColRegions), toRegionDoc(v)) })
	})
}
func (r *AdminGeographyRepo) CreateDistrict(ctx context.Context, v geography.District, e audit.Entry) error {
	return r.atomic(ctx, e, func(tx context.Context) error {
		return r.claimed(tx, e, func(tx context.Context) error {
			return insertOne(tx, r.s.db.Collection(ColDistricts), toDistrictDoc(v))
		})
	})
}
func (r *AdminGeographyRepo) CreatePlace(ctx context.Context, v geography.Place, e audit.Entry) error {
	return r.atomic(ctx, e, func(tx context.Context) error {
		return r.claimed(tx, e, func(tx context.Context) error { return insertOne(tx, r.s.db.Collection(ColPlaces), toPlaceDoc(v)) })
	})
}
func (r *AdminGeographyRepo) CreateRoad(ctx context.Context, v geography.Road, e audit.Entry) error {
	d := roadDoc{ID: v.ID, Name: v.Name, NormalizedName: normalize.Name(v.Name), Ref: v.Ref, Class: string(v.Class), RegionID: v.RegionID, RegionName: v.RegionName, DistrictID: v.DistrictID, Geometry: geomOf(v.Geometry), Status: string(v.Status), VerificationStatus: string(v.VerificationStatus), Provenance: provOf(v.Provenance), Attribution: v.Attribution, DatasetVersion: v.DatasetVersion}
	return r.atomic(ctx, e, func(tx context.Context) error {
		return r.claimed(tx, e, func(tx context.Context) error { return insertOne(tx, r.s.db.Collection(ColRoads), d) })
	})
}
func (r *AdminGeographyRepo) CreatePOI(ctx context.Context, v geography.POI, e audit.Entry) error {
	d := poiDoc{ID: v.ID, Name: v.Name, NormalizedName: normalize.Name(v.Name), Class: string(v.Class), Category: v.Category, RegionID: v.RegionID, RegionName: v.RegionName, DistrictID: v.DistrictID, Centroid: pointOf(v.Centroid), Status: string(v.Status), VerificationStatus: string(v.VerificationStatus), Provenance: provOf(v.Provenance), Attribution: v.Attribution, DatasetVersion: v.DatasetVersion}
	return r.atomic(ctx, e, func(tx context.Context) error {
		return r.claimed(tx, e, func(tx context.Context) error { return insertOne(tx, r.s.db.Collection(ColPOIs), d) })
	})
}

func (r *AdminGeographyRepo) update(ctx context.Context, colName string, id string, doc any, e audit.Entry) error {
	set, err := mergeFields(doc)
	if err != nil {
		return err
	}
	delete(set, "datasetVersion")
	return r.atomic(ctx, e, func(tx context.Context) error {
		return r.claimed(tx, e, func(tx context.Context) error {
			res, err := r.s.db.Collection(colName).UpdateOne(tx, bson.M{"_id": id, "status": string(geography.StatusActive)}, bson.M{"$set": set})
			if err != nil {
				return err
			}
			if res.MatchedCount == 0 {
				return ErrNotFound
			}
			return nil
		})
	})
}
func (r *AdminGeographyRepo) UpdateRegion(ctx context.Context, v geography.Region, e audit.Entry) error {
	return r.update(ctx, ColRegions, v.ID, toRegionDoc(v), e)
}
func (r *AdminGeographyRepo) UpdateDistrict(ctx context.Context, v geography.District, e audit.Entry) error {
	return r.update(ctx, ColDistricts, v.ID, toDistrictDoc(v), e)
}
func (r *AdminGeographyRepo) UpdatePlace(ctx context.Context, v geography.Place, e audit.Entry) error {
	return r.update(ctx, ColPlaces, v.ID, toPlaceDoc(v), e)
}

func entityCollection(kind string) (string, bool) {
	switch kind {
	case "region":
		return ColRegions, true
	case "district":
		return ColDistricts, true
	case "place":
		return ColPlaces, true
	case "road":
		return ColRoads, true
	case "poi":
		return ColPOIs, true
	}
	return "", false
}

func (r *AdminGeographyRepo) Deprecate(ctx context.Context, kind, id, target, reason string, e audit.Entry) error {
	colName, ok := entityCollection(kind)
	if !ok {
		return errors.New("unsupported geography kind")
	}
	return r.atomic(ctx, e, func(tx context.Context) error {
		if replay, err := r.claim(tx, e); err != nil {
			return err
		} else if replay {
			return errIdempotentReplay
		}
		col := r.s.db.Collection(colName)
		if target != "" {
			if target == id {
				return errors.New("record cannot redirect to itself")
			}
			if err := col.FindOne(tx, bson.M{"_id": target, "status": string(geography.StatusActive)}).Err(); err != nil {
				return fmt.Errorf("redirect target is not active: %w", err)
			}
		}
		status := geography.StatusDeprecated
		if target != "" {
			status = geography.StatusMerged
		}
		res, err := col.UpdateOne(tx, bson.M{"_id": id, "status": string(geography.StatusActive)}, bson.M{"$set": bson.M{"status": string(status)}})
		if err != nil {
			return err
		}
		if res.MatchedCount == 0 {
			return ErrNotFound
		}
		if target != "" {
			_, err = r.s.db.Collection(ColRedirects).ReplaceOne(tx, bson.M{"_id": id}, bson.M{"_id": id, "kind": kind, "newId": target, "reason": reason, "mergedAt": time.Now().UTC().Format(time.RFC3339)}, options.Replace().SetUpsert(true))
		}
		return err
	})
}

func (r *AdminGeographyRepo) ListRedirects(ctx context.Context, p ports.ListParams) (ports.Page[geography.Redirect], error) {
	p = p.Normalize()
	q, err := cursorFilter(bson.M{}, p.Cursor)
	if err != nil {
		return ports.Page[geography.Redirect]{}, err
	}
	cur, err := r.s.db.Collection(ColRedirects).Find(ctx, q, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(p.Limit+1)))
	if err != nil {
		return ports.Page[geography.Redirect]{}, err
	}
	defer cur.Close(ctx)
	// Inline cannot decode differently named fields, so use a map-backed pass.
	var raw []bson.M
	if err = cur.All(ctx, &raw); err != nil {
		return ports.Page[geography.Redirect]{}, err
	}
	out := ports.Page[geography.Redirect]{Data: make([]geography.Redirect, 0, p.Limit)}
	for i, d := range raw {
		if i == p.Limit {
			break
		}
		out.Data = append(out.Data, geography.Redirect{OldID: fmt.Sprint(d["_id"]), Kind: fmt.Sprint(d["kind"]), NewID: fmt.Sprint(d["newId"]), Reason: fmt.Sprint(d["reason"]), MergedAt: fmt.Sprint(d["mergedAt"])})
	}
	if len(raw) > p.Limit {
		out.NextCursor = encodeCursor(out.Data[len(out.Data)-1].OldID)
	}
	return out, nil
}

type aliasRecord struct {
	ID              string `bson:"_id"`
	PlaceID         string `bson:"placeId"`
	Value           string `bson:"value"`
	NormalizedValue string `bson:"normalizedValue"`
	AliasType       string `bson:"aliasType"`
	Language        string `bson:"language"`
	IsPreferred     bool   `bson:"isPreferred"`
	Status          string `bson:"status"`
}

func aliasDomain(d aliasRecord) geography.Alias {
	return geography.Alias{ID: d.ID, PlaceID: d.PlaceID, Value: d.Value, NormalizedValue: d.NormalizedValue, AliasType: d.AliasType, Language: d.Language, IsPreferred: d.IsPreferred, Status: geography.Status(d.Status)}
}
func (r *AdminGeographyRepo) ListAliases(ctx context.Context, placeID string) ([]geography.Alias, error) {
	cur, err := r.s.db.Collection(ColAliases).Find(ctx, bson.M{"placeId": placeID}, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var ds []aliasRecord
	if err = cur.All(ctx, &ds); err != nil {
		return nil, err
	}
	out := make([]geography.Alias, 0, len(ds))
	for _, d := range ds {
		out = append(out, aliasDomain(d))
	}
	return out, nil
}
func (r *AdminGeographyRepo) CreateAlias(ctx context.Context, a geography.Alias, e audit.Entry) error {
	d := aliasRecord{a.ID, a.PlaceID, a.Value, normalize.Name(a.Value), a.AliasType, a.Language, a.IsPreferred, string(geography.StatusActive)}
	return r.atomic(ctx, e, func(tx context.Context) error {
		if replay, err := r.claim(tx, e); err != nil {
			return err
		} else if replay {
			return errIdempotentReplay
		}
		if err := r.s.db.Collection(ColPlaces).FindOne(tx, bson.M{"_id": a.PlaceID, "status": string(geography.StatusActive)}).Err(); err != nil {
			return err
		}
		if err := insertOne(tx, r.s.db.Collection(ColAliases), d); err != nil {
			return err
		}
		res, err := r.s.db.Collection(ColPlaces).UpdateOne(tx, bson.M{"_id": a.PlaceID, "aliases.normalizedValue": bson.M{"$ne": normalize.Name(a.Value)}}, bson.M{"$push": bson.M{"aliases": aliasDoc{Value: a.Value, NormalizedValue: normalize.Name(a.Value), AliasType: a.AliasType, Language: a.Language, IsPreferred: a.IsPreferred}}})
		if err == nil && res.MatchedCount == 0 {
			return errors.New("alias already exists on the canonical place")
		}
		return err
	})
}
func (r *AdminGeographyRepo) DeprecateAlias(ctx context.Context, placeID, id string, e audit.Entry) error {
	return r.atomic(ctx, e, func(tx context.Context) error {
		if replay, err := r.claim(tx, e); err != nil {
			return err
		} else if replay {
			return errIdempotentReplay
		}
		var d aliasRecord
		if err := r.s.db.Collection(ColAliases).FindOne(tx, bson.M{"_id": id, "placeId": placeID, "status": string(geography.StatusActive)}).Decode(&d); err != nil {
			return err
		}
		res, err := r.s.db.Collection(ColAliases).UpdateOne(tx, bson.M{"_id": id, "status": string(geography.StatusActive)}, bson.M{"$set": bson.M{"status": string(geography.StatusDeprecated)}})
		if err != nil || res.MatchedCount == 0 {
			return err
		}
		_, err = r.s.db.Collection(ColPlaces).UpdateOne(tx, bson.M{"_id": placeID}, bson.M{"$pull": bson.M{"aliases": bson.M{"normalizedValue": d.NormalizedValue}}})
		return err
	})
}
