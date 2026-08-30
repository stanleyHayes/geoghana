package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/fairuse"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/securityalert"
)

type authTestKeys struct{ key *identity.APIKey }

func (k authTestKeys) ByPrefix(context.Context, string) (*identity.APIKey, error) { return k.key, nil }
func (authTestKeys) MarkUsed(context.Context, string, time.Time) error            { return nil }

type authTestLimiter struct{ decision Decision }

func (l authTestLimiter) Allow(context.Context, identity.Identity, identity.Allowance, identity.CostClass) (Decision, error) {
	return l.decision, nil
}

type captureLimiter struct{ allowance identity.Allowance }

func (l *captureLimiter) Allow(_ context.Context, _ identity.Identity, a identity.Allowance, _ identity.CostClass) (Decision, error) {
	l.allowance = a
	return Decision{Allowed: true, Limit: a.BurstUnits}, nil
}

type policyRepo struct{ policy *identity.FairUsePolicy }

func (r *policyRepo) CurrentPolicy(context.Context, time.Time) (*identity.FairUsePolicy, error) {
	return r.policy, nil
}
func (*policyRepo) CurrentOverride(context.Context, string, time.Time) (*identity.FairUseOverride, error) {
	return nil, nil
}

func TestAuthorizeUsesLivePolicyAndKeepsSandboxStricter(t *testing.T) {
	now := time.Now().UTC()
	p := identity.DefaultFairUsePolicy()
	p.Revision = 2
	p.CreatedAt = now.Add(-time.Minute)
	p.EffectiveFrom = p.CreatedAt
	p.Reason = "capacity measurement"
	p.Authenticated.BurstUnits = 900
	p.Sandbox.BurstUnits = 90
	repo := &policyRepo{policy: &p}
	resolver := fairuse.NewResolver(repo, time.Hour)
	limiter := &captureLimiter{}
	a := New(authTestKeys{}, limiter, true).WithFairUseResolver(resolver)
	if _, _, err := a.Authorize(context.Background(), "", "203.0.113.1", "", identity.CostCheap); err != nil {
		t.Fatal(err)
	}
	if limiter.allowance.BurstUnits != 90 {
		t.Fatalf("sandbox burst=%d, want 90", limiter.allowance.BurstUnits)
	}
	p.Sandbox.BurstUnits = 80
	resolver.Invalidate()
	if _, _, err := a.Authorize(context.Background(), "", "203.0.113.1", "", identity.CostCheap); err != nil {
		t.Fatal(err)
	}
	if limiter.allowance.BurstUnits != 80 {
		t.Fatalf("refreshed sandbox burst=%d, want 80", limiter.allowance.BurstUnits)
	}
}

func TestDisabledCostClassPreservesRateLimitHeaders(t *testing.T) {
	now := time.Now().UTC()
	p := identity.DefaultFairUsePolicy()
	p.Revision = 2
	p.CreatedAt = now.Add(-time.Minute)
	p.EffectiveFrom = p.CreatedAt
	p.Reason = "disable expensive geometry"
	p.CostCeilings[identity.CostGeometry] = 0
	a := New(authTestKeys{}, &captureLimiter{}, false).WithFairUseResolver(fairuse.NewResolver(&policyRepo{policy: &p}, time.Hour))
	h := a.Middleware(func(*http.Request) identity.CostClass { return identity.CostGeometry })(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("disabled request reached handler") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/boundaries/x", nil))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get("X-RateLimit-Limit") == "" || rec.Header().Get("X-RateLimit-Remaining") == "" || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("missing compatibility headers: %v", rec.Header())
	}
}

type authTestAlerts struct{ events []securityalert.Event }

func (a *authTestAlerts) Report(_ context.Context, event securityalert.Event) error {
	a.events = append(a.events, event)
	return nil
}

func TestMiddlewareAndRequireScopeRejectUnderScopedKey(t *testing.T) {
	generated, err := identity.Generate(identity.EnvTest)
	if err != nil {
		t.Fatal(err)
	}
	key := &identity.APIKey{
		Prefix: generated.Prefix, SecretHash: generated.SecretHash,
		Scopes: []identity.Scope{identity.ScopeLocationsRead},
	}
	a := New(authTestKeys{key: key}, authTestLimiter{Decision{Allowed: true, Limit: 600}}, false)
	called := false
	handler := a.Middleware(func(*http.Request) identity.CostClass { return identity.CostNormal })(
		RequireScope(identity.ScopeSearchRead)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })),
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=osu", nil)
	req.Header.Set("Authorization", "Bearer "+generated.Full)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if called {
		t.Fatal("under-scoped key reached the handler")
	}
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", res.Code)
	}
}

func TestAnonymousCallerKeepsPublicReadScopes(t *testing.T) {
	a := New(authTestKeys{}, authTestLimiter{Decision{Allowed: true, Limit: 120}}, false)
	called := false
	handler := a.Middleware(func(*http.Request) identity.CostClass { return identity.CostNormal })(
		RequireScope(identity.ScopeSearchRead)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })),
	)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/search?q=osu", nil))
	if !called {
		t.Fatal("deliberately anonymous public read was rejected")
	}
}

func TestResolveRejectsExpiredAndRevokedKeysAtTheTransportBoundary(t *testing.T) {
	generated, err := identity.Generate(identity.EnvLive)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)

	for _, tc := range []struct {
		name string
		key  identity.APIKey
	}{
		{name: "expired", key: identity.APIKey{ExpiresAt: &past}},
		{name: "revoked", key: identity.APIKey{RevokedAt: &past}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := tc.key
			key.Prefix = generated.Prefix
			key.SecretHash = generated.SecretHash
			a := New(authTestKeys{key: &key}, authTestLimiter{}, false)
			a.now = func() time.Time { return now }
			if _, err := a.Resolve(context.Background(), "Bearer "+generated.Full, "203.0.113.4", ""); err == nil {
				t.Fatalf("%s key was accepted", tc.name)
			}
		})
	}
}

func TestDisallowedBrowserOriginRaisesUnusualUsageAlert(t *testing.T) {
	generated, err := identity.Generate(identity.EnvLive)
	if err != nil {
		t.Fatal(err)
	}
	key := &identity.APIKey{
		Prefix: generated.Prefix, SecretHash: generated.SecretHash,
		Class: identity.ClassBrowser, AllowedOrigins: []string{"https://console.example"},
	}
	alerts := &authTestAlerts{}
	a := New(authTestKeys{key: key}, authTestLimiter{}, false).WithSecurityAlerts(alerts)
	_, err = a.Resolve(context.Background(), "Bearer "+generated.Full, "203.0.113.8", "https://unexpected.example")
	if err == nil {
		t.Fatal("expected origin denial")
	}
	if len(alerts.events) != 1 || alerts.events[0].Kind != securityalert.UnusualKeyUsage {
		t.Fatalf("events = %+v", alerts.events)
	}
	if alerts.events[0].ActorID != key.Prefix {
		t.Fatalf("alert actor = %q, want public prefix", alerts.events[0].ActorID)
	}
}

func TestRateLimitDenialRaisesQuotaSpikeAlert(t *testing.T) {
	alerts := &authTestAlerts{}
	a := New(authTestKeys{}, authTestLimiter{Decision{Allowed: false, Limit: 120, RetryAfter: 7}}, false).
		WithSecurityAlerts(alerts)
	_, _, err := a.Authorize(context.Background(), "", "203.0.113.9", "", identity.CostGeometry)
	if err == nil {
		t.Fatal("expected rate-limit denial")
	}
	if len(alerts.events) != 1 || alerts.events[0].Kind != securityalert.QuotaSpike {
		t.Fatalf("events = %+v", alerts.events)
	}
	if alerts.events[0].SourceIP != "203.0.113.9" {
		t.Fatalf("source IP = %q", alerts.events[0].SourceIP)
	}
}

func TestDisallowedKeyIPRaisesUnusualUsageAlert(t *testing.T) {
	generated, err := identity.Generate(identity.EnvLive)
	if err != nil {
		t.Fatal(err)
	}
	key := &identity.APIKey{
		Prefix: generated.Prefix, SecretHash: generated.SecretHash,
		Class: identity.ClassServer, AllowedIPs: []string{"198.51.100.0/24"},
	}
	alerts := &authTestAlerts{}
	a := New(authTestKeys{key: key}, authTestLimiter{}, false).WithSecurityAlerts(alerts)
	_, err = a.Resolve(context.Background(), "Bearer "+generated.Full, "203.0.113.9", "")
	if err == nil {
		t.Fatal("expected IP denial")
	}
	if len(alerts.events) != 1 || alerts.events[0].Kind != securityalert.UnusualKeyUsage {
		t.Fatalf("events = %+v", alerts.events)
	}
	if got := alerts.events[0].Details["reason"]; got != "ip_not_allowed" {
		t.Fatalf("reason = %v", got)
	}
}
