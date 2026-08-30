package adminidentity

import (
	"context"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type memoryRepo struct {
	mutation *KeyMutation
	evidence *audit.Entry
}

func (m *memoryRepo) Organizations(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.Organization], error) {
	return identity.AdminPage[identity.Organization]{}, nil
}
func (m *memoryRepo) Accounts(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.DeveloperAccount], error) {
	return identity.AdminPage[identity.DeveloperAccount]{}, nil
}
func (m *memoryRepo) Applications(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.Application], error) {
	return identity.AdminPage[identity.Application]{}, nil
}
func (m *memoryRepo) Keys(context.Context, identity.AdminListFilter) (identity.AdminPage[identity.AdminKey], error) {
	return identity.AdminPage[identity.AdminKey]{}, nil
}
func (m *memoryRepo) UsageSummary(context.Context, string, string, time.Time) (usage.Summary, error) {
	return usage.Summary{}, nil
}
func (m *memoryRepo) RequestLog(context.Context, string, string, *time.Time, int) (usage.Page, error) {
	return usage.Page{}, nil
}
func (m *memoryRepo) SetKeyState(_ context.Context, _ string, v KeyMutation, e audit.Entry) error {
	m.mutation = &v
	m.evidence = &e
	return nil
}

func TestDeveloperSupportCanInspectAndSuspendButCannotEditCanonicalData(t *testing.T) {
	r := &memoryRepo{}
	s := NewService(r)
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	actor := account.Account{ID: "support-1", Email: "support@example.test", Role: account.RoleDeveloperSupport}
	if _, err := s.Organizations(context.Background(), actor, identity.AdminListFilter{}); err != nil {
		t.Fatal(err)
	}
	if err := s.SuspendKey(context.Background(), actor, "key-1", "abuse investigation", "req-1", "203.0.113.4"); err != nil {
		t.Fatal(err)
	}
	if r.mutation == nil || r.mutation.State != "suspended" || r.mutation.Reason != "abuse investigation" {
		t.Fatalf("mutation = %+v", r.mutation)
	}
	if r.evidence == nil || r.evidence.Actor.Kind != audit.ActorAdmin || r.evidence.Reason != "abuse investigation" || r.evidence.RequestID != "req-1" {
		t.Fatalf("evidence = %+v", r.evidence)
	}
	if account.Can(actor.Role, account.PermEditGeography) || account.Can(actor.Role, account.PermEditGeometry) {
		t.Fatal("developer support gained canonical edit permission")
	}
}

func TestAdminIdentityDeniesUnrelatedRolesAndRequiresMutationReason(t *testing.T) {
	s := NewService(&memoryRepo{})
	developer := account.Account{ID: "dev", Role: account.RoleDeveloper}
	_, err := s.Keys(context.Background(), developer, identity.AdminListFilter{})
	assertCode(t, err, apierr.PermissionDenied)
	support := account.Account{ID: "support", Role: account.RoleDeveloperSupport}
	err = s.RevokeKey(context.Background(), support, "key-1", "", "", " ")
	assertCode(t, err, apierr.InvalidArgument)
}

func assertCode(t *testing.T, err error, want apierr.Code) {
	t.Helper()
	e, ok := err.(*apierr.Error)
	if !ok || e.Code != want {
		t.Fatalf("error = %v, want %s", err, want)
	}
}
