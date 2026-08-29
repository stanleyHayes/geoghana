package mongo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
)

// Migrate creates collections with JSON Schema validators and the indexes the
// query patterns need.
//
// MongoDB is schemaless by default; validators make it not so. Every collection
// ships validationLevel "strict" / validationAction "error" — a collection
// without a validator must not ship (plan rule R16).
//
// Migrations are idempotent: re-running produces no change.
func Migrate(ctx context.Context, db *mongo.Database) error {
	steps := []struct {
		name      string
		validator bson.M
		indexes   []mongo.IndexModel
	}{
		{
			name:      ColRegions,
			validator: regionSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "normalizedName", Value: 1}}},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "name", Value: 1}}},
				{Keys: bson.D{{Key: "centroid", Value: "2dsphere"}}},
				{Keys: bson.D{{Key: "geometry", Value: "2dsphere"}}},
			},
		},
		{
			name:      ColDistricts,
			validator: districtSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "regionId", Value: 1}, {Key: "name", Value: 1}}},
				{Keys: bson.D{{Key: "normalizedName", Value: 1}}},
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "name", Value: 1}}},
				{Keys: bson.D{{Key: "centroid", Value: "2dsphere"}}},
				{Keys: bson.D{{Key: "geometry", Value: "2dsphere"}}},
			},
		},
		{
			name:      ColPlaces,
			validator: placeSchema(),
			indexes: []mongo.IndexModel{
				// Serves GET /districts/{id}/places with type faceting.
				{Keys: bson.D{{Key: "districtId", Value: 1}, {Key: "type", Value: 1}, {Key: "normalizedName", Value: 1}}},
				{Keys: bson.D{{Key: "regionId", Value: 1}, {Key: "type", Value: 1}}},
				{Keys: bson.D{{Key: "normalizedName", Value: 1}}},
				{Keys: bson.D{{Key: "aliases.normalizedValue", Value: 1}}},
				{Keys: bson.D{{Key: "centroid", Value: "2dsphere"}}}, // $nearSphere
				{Keys: bson.D{{Key: "geometry", Value: "2dsphere"}}}, // $geoIntersects
				{Keys: bson.D{{Key: "parentPlaceId", Value: 1}}},
			},
		},
		{
			name:      ColRedirects,
			validator: redirectSchema(),
			indexes:   []mongo.IndexModel{{Keys: bson.D{{Key: "newId", Value: 1}}}},
		},
		{
			name:      ColOrganizations,
			validator: organizationSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "ownerId", Value: 1}, {Key: "createdAt", Value: 1}}},
			},
		},
		{
			name:      ColApplications,
			validator: applicationSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: "createdAt", Value: 1}}},
			},
		},
		{
			name:      ColOrgInvitations,
			validator: organizationInvitationSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: "status", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
			},
		},
		{
			name:      ColAPIKeys,
			validator: apiKeySchema(),
			indexes: []mongo.IndexModel{
				// The lookup path on every authenticated request.
				{Keys: bson.D{{Key: "prefix", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: "createdAt", Value: -1}}},
				{Keys: bson.D{{Key: "applicationId", Value: 1}}},
			},
		},
		{
			name:      ColAccounts,
			validator: accountSchema(),
			indexes: []mongo.IndexModel{
				// Unique on the NORMALIZED email: this is what makes
				// registration race-safe. Two simultaneous sign-ups for the
				// same address cannot both win, whatever the handler does.
				{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "role", Value: 1}}},
			},
		},
		{
			name:      ColSessions,
			validator: sessionSchema(),
			indexes: []mongo.IndexModel{
				// The lookup on every authenticated request.
				{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "accountId", Value: 1}, {Key: "lastUsedAt", Value: -1}}},
				// Expired rows are swept by Mongo rather than by a cron we
				// would have to remember to run. The grace period keeps a
				// superseded row around long enough for a replay to still be
				// recognised as theft rather than as an unknown token.
				{
					Keys:    bson.D{{Key: "expiresAt", Value: 1}},
					Options: options.Index().SetExpireAfterSeconds(int32((24 * time.Hour).Seconds())),
				},
			},
		},
		{
			name:      ColOneTimeTokens,
			validator: oneTimeTokenSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "hash", Value: 1}}, Options: options.Index().SetUnique(true)},
				{Keys: bson.D{{Key: "accountId", Value: 1}, {Key: "purpose", Value: 1}}},
				{
					Keys:    bson.D{{Key: "expiresAt", Value: 1}},
					Options: options.Index().SetExpireAfterSeconds(int32((48 * time.Hour).Seconds())),
				},
			},
		},
		{
			name:      ColWebAuthnChal,
			validator: webauthnChallengeSchema(),
			indexes: []mongo.IndexModel{
				// Challenges are swept by Mongo. A stale challenge that
				// outlived its ceremony is a replay opportunity, so the TTL is
				// short and the sweep is not something we have to remember.
				{
					Keys:    bson.D{{Key: "expiresAt", Value: 1}},
					Options: options.Index().SetExpireAfterSeconds(0),
				},
				{Keys: bson.D{{Key: "accountId", Value: 1}}},
			},
		},
		{
			name:      ColAuditLog,
			validator: auditSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "at", Value: -1}}},
				{Keys: bson.D{{Key: "actorId", Value: 1}, {Key: "at", Value: -1}}},
				{Keys: bson.D{{Key: "action", Value: 1}, {Key: "at", Value: -1}}},
				{Keys: bson.D{{Key: "targetId", Value: 1}, {Key: "at", Value: -1}}},
			},
		},
		{
			name:      ColUsageEvents,
			validator: usageEventSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "organizationId", Value: 1}, {Key: "applicationId", Value: 1}, {Key: "at", Value: -1}}},
				{Keys: bson.D{{Key: "at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(int32((30 * 24 * time.Hour).Seconds()))},
				{Keys: bson.D{{Key: "requestId", Value: 1}}, Options: options.Index().SetUnique(true)},
			},
		},
		{
			name:      ColDatasetVersion,
			validator: datasetVersionSchema(),
			indexes: []mongo.IndexModel{
				{Keys: bson.D{{Key: "status", Value: 1}, {Key: "publishedAt", Value: -1}}},
			},
		},
	}

	existing, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("list collections: %w", err)
	}
	have := map[string]bool{}
	for _, n := range existing {
		have[n] = true
	}
	if err := backfillIdentityDocuments(ctx, db, have); err != nil {
		return err
	}
	if err := backfillGeographyMetadata(ctx, db, have); err != nil {
		return err
	}
	if err := migrateGeographyIDs(ctx, db, have); err != nil {
		return err
	}

	for _, s := range steps {
		opts := options.CreateCollection().
			SetValidator(s.validator).
			SetValidationLevel("strict").
			SetValidationAction("error")

		if !have[s.name] {
			if err := db.CreateCollection(ctx, s.name, opts); err != nil {
				return fmt.Errorf("create %s: %w", s.name, err)
			}
		} else {
			// Idempotent re-apply of the validator on an existing collection.
			cmd := bson.D{
				{Key: "collMod", Value: s.name},
				{Key: "validator", Value: s.validator},
				{Key: "validationLevel", Value: "strict"},
				{Key: "validationAction", Value: "error"},
			}
			if err := db.RunCommand(ctx, cmd).Err(); err != nil {
				return fmt.Errorf("collMod %s: %w", s.name, err)
			}
		}
		if len(s.indexes) > 0 {
			if _, err := db.Collection(s.name).Indexes().CreateMany(ctx, s.indexes); err != nil {
				return fmt.Errorf("indexes %s: %w", s.name, err)
			}
		}
	}
	return nil
}

func backfillIdentityDocuments(ctx context.Context, db *mongo.Database, have map[string]bool) error {
	if have[ColOrganizations] {
		_, err := db.Collection(ColOrganizations).UpdateMany(ctx, bson.M{"members": bson.M{"$exists": false}}, mongo.Pipeline{{{Key: "$set", Value: bson.M{"members": bson.A{bson.M{"accountId": "$ownerId", "email": "", "role": "OWNER", "joinedAt": bson.M{"$ifNull": bson.A{"$createdAt", time.Now().UTC()}}}}}}}})
		if err != nil {
			return fmt.Errorf("backfill organization members: %w", err)
		}
	}
	if have[ColApplications] {
		_, err := db.Collection(ColApplications).UpdateMany(ctx, bson.M{}, bson.M{"$set": bson.M{"environments": bson.A{"test", "live"}, "plan": "free"}})
		if err != nil {
			return fmt.Errorf("backfill application metadata: %w", err)
		}
	}
	return nil
}

// backfillGeographyMetadata brings legacy rows forward before the stricter
// validators below are applied. The payload hash covers the normalized source
// document available at migration time; it is never recomputed once present.
func backfillGeographyMetadata(ctx context.Context, db *mongo.Database, have map[string]bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	for _, collection := range []string{ColRegions, ColDistricts, ColPlaces} {
		if !have[collection] {
			continue
		}
		col := db.Collection(collection)
		filter := bson.M{"$or": bson.A{
			bson.M{"datasetVersion": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"datasetVersion": bson.M{"$exists": false}},
			bson.M{"provenance.externalId": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"provenance.externalId": bson.M{"$exists": false}},
			bson.M{"provenance.retrievedAt": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"provenance.retrievedAt": bson.M{"$exists": false}},
			bson.M{"provenance.sourcePayloadHash": bson.M{"$in": bson.A{nil, ""}}},
			bson.M{"provenance.sourcePayloadHash": bson.M{"$exists": false}},
		}}
		cursor, err := col.Find(ctx, filter)
		if err != nil {
			return fmt.Errorf("find %s metadata gaps: %w", collection, err)
		}
		for cursor.Next(ctx) {
			var doc bson.M
			if err := cursor.Decode(&doc); err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("decode %s metadata gap: %w", collection, err)
			}
			set, err := geographyMetadataDefaults(doc, now)
			if err != nil {
				cursor.Close(ctx)
				return fmt.Errorf("prepare %s metadata backfill: %w", collection, err)
			}
			if len(set) > 0 {
				if _, err := col.UpdateOne(ctx, bson.M{"_id": doc["_id"]}, bson.M{"$set": set}); err != nil {
					cursor.Close(ctx)
					return fmt.Errorf("backfill %s %v: %w", collection, doc["_id"], err)
				}
			}
		}
		if err := cursor.Err(); err != nil {
			cursor.Close(ctx)
			return fmt.Errorf("scan %s metadata gaps: %w", collection, err)
		}
		cursor.Close(ctx)
	}
	return nil
}

func geographyMetadataDefaults(doc bson.M, retrievedAt string) (bson.M, error) {
	set := bson.M{}
	id := strings.TrimSpace(fmt.Sprint(doc["_id"]))
	provenance, _ := doc["provenance"].(bson.M)
	if provenance == nil {
		provenance = bson.M{}
	}
	if strings.TrimSpace(fmt.Sprint(provenance["externalId"])) == "" || provenance["externalId"] == nil {
		set["provenance.externalId"] = id
	}
	if strings.TrimSpace(fmt.Sprint(provenance["retrievedAt"])) == "" || provenance["retrievedAt"] == nil {
		set["provenance.retrievedAt"] = retrievedAt
	}
	if strings.TrimSpace(fmt.Sprint(doc["datasetVersion"])) == "" || doc["datasetVersion"] == nil {
		set["datasetVersion"] = "legacy-unversioned"
	}
	if strings.TrimSpace(fmt.Sprint(provenance["sourcePayloadHash"])) == "" || provenance["sourcePayloadHash"] == nil {
		delete(provenance, "sourcePayloadHash")
		doc["provenance"] = provenance
		payload, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(payload)
		set["provenance.sourcePayloadHash"] = hex.EncodeToString(sum[:])
	}
	return set, nil
}

type geographyIDMigration struct {
	collection string
	entity     string
	documents  []bson.M
	ids        map[string]string
}

// migrateGeographyIDs replaces legacy human-readable primary keys with the
// deterministic ULIDs required by R7. It copies and relinks the entire graph,
// writes a redirect for every published legacy id, then removes the old rows —
// all in one transaction so a failed migration cannot expose a mixed graph.
func migrateGeographyIDs(ctx context.Context, db *mongo.Database, have map[string]bool) error {
	definitions := []struct{ collection, entity string }{
		{ColRegions, "region"}, {ColDistricts, "district"}, {ColPlaces, "place"},
	}
	migrations := make([]geographyIDMigration, 0, len(definitions))
	legacyCount := 0
	for _, definition := range definitions {
		migration := geographyIDMigration{collection: definition.collection, entity: definition.entity, ids: map[string]string{}}
		if have[definition.collection] {
			cursor, err := db.Collection(definition.collection).Find(ctx, bson.M{})
			if err != nil {
				return fmt.Errorf("read %s for id migration: %w", definition.collection, err)
			}
			if err := cursor.All(ctx, &migration.documents); err != nil {
				return fmt.Errorf("decode %s for id migration: %w", definition.collection, err)
			}
		}
		for _, doc := range migration.documents {
			oldID := fmt.Sprint(doc["_id"])
			if geography.IsULID(oldID) {
				migration.ids[oldID] = oldID
				continue
			}
			provenance, err := asBSONMap(doc["provenance"])
			if err != nil {
				return fmt.Errorf("decode %s provenance for %s: %w", definition.entity, oldID, err)
			}
			newID, err := geography.StableID(definition.entity,
				fmt.Sprint(provenance["sourceId"]), fmt.Sprint(provenance["externalId"]))
			if err != nil {
				return fmt.Errorf("derive %s id for %s: %w", definition.entity, oldID, err)
			}
			for previousOldID, previousNewID := range migration.ids {
				if previousNewID == newID && previousOldID != oldID {
					return fmt.Errorf("%s id collision: %s and %s map to %s", definition.entity, previousOldID, oldID, newID)
				}
			}
			migration.ids[oldID] = newID
			legacyCount++
		}
		migrations = append(migrations, migration)
	}
	if legacyCount == 0 {
		return nil
	}

	regionIDs, districtIDs, placeIDs := migrations[0].ids, migrations[1].ids, migrations[2].ids
	existingRedirects := map[string]string{}
	if have[ColRedirects] {
		cursor, err := db.Collection(ColRedirects).Find(ctx, bson.M{})
		if err != nil {
			return fmt.Errorf("read redirects for id migration: %w", err)
		}
		var redirects []bson.M
		if err := cursor.All(ctx, &redirects); err != nil {
			return fmt.Errorf("decode redirects for id migration: %w", err)
		}
		for _, redirect := range redirects {
			existingRedirects[fmt.Sprint(redirect["_id"])] = fmt.Sprint(redirect["newId"])
		}
	}
	migratedAt := time.Now().UTC().Format(time.RFC3339)
	session, err := db.Client().StartSession()
	if err != nil {
		return fmt.Errorf("start geography id migration session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		for _, migration := range migrations {
			var inserts []any
			var legacyIDs []string
			var redirects []mongo.WriteModel
			for _, original := range migration.documents {
				oldID := fmt.Sprint(original["_id"])
				newID := migration.ids[oldID]
				if oldID == newID {
					continue
				}
				doc := cloneBSONMap(original)
				doc["_id"] = newID
				switch migration.entity {
				case "district":
					doc["regionId"] = mappedID(regionIDs, doc["regionId"])
				case "place":
					doc["regionId"] = mappedID(regionIDs, doc["regionId"])
					doc["districtId"] = mappedID(districtIDs, doc["districtId"])
					doc["parentPlaceId"] = mappedID(placeIDs, doc["parentPlaceId"])
				}
				inserts = append(inserts, doc)
				legacyIDs = append(legacyIDs, oldID)
				redirectTarget := newID
				if existingTarget := existingRedirects[oldID]; existingTarget != "" {
					redirectTarget = mapAnyGeographyID(existingTarget, regionIDs, districtIDs, placeIDs)
				}
				redirects = append(redirects, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": oldID}).
					SetUpdate(bson.M{"$set": bson.M{"newId": redirectTarget, "reason": "id-format-migration", "mergedAt": migratedAt}}).
					SetUpsert(true))
				if redirectTarget != newID {
					redirects = append(redirects, mongo.NewUpdateOneModel().
						SetFilter(bson.M{"_id": newID}).
						SetUpdate(bson.M{"$set": bson.M{"newId": redirectTarget, "reason": "preserved-merge-lineage", "mergedAt": migratedAt}}).
						SetUpsert(true))
				}
			}
			if len(inserts) == 0 {
				continue
			}
			if _, err := db.Collection(migration.collection).InsertMany(tx, inserts); err != nil {
				return nil, fmt.Errorf("insert ULID %s rows: %w", migration.collection, err)
			}
			if _, err := db.Collection(ColRedirects).BulkWrite(tx, redirects); err != nil {
				return nil, fmt.Errorf("write %s legacy redirects: %w", migration.collection, err)
			}
			if _, err := db.Collection(migration.collection).DeleteMany(tx, bson.M{"_id": bson.M{"$in": legacyIDs}}); err != nil {
				return nil, fmt.Errorf("remove legacy %s rows: %w", migration.collection, err)
			}
		}
		var redirectRelinks []mongo.WriteModel
		for oldID, oldTarget := range existingRedirects {
			if newID, migratedWithRecord := regionIDs[oldID]; migratedWithRecord && newID != oldID {
				continue
			}
			if newID, migratedWithRecord := districtIDs[oldID]; migratedWithRecord && newID != oldID {
				continue
			}
			if newID, migratedWithRecord := placeIDs[oldID]; migratedWithRecord && newID != oldID {
				continue
			}
			newTarget := mapAnyGeographyID(oldTarget, regionIDs, districtIDs, placeIDs)
			if newTarget != oldTarget {
				redirectRelinks = append(redirectRelinks, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"_id": oldID}).SetUpdate(bson.M{"$set": bson.M{"newId": newTarget}}))
			}
		}
		if len(redirectRelinks) > 0 {
			if _, err := db.Collection(ColRedirects).BulkWrite(tx, redirectRelinks); err != nil {
				return nil, fmt.Errorf("relink existing geography redirects: %w", err)
			}
		}
		return legacyCount, nil
	})
	if err != nil {
		return fmt.Errorf("migrate geography ids: %w", err)
	}
	return nil
}

func mapAnyGeographyID(id string, maps ...map[string]string) string {
	for _, ids := range maps {
		if next, ok := ids[id]; ok {
			return next
		}
	}
	return id
}

func mappedID(ids map[string]string, value any) any {
	old := strings.TrimSpace(fmt.Sprint(value))
	if old == "" || value == nil {
		return value
	}
	if next, ok := ids[old]; ok {
		return next
	}
	return value
}

func cloneBSONMap(source bson.M) bson.M {
	raw, _ := bson.Marshal(source)
	var clone bson.M
	_ = bson.Unmarshal(raw, &clone)
	return clone
}

func asBSONMap(value any) (bson.M, error) {
	raw, err := bson.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out bson.M
	if err := bson.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func usageEventSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": bson.A{"_id", "requestId", "organizationId", "applicationId", "keyId", "protocol", "operation", "status", "success", "latencyMs", "quotaCost", "quotaLimit", "quotaRemaining", "geography", "at"},
		"properties": bson.M{
			"_id": bson.M{"bsonType": "string"}, "requestId": bson.M{"bsonType": "string"},
			"organizationId": bson.M{"bsonType": "string"}, "applicationId": bson.M{"bsonType": "string"}, "keyId": bson.M{"bsonType": "string"},
			"protocol": bson.M{"enum": bson.A{"rest", "graphql", "grpc"}}, "operation": bson.M{"bsonType": "string"}, "status": bson.M{"bsonType": "string"},
			"success": bson.M{"bsonType": "bool"}, "latencyMs": bson.M{"bsonType": bson.A{"int", "long"}},
			"quotaCost": bson.M{"bsonType": bson.A{"int", "long"}}, "quotaLimit": bson.M{"bsonType": bson.A{"int", "long"}}, "quotaRemaining": bson.M{"bsonType": bson.A{"int", "long"}},
			"geography": bson.M{"bsonType": "string"}, "at": bson.M{"bsonType": "date"},
		},
	}}
}

// geoJSONSchema is the shared shape for any stored geometry. MongoDB only
// accepts WGS84, so no CRS field is permitted.
func geoJSONSchema() bson.M {
	return bson.M{
		"bsonType": []string{"object", "null"},
		"required": []string{"type", "coordinates"},
		"properties": bson.M{
			"type": bson.M{"enum": []string{
				"Point", "LineString", "Polygon", "MultiPoint", "MultiLineString", "MultiPolygon",
			}},
			"coordinates": bson.M{"bsonType": "array"},
		},
	}
}

func provenanceSchema() bson.M {
	return bson.M{
		"bsonType": "object",
		"required": []string{"sourceId", "externalId", "retrievedAt", "sourcePayloadHash"},
		"properties": bson.M{
			"sourceId":          bson.M{"bsonType": "string", "minLength": 1},
			"externalId":        bson.M{"bsonType": "string", "minLength": 1},
			"sourceUrl":         bson.M{"bsonType": []string{"string", "null"}},
			"retrievedAt":       bson.M{"bsonType": "string", "minLength": 1},
			"sourcePayloadHash": bson.M{"bsonType": "string", "minLength": 64, "maxLength": 64},
			"notes":             bson.M{"bsonType": []string{"string", "null"}},
		},
	}
}

var statusEnum = []string{"ACTIVE", "DEPRECATED", "MERGED"}
var verificationEnum = []string{
	"REFERENCE", "SEED_NEEDS_CANONICAL_RECONCILIATION", "REVIEWED", "CANONICAL",
}

const ulidPattern = "^[0-7][0-9A-HJKMNP-TV-Z]{25}$"

func regionSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "name", "status", "verificationStatus", "provenance", "datasetVersion"},
		"properties": bson.M{
			"_id":                bson.M{"bsonType": "string", "pattern": ulidPattern},
			"countryCode":        bson.M{"bsonType": "string"},
			"name":               bson.M{"bsonType": "string", "minLength": 1},
			"normalizedName":     bson.M{"bsonType": "string"},
			"capital":            bson.M{"bsonType": []string{"string", "null"}},
			"officialCode":       bson.M{"bsonType": []string{"string", "null"}},
			"status":             bson.M{"enum": statusEnum},
			"verificationStatus": bson.M{"enum": verificationEnum},
			"centroid":           geoJSONSchema(),
			"geometry":           geoJSONSchema(),
			"provenance":         provenanceSchema(),
			"datasetVersion":     bson.M{"bsonType": "string", "minLength": 1},
		},
	}}
}

func districtSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "regionId", "name", "status", "verificationStatus", "provenance", "datasetVersion"},
		"properties": bson.M{
			"_id":                bson.M{"bsonType": "string", "pattern": ulidPattern},
			"regionId":           bson.M{"bsonType": "string", "pattern": ulidPattern},
			"regionName":         bson.M{"bsonType": []string{"string", "null"}},
			"name":               bson.M{"bsonType": "string", "minLength": 1},
			"normalizedName":     bson.M{"bsonType": "string"},
			"districtType":       bson.M{"bsonType": []string{"string", "null"}},
			"officialCode":       bson.M{"bsonType": []string{"string", "null"}},
			"capital":            bson.M{"bsonType": []string{"string", "null"}},
			"status":             bson.M{"enum": statusEnum},
			"verificationStatus": bson.M{"enum": verificationEnum},
			"centroid":           geoJSONSchema(),
			"geometry":           geoJSONSchema(),
			"provenance":         provenanceSchema(),
			"datasetVersion":     bson.M{"bsonType": "string", "minLength": 1},
		},
	}}
}

func placeSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "name", "type", "status", "verificationStatus", "provenance", "datasetVersion"},
		"properties": bson.M{
			"_id":            bson.M{"bsonType": "string", "pattern": ulidPattern},
			"name":           bson.M{"bsonType": "string", "minLength": 1},
			"normalizedName": bson.M{"bsonType": "string"},
			"type": bson.M{"enum": []string{
				"CITY", "TOWN", "VILLAGE", "COMMUNITY", "SUBURB",
				"NEIGHBOURHOOD", "HAMLET", "SETTLEMENT", "LOCALITY", "REGIONAL_CAPITAL",
			}},
			"regionId":      bson.M{"bsonType": []string{"string", "null"}, "pattern": ulidPattern},
			"regionName":    bson.M{"bsonType": []string{"string", "null"}},
			"districtId":    bson.M{"bsonType": []string{"string", "null"}, "pattern": ulidPattern},
			"districtName":  bson.M{"bsonType": []string{"string", "null"}},
			"parentPlaceId": bson.M{"bsonType": []string{"string", "null"}, "pattern": ulidPattern},
			"aliases": bson.M{
				"bsonType": "array",
				"items": bson.M{
					"bsonType": "object",
					"required": []string{"value"},
					"properties": bson.M{
						"value":           bson.M{"bsonType": "string"},
						"normalizedValue": bson.M{"bsonType": "string"},
						"aliasType":       bson.M{"bsonType": []string{"string", "null"}},
						"language":        bson.M{"bsonType": []string{"string", "null"}},
						"isPreferred":     bson.M{"bsonType": []string{"bool", "null"}},
					},
				},
			},
			"population":         bson.M{"bsonType": []string{"long", "int", "null"}},
			"status":             bson.M{"enum": statusEnum},
			"verificationStatus": bson.M{"enum": verificationEnum},
			"centroid":           geoJSONSchema(),
			"geometry":           geoJSONSchema(),
			"provenance":         provenanceSchema(),
			"datasetVersion":     bson.M{"bsonType": "string", "minLength": 1},
		},
	}}
}

func redirectSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "newId"},
		"properties": bson.M{
			"_id":      bson.M{"bsonType": "string"},
			"newId":    bson.M{"bsonType": "string", "pattern": ulidPattern},
			"reason":   bson.M{"bsonType": []string{"string", "null"}},
			"mergedAt": bson.M{"bsonType": []string{"string", "null"}},
		},
	}}
}

func apiKeySchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "prefix", "secretHash", "class", "environment", "scopes"},
		"properties": bson.M{
			"_id":            bson.M{"bsonType": "string"},
			"applicationId":  bson.M{"bsonType": []string{"string", "null"}},
			"organizationId": bson.M{"bsonType": []string{"string", "null"}},
			"name":           bson.M{"bsonType": []string{"string", "null"}},
			"class":          bson.M{"enum": []string{"BROWSER", "SERVER", "TEST"}},
			"environment":    bson.M{"enum": []string{"live", "test"}},
			"prefix":         bson.M{"bsonType": "string"},
			// The database schema itself records that only a digest is stored.
			"secretHash":     bson.M{"bsonType": "string"},
			"scopes":         bson.M{"bsonType": "array", "items": bson.M{"bsonType": "string"}},
			"allowedOrigins": bson.M{"bsonType": []string{"array", "null"}},
			"allowedIps":     bson.M{"bsonType": []string{"array", "null"}},
			"createdAt":      bson.M{"bsonType": []string{"date", "null"}},
			"expiresAt":      bson.M{"bsonType": []string{"date", "null"}},
			"lastUsedAt":     bson.M{"bsonType": []string{"date", "null"}},
			"revokedAt":      bson.M{"bsonType": []string{"date", "null"}},
			"elevated":       bson.M{"bsonType": []string{"bool", "null"}},
			"elevatedReason": bson.M{"bsonType": []string{"string", "null"}},
		},
	}}
}

func organizationSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "name", "ownerId", "members", "createdAt"},
		"properties": bson.M{
			"_id": bson.M{"bsonType": "string"}, "name": bson.M{"bsonType": "string"},
			"ownerId": bson.M{"bsonType": "string"}, "createdAt": bson.M{"bsonType": "date"},
			"members": bson.M{"bsonType": "array", "items": bson.M{"bsonType": "object", "required": []string{"accountId", "email", "role", "joinedAt"}, "properties": bson.M{"accountId": bson.M{"bsonType": "string"}, "email": bson.M{"bsonType": "string"}, "role": bson.M{"enum": []string{"OWNER", "ADMIN", "MEMBER", "VIEWER"}}, "joinedAt": bson.M{"bsonType": "date"}}}},
		},
	}}
}

func applicationSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "organizationId", "name", "environments", "plan", "createdAt"},
		"properties": bson.M{
			"_id": bson.M{"bsonType": "string"}, "organizationId": bson.M{"bsonType": "string"},
			"name": bson.M{"bsonType": "string"}, "description": bson.M{"bsonType": []string{"string", "null"}},
			"environments": bson.M{"bsonType": "array", "items": bson.M{"enum": []string{"test", "live"}}},
			"domains":      bson.M{"bsonType": []string{"array", "null"}, "items": bson.M{"bsonType": "string"}},
			"callbackUrl":  bson.M{"bsonType": []string{"string", "null"}}, "plan": bson.M{"enum": []string{"free"}},
			"createdAt": bson.M{"bsonType": "date"},
		},
	}}
}

func organizationInvitationSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{"bsonType": "object", "required": []string{"_id", "organizationId", "email", "role", "tokenHash", "status", "invitedBy", "createdAt", "expiresAt"}, "properties": bson.M{
		"_id": bson.M{"bsonType": "string"}, "organizationId": bson.M{"bsonType": "string"}, "email": bson.M{"bsonType": "string"},
		"role": bson.M{"enum": []string{"ADMIN", "MEMBER", "VIEWER"}}, "tokenHash": bson.M{"bsonType": "string"},
		"status": bson.M{"enum": []string{"PENDING", "ACCEPTED", "REVOKED"}}, "invitedBy": bson.M{"bsonType": "string"},
		"createdAt": bson.M{"bsonType": "date"}, "expiresAt": bson.M{"bsonType": "date"}, "acceptedAt": bson.M{"bsonType": []string{"date", "null"}},
	}}}
}

func webauthnChallengeSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "purpose", "session", "expiresAt"},
		"properties": bson.M{
			"_id":       bson.M{"bsonType": "string"},
			"accountId": bson.M{"bsonType": []string{"string", "null"}},
			"purpose":   bson.M{"enum": []string{"registration", "login"}},
			"session":   bson.M{"bsonType": "binData"},
			"expiresAt": bson.M{"bsonType": "date"},
		},
	}}
}

func accountSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "email", "emailVerified", "role", "sessionEpoch"},
		"properties": bson.M{
			"_id":           bson.M{"bsonType": "string"},
			"email":         bson.M{"bsonType": "string"},
			"emailVerified": bson.M{"bsonType": "bool"},
			"passwordHash":  bson.M{"bsonType": []string{"string", "null"}},
			"role": bson.M{"enum": []string{
				"DEVELOPER", "SUPER_ADMIN", "DATA_ADMIN", "DATA_REVIEWER",
				"DATA_CONTRIBUTOR", "DEVELOPER_SUPPORT", "SECURITY_AUDITOR",
			}},
			"disabled":     bson.M{"bsonType": []string{"bool", "null"}},
			"sessionEpoch": bson.M{"bsonType": []string{"int", "long"}},
		},
	}}
}

func sessionSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "accountId", "role", "stage", "tokenHash", "epoch", "expiresAt"},
		"properties": bson.M{
			"_id":       bson.M{"bsonType": "string"},
			"accountId": bson.M{"bsonType": "string"},
			"stage":     bson.M{"enum": []string{"pending_mfa", "authenticated"}},
			// Only the hash is ever stored; a raw token in this collection
			// would turn a database leak into live sessions.
			"tokenHash": bson.M{"bsonType": "string"},
			"epoch":     bson.M{"bsonType": []string{"int", "long"}},
			"expiresAt": bson.M{"bsonType": "date"},
		},
	}}
}

func oneTimeTokenSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "accountId", "purpose", "hash", "expiresAt"},
		"properties": bson.M{
			"_id":       bson.M{"bsonType": "string"},
			"accountId": bson.M{"bsonType": "string"},
			"purpose":   bson.M{"enum": []string{"email_verification", "password_reset", "mfa_recovery"}},
			"hash":      bson.M{"bsonType": "string"},
			"expiresAt": bson.M{"bsonType": "date"},
		},
	}}
}

// auditSchema enforces at the DATABASE that an audit row carries who, what,
// when and to which target. A row missing any of those is not evidence, and
// the validator refuses it rather than storing something unusable.
func auditSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "at", "actorKind", "actorId", "action", "targetKind", "targetId", "outcome", "hash", "prevHash"},
		"properties": bson.M{
			"_id":        bson.M{"bsonType": "string"},
			"at":         bson.M{"bsonType": "date"},
			"actorKind":  bson.M{"enum": []string{"operator", "admin", "developer", "api_key", "system"}},
			"actorId":    bson.M{"bsonType": "string"},
			"actorLabel": bson.M{"bsonType": []string{"string", "null"}},
			"actorIp":    bson.M{"bsonType": []string{"string", "null"}},
			"action":     bson.M{"bsonType": "string"},
			"targetKind": bson.M{"bsonType": "string"},
			"targetId":   bson.M{"bsonType": "string"},
			"outcome":    bson.M{"enum": []string{"succeeded", "failed"}},
			// The tamper-evident chain. Present on every row by construction.
			"hash":     bson.M{"bsonType": "string"},
			"prevHash": bson.M{"bsonType": "string"},
		},
	}}
}

func datasetVersionSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "version", "status"},
		"properties": bson.M{
			"_id":     bson.M{"bsonType": "string"},
			"version": bson.M{"bsonType": "string"},
			"status": bson.M{"enum": []string{
				"draft", "validation", "review", "approved", "published", "rolled_back",
			}},
			"publishedAt": bson.M{"bsonType": []string{"string", "null"}},
			"changelog":   bson.M{"bsonType": []string{"string", "null"}},
			"checksum":    bson.M{"bsonType": []string{"string", "null"}},
			"counts":      bson.M{"bsonType": []string{"object", "null"}},
			// Downloadable files for this version. Each entry describes a file
			// that EXISTS, with a checksum taken from its actual bytes.
			"artifacts": bson.M{
				"bsonType": []string{"array", "null"},
				"items": bson.M{
					"bsonType": "object",
					"required": []string{"entity", "format", "filename", "sizeBytes", "sha256"},
					"properties": bson.M{
						"entity":      bson.M{"bsonType": "string"},
						"format":      bson.M{"enum": []string{"geojson", "csv", "json"}},
						"filename":    bson.M{"bsonType": "string"},
						"sizeBytes":   bson.M{"bsonType": []string{"long", "int"}},
						"sha256":      bson.M{"bsonType": "string"},
						"recordCount": bson.M{"bsonType": []string{"long", "int", "null"}},
					},
				},
			},
		},
	}}
}
