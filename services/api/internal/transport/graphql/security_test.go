package graphql

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func graphqlRequest(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	body := []byte(`{"query":` + strconvQuote(query) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewHandler(nil, nil).ServeHTTP(rec, req)
	return rec
}

func strconvQuote(value string) string {
	var out bytes.Buffer
	_, _ = io.WriteString(&out, `"`)
	for _, r := range value {
		switch r {
		case '\\', '"':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\n':
			out.WriteString(`\n`)
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

func TestGraphQLRejectsDepthAttackBeforeResolvers(t *testing.T) {
	query := `{ regions(first: 1) { nodes { districts(first: 1) { nodes { region { districts(first: 1) { nodes { region { name } } } } } } } } }`
	rec := graphqlRequest(t, query)
	if rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "depth exceeds") {
		t.Fatalf("depth attack was not rejected by the depth budget: %s", rec.Body.String())
	}
}

func TestGraphQLRejectsWeightedComplexityAttackBeforeResolvers(t *testing.T) {
	query := `{ nearby(latitude: 5.6, longitude: -0.2, first: 100) { distanceMeters } }`
	rec := graphqlRequest(t, query)
	if !strings.Contains(rec.Body.String(), "operation has complexity") {
		t.Fatalf("complexity attack was not rejected by the weighted budget: %s", rec.Body.String())
	}
}
