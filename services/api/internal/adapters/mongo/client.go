// Package mongo implements the repository ports against MongoDB.
// All BSON mapping lives in this package — the domain has no bson tags.
package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Store owns the client and hands out collections.
type Store struct {
	client *mongo.Client
	db     *mongo.Database
}

func Connect(ctx context.Context, uri, dbName string, monitors ...*event.CommandMonitor) (*Store, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(uri)
	if len(monitors) > 0 && monitors[0] != nil {
		clientOptions.SetMonitor(monitors[0])
	}
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongo: %w", err)
	}
	return &Store{client: client, db: client.Database(dbName)}, nil
}

func (s *Store) Close(ctx context.Context) error { return s.client.Disconnect(ctx) }
func (s *Store) DB() *mongo.Database             { return s.db }
func (s *Store) Client() *mongo.Client           { return s.client }

// Collection names, in one place.
const (
	ColRegions           = "regions"
	ColDistricts         = "districts"
	ColPlaces            = "places"
	ColRedirects         = "place_redirects"
	ColAliases           = "place_aliases"
	ColAdminMutationKeys = "admin_geography_mutation_keys"
	ColSources           = "sources"
	ColSourceRecords     = "source_records"
	ColSourceRuns        = "source_runs"
	ColDatasetVersion    = "dataset_versions"
	ColChangeRequests    = "change_requests"
	ColAuditLog          = "audit_log"
	ColAccounts          = "accounts"
	ColSessions          = "sessions"
	ColOneTimeTokens     = "one_time_tokens"
	ColWebAuthnChal      = "webauthn_challenges"
	ColRoads             = "roads"
	ColPOIs              = "pois"
	ColUsageEvents       = "usage_events"
	ColOutbox            = "outbox"
	ColFairUsePolicies   = "fair_use_policies"
	ColFairUseOverrides  = "fair_use_overrides"
)
