package mongo

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
		"required": []string{"sourceId"},
		"properties": bson.M{
			"sourceId":          bson.M{"bsonType": "string"},
			"externalId":        bson.M{"bsonType": []string{"string", "null"}},
			"sourceUrl":         bson.M{"bsonType": []string{"string", "null"}},
			"retrievedAt":       bson.M{"bsonType": []string{"string", "null"}},
			"sourcePayloadHash": bson.M{"bsonType": []string{"string", "null"}},
			"notes":             bson.M{"bsonType": []string{"string", "null"}},
		},
	}
}

var statusEnum = []string{"ACTIVE", "DEPRECATED", "MERGED"}
var verificationEnum = []string{
	"REFERENCE", "SEED_NEEDS_CANONICAL_RECONCILIATION", "REVIEWED", "CANONICAL",
}

func regionSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "name", "status", "verificationStatus", "provenance"},
		"properties": bson.M{
			"_id":                bson.M{"bsonType": "string"},
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
			"datasetVersion":     bson.M{"bsonType": []string{"string", "null"}},
		},
	}}
}

func districtSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "regionId", "name", "status", "verificationStatus", "provenance"},
		"properties": bson.M{
			"_id":                bson.M{"bsonType": "string"},
			"regionId":           bson.M{"bsonType": "string", "minLength": 1},
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
			"datasetVersion":     bson.M{"bsonType": []string{"string", "null"}},
		},
	}}
}

func placeSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "name", "type", "status", "verificationStatus", "provenance"},
		"properties": bson.M{
			"_id":            bson.M{"bsonType": "string"},
			"name":           bson.M{"bsonType": "string", "minLength": 1},
			"normalizedName": bson.M{"bsonType": "string"},
			"type": bson.M{"enum": []string{
				"CITY", "TOWN", "VILLAGE", "COMMUNITY", "SUBURB",
				"NEIGHBOURHOOD", "HAMLET", "SETTLEMENT", "LOCALITY", "REGIONAL_CAPITAL",
			}},
			"regionId":      bson.M{"bsonType": []string{"string", "null"}},
			"regionName":    bson.M{"bsonType": []string{"string", "null"}},
			"districtId":    bson.M{"bsonType": []string{"string", "null"}},
			"districtName":  bson.M{"bsonType": []string{"string", "null"}},
			"parentPlaceId": bson.M{"bsonType": []string{"string", "null"}},
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
			"datasetVersion":     bson.M{"bsonType": []string{"string", "null"}},
		},
	}}
}

func redirectSchema() bson.M {
	return bson.M{"$jsonSchema": bson.M{
		"bsonType": "object",
		"required": []string{"_id", "newId"},
		"properties": bson.M{
			"_id":      bson.M{"bsonType": "string"},
			"newId":    bson.M{"bsonType": "string"},
			"reason":   bson.M{"bsonType": []string{"string", "null"}},
			"mergedAt": bson.M{"bsonType": []string{"string", "null"}},
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
		},
	}}
}
