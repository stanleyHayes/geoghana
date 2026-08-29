package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/securityalert"
)

type authTestKeys struct{ key *identity.APIKey }

func (k authTestKeys) ByPrefix(context.Context, string) (*identity.APIKey, error) { return k.key, nil }
func (authTestKeys) MarkUsed(context.Context, string, time.Time) error            { return nil }

type authTestLimiter struct{ decision Decision }

func (l authTestLimiter) Allow(context.Context, identity.Identity, identity.Allowance, identity.CostClass) (Decision, error) {
	return l.decision, nil
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
