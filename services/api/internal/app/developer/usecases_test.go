package developer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type orgMemory struct {
	items map[string]identity.Organization
}

func (r *orgMemory) Create(_ context.Context, v identity.Organization) error {
	r.items[v.ID] = v
	return nil
}
func (r *orgMemory) ByIDForOwner(_ context.Context, id, owner string) (*identity.Organization, error) {
	v, ok := r.items[id]
	if !ok || v.OwnerID != owner {
		return nil, identity.ErrNotFound
	}
	return &v, nil
}
func (r *orgMemory) ListByOwner(_ context.Context, owner string) ([]identity.Organization, error) {
	var out []identity.Organization
	for _, v := range r.items {
		if v.OwnerID == owner {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *orgMemory) ByIDForAccount(_ context.Context, id, account string) (*identity.Organization, error) {
	v, ok := r.items[id]
	if !ok {
		return nil, identity.ErrNotFound
	}
	for _, m := range v.Members {
		if m.AccountID == account {
			return &v, nil
		}
	}
	return nil, identity.ErrNotFound
}
func (r *orgMemory) ListByAccount(_ context.Context, account string) ([]identity.Organization, error) {
	var out []identity.Organization
	for _, v := range r.items {
		for _, m := range v.Members {
			if m.AccountID == account {
				out = append(out, v)
				break
			}
		}
	}
	return out, nil
}
func (r *orgMemory) AddMember(_ context.Context, id string, m identity.OrganizationMember) error {
	v, ok := r.items[id]
	if !ok {
		return identity.ErrNotFound
	}
	for _, have := range v.Members {
		if have.AccountID == m.AccountID {
			return identity.ErrNotFound
		}
	}
	v.Members = append(v.Members, m)
	r.items[id] = v
	return nil
}
func (r *orgMemory) TransferOwnership(_ context.Context, id, current, next string, _ time.Time) error {
	v, ok := r.items[id]
	if !ok || v.OwnerID != current {
		return identity.ErrNotFound
	}
	found := false
	for i, m := range v.Members {
		if m.AccountID == next {
			v.Members[i].Role = identity.OrganizationOwner
			found = true
		}
		if m.AccountID == current {
			v.Members[i].Role = identity.OrganizationAdmin
		}
	}
	if !found {
		return identity.ErrNotFound
	}
	v.OwnerID = next
	r.items[id] = v
	return nil
}

type appMemory struct {
	items map[string]identity.Application
}

func (r *appMemory) Create(_ context.Context, v identity.Application) error {
	r.items[v.ID] = v
	return nil
}
func (r *appMemory) ByID(_ context.Context, id, org string) (*identity.Application, error) {
	v, ok := r.items[id]
	if !ok || v.OrganizationID != org {
		return nil, identity.ErrNotFound
	}
	return &v, nil
}
func (r *appMemory) ListByOrganization(_ context.Context, org string) ([]identity.Application, error) {
	var out []identity.Application
	for _, v := range r.items {
		if v.OrganizationID == org {
			out = append(out, v)
		}
	}
	return out, nil
}

type keyMemory struct{ items map[string]identity.APIKey }

func (r *keyMemory) Create(_ context.Context, v identity.APIKey) error { r.items[v.ID] = v; return nil }
func (r *keyMemory) Revoke(_ context.Context, prefix string, at time.Time) error {
	for id, v := range r.items {
		if v.Prefix == prefix {
			v.RevokedAt = &at
			r.items[id] = v
			return nil
		}
	}
	return identity.ErrNotFound
}
func (r *keyMemory) ByIDForApplication(_ context.Context, id, app string) (*identity.APIKey, error) {
	v, ok := r.items[id]
	if !ok || v.ApplicationID != app {
		return nil, identity.ErrNotFound
	}
	return &v, nil
}
func (r *keyMemory) ListByApplication(_ context.Context, app string) ([]identity.APIKey, error) {
	var out []identity.APIKey
	for _, v := range r.items {
		if v.ApplicationID == app {
			out = append(out, v)
		}
	}
	return out, nil
}

type auditMemory struct{ entries []audit.Entry }

func (r *auditMemory) Append(_ context.Context, e audit.Entry) (audit.Entry, error) {
	r.entries = append(r.entries, e)
	return e, nil
}

type invitationMemory struct {
	items map[string]identity.OrganizationInvitation
}

func (r *invitationMemory) Create(_ context.Context, v identity.OrganizationInvitation) error {
	r.items[v.ID] = v
	return nil
}
func (r *invitationMemory) ListPending(_ context.Context, org string) ([]identity.OrganizationInvitation, error) {
	var out []identity.OrganizationInvitation
	for _, v := range r.items {
		if v.OrganizationID == org && v.Status == identity.InvitationPending {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *invitationMemory) ByTokenHash(_ context.Context, hash string) (*identity.OrganizationInvitation, error) {
	for _, v := range r.items {
		if v.TokenHash == hash && v.Status == identity.InvitationPending {
			return &v, nil
		}
	}
	return nil, identity.ErrNotFound
}
func (r *invitationMemory) Accept(_ context.Context, id string, at time.Time) error {
	v, ok := r.items[id]
	if !ok {
		return identity.ErrNotFound
	}
	v.Status = identity.InvitationAccepted
	v.AcceptedAt = &at
	r.items[id] = v
	return nil
}
func (r *invitationMemory) Revoke(_ context.Context, id, org string) error {
	v, ok := r.items[id]
	if !ok || v.OrganizationID != org {
		return identity.ErrNotFound
	}
	v.Status = identity.InvitationRevoked
	r.items[id] = v
	return nil
}

func fixture() (*Service, *orgMemory, *appMemory, *keyMemory, *auditMemory) {
	orgs := &orgMemory{items: map[string]identity.Organization{}}
	apps := &appMemory{items: map[string]identity.Application{}}
	keys := &keyMemory{items: map[string]identity.APIKey{}}
	audits := &auditMemory{}
	invites := &invitationMemory{items: map[string]identity.OrganizationInvitation{}}
	s := NewService(orgs, apps, keys, invites, audits)
	s.now = func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	return s, orgs, apps, keys, audits
}

func TestDeveloperHierarchyAndOneTimeSecret(t *testing.T) {
	s, _, _, keys, audits := fixture()
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acc_owner", "owner@example.test", "Open Maps", "req_1", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	app, err := s.CreateApplication(ctx, "acc_owner", org.ID, CreateApplicationInput{Name: "Field app", Description: "Survey workflow"}, "req_2", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.CreateKey(ctx, "acc_owner", org.ID, app.ID, CreateKeyInput{Name: "browser", Class: identity.ClassBrowser, Environment: identity.EnvLive, Scopes: []identity.Scope{identity.ScopeLocationsRead}, AllowedOrigins: []string{"https://field.example"}}, "req_3", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	if created.Secret == "" {
		t.Fatal("complete key was not returned at creation")
	}
	stored := keys.items[created.Key.ID]
	if stored.SecretHash == "" {
		t.Fatal("key digest was not stored")
	}
	if stored.SecretHash == created.Secret {
		t.Fatal("raw key was stored")
	}
	if err := identity.VerifySecret(created.Secret[len(created.Key.Prefix)+1:], stored.SecretHash); err != nil {
		t.Fatalf("stored digest does not verify: %v", err)
	}
	listed, err := s.Keys(ctx, "acc_owner", org.ID, app.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Fatalf("keys=%d", len(listed))
	}
	if len(audits.entries) != 3 {
		t.Fatalf("audit entries=%d, want 3", len(audits.entries))
	}
	for _, e := range audits.entries {
		if e.AfterJSON == created.Secret || e.BeforeJSON == created.Secret {
			t.Fatal("secret reached audit payload")
		}
	}
}

func TestOwnershipIsolationAndRotationRevokesPreviousKey(t *testing.T) {
	s, _, _, keys, _ := fixture()
	ctx := context.Background()
	org, _ := s.CreateOrganization(ctx, "acc_owner", "owner@example.test", "Owner org", "", "1.1.1.1")
	app, _ := s.CreateApplication(ctx, "acc_owner", org.ID, CreateApplicationInput{Name: "App"}, "", "1.1.1.1")
	created, _ := s.CreateKey(ctx, "acc_owner", org.ID, app.ID, CreateKeyInput{Name: "server", Class: identity.ClassServer, Environment: identity.EnvLive, Scopes: []identity.Scope{identity.ScopeSearchRead}}, "", "1.1.1.1")
	if _, err := s.Keys(ctx, "acc_other", org.ID, app.ID); apierr.From(err).Code != apierr.NotFound {
		t.Fatalf("cross-owner access error=%v", err)
	}
	replacement, err := s.RotateKey(ctx, "acc_owner", org.ID, app.ID, created.Key.ID, "req_rotate", "1.1.1.1")
	if err != nil {
		t.Fatal(err)
	}
	if replacement.Secret == created.Secret {
		t.Fatal("rotation reused the old secret")
	}
	if keys.items[created.Key.ID].RevokedAt == nil {
		t.Fatal("previous key remains active")
	}
	if keys.items[replacement.Key.ID].RevokedAt != nil {
		t.Fatal("replacement key is revoked")
	}
	if err := s.RevokeKey(ctx, "acc_owner", org.ID, app.ID, replacement.Key.ID, "req_revoke", "1.1.1.1"); err != nil {
		t.Fatal(err)
	}
	if keys.items[replacement.Key.ID].RevokedAt == nil {
		t.Fatal("replacement was not revoked")
	}
}

func TestUnsafeBrowserKeyIsRejected(t *testing.T) {
	s, _, _, _, _ := fixture()
	ctx := context.Background()
	org, _ := s.CreateOrganization(ctx, "acc", "account@example.test", "Org", "", "1.1.1.1")
	app, _ := s.CreateApplication(ctx, "acc", org.ID, CreateApplicationInput{Name: "App"}, "", "1.1.1.1")
	_, err := s.CreateKey(ctx, "acc", org.ID, app.ID, CreateKeyInput{Name: "unsafe", Class: identity.ClassBrowser, Environment: identity.EnvLive, Scopes: []identity.Scope{identity.ScopeGRPCAccess}, AllowedOrigins: []string{"https://example"}}, "", "1.1.1.1")
	if err == nil || !errors.Is(err, identity.ErrUnsafeBrowserKey) || apierr.From(err).Code != apierr.InvalidArgument {
		t.Fatalf("unsafe key error=%v", err)
	}
}

func TestInvitationAcceptanceRolesAndOwnershipTransfer(t *testing.T) {
	s, orgs, _, _, audits := fixture()
	ctx := context.Background()
	org, err := s.CreateOrganization(ctx, "acc_owner", "owner@example.test", "Civic Maps", "", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	created, err := s.InviteMember(ctx, "acc_owner", org.ID, "member@example.test", identity.OrganizationAdmin, "req_invite", "203.0.113.1")
	if err != nil {
		t.Fatal(err)
	}
	if created.Token == "" || created.Invitation.TokenHash == created.Token {
		t.Fatal("invitation token was not one-time hashed")
	}
	if _, err := s.AcceptInvitation(ctx, "acc_other", "wrong@example.test", created.Token, "", "203.0.113.2"); apierr.From(err).Code != apierr.PermissionDenied {
		t.Fatalf("wrong email accepted: %v", err)
	}
	joined, err := s.AcceptInvitation(ctx, "acc_member", "member@example.test", created.Token, "req_join", "203.0.113.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(joined.Members) != 2 {
		t.Fatalf("members=%d", len(joined.Members))
	}
	if err := s.TransferOwnership(ctx, "acc_owner", org.ID, "acc_member", "req_transfer", "203.0.113.1"); err != nil {
		t.Fatal(err)
	}
	got := orgs.items[org.ID]
	if got.OwnerID != "acc_member" {
		t.Fatalf("owner=%s", got.OwnerID)
	}
	if err := s.TransferOwnership(ctx, "acc_owner", org.ID, "acc_owner", "", "203.0.113.1"); apierr.From(err).Code != apierr.PermissionDenied {
		t.Fatalf("former owner retained owner-only control: %v", err)
	}
	if len(audits.entries) < 4 {
		t.Fatalf("audit entries=%d", len(audits.entries))
	}
}
