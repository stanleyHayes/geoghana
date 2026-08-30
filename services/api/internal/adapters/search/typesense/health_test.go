package typesense

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeReturnsBoundedCollectionSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/collections/"+Collection {
			t.Fatalf("unexpected probe request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-TYPESENSE-API-KEY") != "secret" {
			t.Fatal("probe did not authenticate upstream")
		}
		_, _ = w.Write([]byte(`{"name":"ghanageo_places","num_documents":42,"fields":[{"name":"private"}]}`))
	}))
	defer server.Close()

	result := New(server.URL, "secret").Probe(context.Background())
	if result.Status != "healthy" || result.DocumentCount == nil || *result.DocumentCount != 42 {
		t.Fatalf("unexpected probe result: %+v", result)
	}
	if result.Detail != "search collection is readable" {
		t.Fatalf("probe leaked or changed detail: %q", result.Detail)
	}
}

func TestProbeSanitizesUpstreamFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "secret backend diagnostic", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	result := New(server.URL, "super-secret-key").Probe(context.Background())
	if result.Status != "unavailable" || result.Detail != "search probe failed" {
		t.Fatalf("unsafe failure result: %+v", result)
	}
}
