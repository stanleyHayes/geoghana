package mongo

import (
	"context"
	"strings"
	"testing"
	"time"

	app "github.com/ghanageo/ghanageo/services/api/internal/app/adminidentity"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestAdminIdentityGlobalSearchRedactsSecretsAndPaginates(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewAdminIdentityRepo(store)
	now := time.Now().UTC()
	for i, id := range []string{"org-a", "org-b", "org-c"} {
		org := identity.Organization{ID: id, Name: "Civic " + id, OwnerID: "acc-owner", Members: []identity.OrganizationMember{{AccountID: "acc-owner", Email: "owner@example.test", Role: identity.OrganizationOwner, JoinedAt: now}}, CreatedAt: now.Add(time.Duration(i) * time.Second)}
		if err := NewOrganizationRepo(store).Create(ctx, org); err != nil {
			t.Fatal(err)
		}
	}
	p1, err := repo.Organizations(ctx, identity.AdminListFilter{Query: "civic", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(p1.Data) != 2 || p1.NextCursor == "" || p1.Total != 3 {
		t.Fatalf("page 1 = %+v", p1)
	}
	p2, err := repo.Organizations(ctx, identity.AdminListFilter{Query: "civic", Limit: 2, Cursor: p1.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Data) != 1 || p2.Data[0].ID == p1.Data[0].ID || p2.Data[0].ID == p1.Data[1].ID {
		t.Fatalf("page 2 = %+v", p2)
	}
	key := identity.APIKey{ID: "key-admin", ApplicationID: "app-admin", OrganizationID: "org-a", Name: "Production", Class: identity.ClassServer, Environment: identity.EnvLive, Prefix: "gh_live_public", SecretHash: "TOP_SECRET_DIGEST", Scopes: []identity.Scope{identity.ScopeSearchRead}, CreatedAt: now}
	if _, err := store.db.Collection(ColAPIKeys).InsertOne(ctx, toKeyDoc(key)); err != nil {
		t.Fatal(err)
	}
	keys, err := repo.Keys(ctx, identity.AdminListFilter{Query: "production"})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys.Data) != 1 || keys.Data[0].Prefix != "gh_live_public" {
		t.Fatalf("keys = %+v", keys)
	}
	encoded, err := bson.MarshalExtJSON(keys.Data[0], false, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "TOP_SECRET_DIGEST") || strings.Contains(strings.ToLower(string(encoded)), "secrethash") {
		t.Fatalf("secret leaked: %s", encoded)
	}
	if _, err := repo.Keys(ctx, identity.AdminListFilter{Cursor: "not base64!"}); err != identity.ErrInvalidCursor {
		t.Fatalf("invalid cursor error = %v", err)
	}
}

func TestAdminKeyMutationAndAuditCommitAtomically(t *testing.T) {
	store := sessionTestStore(t)
	ctx := context.Background()
	if err := Migrate(ctx, store.DB()); err != nil {
		t.Fatal(err)
	}
	repo := NewAdminIdentityRepo(store)
	now := time.Now().UTC()
	key := identity.APIKey{ID: "key-atomic", ApplicationID: "app", OrganizationID: "org", Name: "Live", Class: identity.ClassServer, Environment: identity.EnvLive, Prefix: "gh_live_atomic", SecretHash: "digest", Scopes: []identity.Scope{identity.ScopeSearchRead}, CreatedAt: now}
	if _, err := store.db.Collection(ColAPIKeys).InsertOne(ctx, toKeyDoc(key)); err != nil {
		t.Fatal(err)
	}
	e, err := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: "support"}, audit.ActionKeySuspended, audit.Target{Kind: "api_key", ID: key.ID})
	if err != nil {
		t.Fatal(err)
	}
	e.Reason = "credential shared publicly"
	if err := repo.SetKeyState(ctx, key.ID, app.KeyMutation{State: "suspended", At: now, Reason: e.Reason}, e); err != nil {
		t.Fatal(err)
	}
	var stored apiKeyDoc
	if err := store.db.Collection(ColAPIKeys).FindOne(ctx, bson.M{"_id": key.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.SuspendedAt == nil || stored.SuspendedReason != e.Reason {
		t.Fatalf("key = %+v", stored)
	}
	if count, err := store.db.Collection(ColAuditLog).CountDocuments(ctx, bson.M{"targetId": key.ID, "action": string(audit.ActionKeySuspended), "reason": e.Reason}); err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
	bad, _ := audit.New(audit.Actor{Kind: audit.ActorAdmin, ID: "support"}, audit.ActionKeyRevoked, audit.Target{Kind: "api_key", ID: "missing"})
	if err := repo.SetKeyState(ctx, "missing", app.KeyMutation{State: "revoked", At: now, Reason: "test"}, bad); err != identity.ErrNotFound {
		t.Fatalf("missing mutation = %v", err)
	}
	if count, _ := store.db.Collection(ColAuditLog).CountDocuments(ctx, bson.M{"targetId": "missing"}); count != 0 {
		t.Fatal("failed mutation left an audit row")
	}
}
