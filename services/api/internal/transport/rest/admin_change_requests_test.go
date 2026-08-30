package rest

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/changerequest"
	"github.com/go-chi/chi/v5"
)

func TestChangeRequestDTOIsPrivateAndStable(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	cr := domain.ChangeRequest{
		ID: "cr_01KDVDNA00N6BFFK8VF5K8YXPW", Target: domain.Target{Kind: "place", ID: "place-1"},
		ProposedPatch: map[string]any{"name": "Accra"}, Snapshot: domain.Snapshot{BeforeJSON: `{"name":"Akra"}`, AfterJSON: `{"name":"Accra"}`, Digest: strings.Repeat("a", 64)},
		Evidence: domain.Evidence{SourceID: "gss"}, SubmitterID: "acct-1", State: domain.StateSubmitted, Version: 1, CreatedAt: now, UpdatedAt: now,
		Comments: []domain.Comment{{ID: "comment-1", AuthorID: "acct-2", Body: "Checked", RequestID: "secret-replay-key", CreatedAt: now}},
		History:  []domain.HistoryEvent{{ID: "history-1", To: domain.StateSubmitted, ActorID: "acct-1", RequestID: "secret-replay-key", CreatedAt: now}},
	}
	b, err := json.Marshal(changeRequestOut(cr))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"target":{"kind":"place","id":"place-1"}`, `"before":{"name":"Akra"}`, `"after":{"name":"Accra"}`, `"comments":[`, `"history":[`, `"revisions":[]`} {
		if !strings.Contains(got, want) {
			t.Errorf("response missing %s: %s", want, got)
		}
	}
	if strings.Contains(got, "secret-replay-key") || strings.Contains(got, "requestId") {
		t.Fatalf("idempotency key leaked in response: %s", got)
	}
}

func TestChangeRequestRoutesAreRegistered(t *testing.T) {
	h := New(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	want := map[string]bool{
		"GET /v1/admin/change-requests":                   false,
		"POST /v1/admin/change-requests":                  false,
		"GET /v1/admin/change-requests/{id}":              false,
		"POST /v1/admin/change-requests/{id}/transitions": false,
		"POST /v1/admin/change-requests/{id}/revisions":   false,
		"POST /v1/admin/change-requests/{id}/comments":    false,
	}
	err := chi.Walk(h.Routes().(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if _, ok := want[key]; ok {
			want[key] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for route, found := range want {
		if !found {
			t.Errorf("route not registered: %s", route)
		}
	}
}

func TestIdempotencyKeysAreActorScopedAndOpaque(t *testing.T) {
	a := idempotencyRequestID("actor-a", "common-key")
	b := idempotencyRequestID("actor-b", "common-key")
	if a == b {
		t.Fatal("idempotency key must be scoped to its authenticated actor")
	}
	if strings.Contains(a, "actor-a") || strings.Contains(a, "common-key") {
		t.Fatalf("raw idempotency material leaked: %s", a)
	}
	if a != idempotencyRequestID("actor-a", "common-key") {
		t.Fatal("idempotency digest is not deterministic")
	}
}
