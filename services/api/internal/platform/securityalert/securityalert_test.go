package securityalert

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookReporterSignsAndDeliversEvent(t *testing.T) {
	const secret = "test-signing-secret"
	var received Event
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if got := r.Header.Get("X-GhanaGeo-Signature"); got != want {
			t.Errorf("signature = %q, want %q", got, want)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	reporter := New(server.URL, secret, slog.New(slog.NewTextHandler(io.Discard, nil)))
	reporter.now = func() time.Time { return time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC) }
	if err := reporter.Report(context.Background(), Event{Kind: QuotaSpike, ActorID: "key_public_prefix"}); err != nil {
		t.Fatal(err)
	}
	if received.Kind != QuotaSpike || received.Severity != "warning" || received.OccurredAt.IsZero() {
		t.Fatalf("unexpected event: %+v", received)
	}
}

func TestWebhookReporterReturnsNonSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no", http.StatusBadGateway)
	}))
	defer server.Close()
	if err := New(server.URL, "", nil).Report(context.Background(), Event{Kind: AdminAuthFailed}); err == nil {
		t.Fatal("expected non-success response to fail")
	}
}
