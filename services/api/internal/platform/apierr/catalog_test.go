package apierr

import "testing"

// TestCatalogMatchesHTTPMapping is the drift guard between the YAML catalog and
// the hand-written httpStatus map. If someone adds a code to one and not the
// other, this fails rather than shipping an undocumented status.
func TestCatalogMatchesHTTPMapping(t *testing.T) {
	for code, entry := range Catalog {
		if got := code.HTTPStatus(); got != entry.HTTP {
			t.Errorf("%s: catalog says HTTP %d, code.HTTPStatus() returns %d", code, entry.HTTP, got)
		}
	}
}

// Every code the package declares must appear in the published catalog,
// or callers would meet an error code with no documentation page.
func TestEveryDeclaredCodeIsPublished(t *testing.T) {
	declared := []Code{
		InvalidArgument, InvalidCoordinate, RadiusOutOfRange, QueryTooShort,
		PayloadTooLarge, Unauthenticated, KeyRevoked, PermissionDenied,
		OriginNotAllowed, NotFound, ResourceGone, RateLimitExceeded,
		QuotaExceeded, QueryTooComplex, DeadlineExceeded, Internal,
	}
	for _, c := range declared {
		if _, ok := Catalog[c]; !ok {
			t.Errorf("%s is declared in Go but missing from contracts/errors/catalog.yaml", c)
		}
	}
	if len(Catalog) != len(declared) {
		t.Errorf("catalog has %d entries, package declares %d — they must match", len(Catalog), len(declared))
	}
}

func TestEveryCodeHasADocsURL(t *testing.T) {
	for code := range Catalog {
		if got := code.DocsURL(); got != "/docs/errors/"+string(code) {
			t.Errorf("%s: unexpected docs URL %q", code, got)
		}
	}
}

func TestFromDefaultsToInternal(t *testing.T) {
	// An unmapped failure must never leak an implementation detail.
	e := From(errNotAnAPIError{})
	if e.Code != Internal {
		t.Errorf("expected INTERNAL for an unknown error, got %s", e.Code)
	}
	if e.Message == "boom" {
		t.Error("the underlying message must not become the public message")
	}
}

type errNotAnAPIError struct{}

func (errNotAnAPIError) Error() string { return "boom" }
