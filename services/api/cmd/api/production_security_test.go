package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
	restapi "github.com/ghanageo/ghanageo/services/api/internal/transport/rest"
)

func TestProductionMailerFailsClosed(t *testing.T) {
	_, err := newAccountMailer(config.Config{Env: "production"})
	if err == nil {
		t.Fatal("production accepted a nil mailer configuration")
	}
	if strings.Contains(err.Error(), "token") {
		t.Fatalf("configuration error unexpectedly contains a token: %v", err)
	}
}

func TestResendMailerDeliversTokenWithoutLoggingResponse(t *testing.T) {
	var body struct {
		From string   `json:"from"`
		To   []string `json:"to"`
		HTML string   `json:"html"`
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer provider-secret" {
			t.Error("missing provider authorization")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer provider.Close()

	mailer, err := newAccountMailer(config.Config{
		Env: "test", ResendAPIKey: "provider-secret", ResendFromEmail: "mail@example.com",
		ResendAPIURL: provider.URL, PortalURL: "https://console.example.com", AllowTestResendEndpoint: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mailer.SendVerification(context.Background(), "person@example.com", "one-time-token"); err != nil {
		t.Fatal(err)
	}
	if body.From != "mail@example.com" || len(body.To) != 1 || body.To[0] != "person@example.com" {
		t.Fatalf("unexpected provider payload: %#v", body)
	}
	if !strings.Contains(body.HTML, "https://console.example.com/verify-email?token=one-time-token") {
		t.Fatalf("verification link missing from provider payload: %s", body.HTML)
	}
	if err := mailer.SendPasswordReset(context.Background(), "person@example.com", "reset-token"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body.HTML, "https://console.example.com/reset-password?token=reset-token") {
		t.Fatalf("password reset link missing from provider payload: %s", body.HTML)
	}
}

func TestProductionMailerRejectsUnsafeEndpointsWithoutLeakingCredentials(t *testing.T) {
	const credential = "super-secret-provider-credential"
	base := config.Config{
		Env: "production", ResendAPIKey: credential, ResendFromEmail: "mail@example.com",
		PortalURL: "https://console-geo.digitalghana.dev",
	}
	for _, tc := range []struct {
		name, endpoint string
	}{
		{"plain HTTP", "http://api.resend.com/emails"},
		{"unexpected host", "https://resend.attacker.example/emails"},
		{"scheme only", "https://"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			cfg.ResendAPIURL = tc.endpoint
			_, err := newAccountMailer(cfg)
			if err == nil {
				t.Fatal("unsafe provider endpoint was accepted")
			}
			if strings.Contains(err.Error(), credential) {
				t.Fatal("provider credential leaked in validation error")
			}
		})
	}
}

func TestProductionMailerRejectsUnsafePortalURLsWithoutLeakingCredentials(t *testing.T) {
	const credential = "super-secret-provider-credential"
	for _, portal := range []string{"http://console-geo.digitalghana.dev", "https://", "https://user:pass@console-geo.digitalghana.dev"} {
		_, err := newAccountMailer(config.Config{
			Env: "production", ResendAPIKey: credential, ResendFromEmail: "mail@example.com",
			ResendAPIURL: productionResendEndpoint, PortalURL: portal,
		})
		if err == nil {
			t.Fatalf("unsafe portal URL %q was accepted", portal)
		}
		if strings.Contains(err.Error(), credential) {
			t.Fatal("provider credential leaked in portal validation error")
		}
	}
}

func TestProductionPasskeyConfigurationFailsClosed(t *testing.T) {
	valid := config.Config{
		Env: "production", PasskeyRPID: "digitalghana.dev",
		PasskeyOrigins: []string{"https://console-geo.digitalghana.dev", "https://admin-geo.digitalghana.dev"},
	}
	if err := validateProductionIdentityConfig(valid); err != nil {
		t.Fatalf("valid production identity config rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*config.Config)
	}{
		{"localhost RPID", func(c *config.Config) { c.PasskeyRPID = "localhost" }},
		{"missing origins", func(c *config.Config) { c.PasskeyOrigins = nil }},
		{"HTTP origin", func(c *config.Config) { c.PasskeyOrigins = []string{"http://console-geo.digitalghana.dev"} }},
		{"malformed origin", func(c *config.Config) { c.PasskeyOrigins = []string{"https://"} }},
		{"RPID mismatch", func(c *config.Config) { c.PasskeyOrigins = []string{"https://console.attacker.example"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			cfg.PasskeyOrigins = append([]string(nil), valid.PasskeyOrigins...)
			tc.edit(&cfg)
			if err := validateProductionIdentityConfig(cfg); err == nil {
				t.Fatal("unsafe production passkey configuration was accepted")
			}
		})
	}
}

func TestProxySecurityRejectsInsecureTrafficAndPreservesHealth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := mustProxySecurity(t, next, config.Config{
		RequireHTTPS: true, TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"192.0.2.0/24"},
	})

	insecure := httptest.NewRecorder()
	handler.ServeHTTP(insecure, httptest.NewRequest(http.MethodGet, "/v1/regions", nil))
	if insecure.Code != http.StatusUpgradeRequired {
		t.Fatalf("insecure request status = %d, want %d", insecure.Code, http.StatusUpgradeRequired)
	}

	secureReq := httptest.NewRequest(http.MethodGet, "/v1/regions", nil)
	secureReq.Header.Set("X-Forwarded-Proto", "https")
	secure := httptest.NewRecorder()
	handler.ServeHTTP(secure, secureReq)
	if secure.Code != http.StatusNoContent {
		t.Fatalf("trusted HTTPS proxy request status = %d, want %d", secure.Code, http.StatusNoContent)
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	if health.Code != http.StatusNoContent {
		t.Fatalf("plain internal health request status = %d, want %d", health.Code, http.StatusNoContent)
	}
}

func TestProxySecurityStripsUntrustedForwardingHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, name := range []string{"CF-Connecting-IP", "Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Real-IP"} {
			if r.Header.Get(name) != "" {
				t.Errorf("%s reached the application from an untrusted source", name)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := mustProxySecurity(t, next, config.Config{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("CF-Connecting-IP", "203.0.113.11")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestProxySecurityUsesOnlyRenderEdgeClientIP(t *testing.T) {
	var remoteIP string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteIP, _, _ = net.SplitHostPort(r.RemoteAddr)
		if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("CF-Connecting-IP") != "" {
			t.Error("forwarding metadata reached application middleware")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := mustProxySecurity(t, next, config.Config{TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"10.0.0.0/8"}})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.8:4321"
	req.Header.Set("X-Forwarded-For", "198.51.100.99, 203.0.113.7")
	req.Header.Set("X-Real-IP", "198.51.100.98")
	req.Header.Set("CF-Connecting-IP", "203.0.113.7")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if remoteIP != "203.0.113.7" {
		t.Fatalf("normalized client IP = %q, want edge-owned address", remoteIP)
	}
}

func TestProxySecurityRejectsMalformedEdgeClientIP(t *testing.T) {
	var remote string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remote = r.RemoteAddr
		w.WriteHeader(http.StatusNoContent)
	})
	handler := mustProxySecurity(t, next, config.Config{TrustProxyHeaders: true, TrustedProxyCIDRs: []string{"10.0.0.0/8"}})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.8:4321"
	req.Header.Set("CF-Connecting-IP", "198.51.100.1, 203.0.113.7")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if remote != "10.0.0.8:4321" {
		t.Fatalf("malformed edge address replaced safe socket address: %q", remote)
	}
}

func TestProxySecurityRequiresExplicitTrustedPeerCIDRs(t *testing.T) {
	_, err := proxySecurity(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), config.Config{TrustProxyHeaders: true})
	if err == nil {
		t.Fatal("trusted proxy mode started without API_TRUSTED_PROXY_CIDRS")
	}
}

func TestProductionCookieFlowsRemainSecureBehindTrustedEdge(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !restapi.SecureRequest(r) {
			t.Errorf("%s lost its effective HTTPS marker", r.URL.Path)
		}
		http.SetCookie(w, &http.Cookie{
			Name: "gg_session", Value: "test", Path: "/", HttpOnly: true,
			Secure: restapi.SecureRequest(r), Expires: time.Now().Add(time.Hour),
		})
		w.WriteHeader(http.StatusNoContent)
	})
	handler := mustProxySecurity(t, next, config.Config{
		TrustProxyHeaders: true, RequireHTTPS: true, TrustedProxyCIDRs: []string{"10.0.0.0/8"},
	})
	for _, path := range []string{"/v1/auth/login", "/v1/auth/session", "/v1/auth/logout"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			req.RemoteAddr = "10.1.2.3:1234"
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("CF-Connecting-IP", "203.0.113.8")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			cookies := rec.Result().Cookies()
			if len(cookies) != 1 || !cookies[0].Secure {
				t.Fatalf("Set-Cookie = %q; want Secure session cookie", rec.Header().Get("Set-Cookie"))
			}
		})
	}
}

func TestUntrustedPeerCannotAssertHTTPSOrClientIP(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("untrusted insecure request reached application")
	})
	handler := mustProxySecurity(t, next, config.Config{
		TrustProxyHeaders: true, RequireHTTPS: true, TrustedProxyCIDRs: []string{"10.0.0.0/8"},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil)
	req.RemoteAddr = "192.0.2.44:1234"
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("CF-Connecting-IP", "203.0.113.8")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUpgradeRequired {
		t.Fatalf("untrusted peer status = %d, want %d", rec.Code, http.StatusUpgradeRequired)
	}
}

func mustProxySecurity(t *testing.T, next http.Handler, cfg config.Config) http.Handler {
	t.Helper()
	handler, err := proxySecurity(next, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}
