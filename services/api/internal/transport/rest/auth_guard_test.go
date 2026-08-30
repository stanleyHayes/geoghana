package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExpensiveAuthGuardFailsClosedPerIP(t *testing.T) {
	ip := "192.0.2.77:1234"
	h := authExpensiveGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		req.RemoteAddr = ip
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if i < 5 && rec.Code != http.StatusNoContent {
			t.Fatalf("attempt %d = %d", i+1, rec.Code)
		}
		if i == 5 && rec.Code != http.StatusTooManyRequests {
			t.Fatalf("sixth attempt = %d", rec.Code)
		}
	}
}

func TestPrivateResponsePreventsSharedCaching(t *testing.T) {
	h := adminPrivateResponse(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/session", nil))
	if got := rec.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := rec.Header().Values("Vary"); len(got) == 0 {
		t.Fatal("Vary Cookie is missing")
	}
}

func TestAuthRateKeyBucketsIPv6ByPrefix(t *testing.T) {
	a := authRateKey("[2001:db8:abcd:12::1]:443")
	b := authRateKey("2001:db8:abcd:12::ffff")
	if a != b || a != "2001:db8:abcd:12::/64" {
		t.Fatalf("keys = %q, %q", a, b)
	}
	if got := authRateKey("192.0.2.9:443"); got != "192.0.2.9" {
		t.Fatalf("IPv4 key = %q", got)
	}
}
