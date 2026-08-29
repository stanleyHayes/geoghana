package identity

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGeneratedKeyShape(t *testing.T) {
	for _, env := range []Environment{EnvLive, EnvTest} {
		k, err := Generate(env)
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		if !strings.HasPrefix(k.Full, "gh_"+string(env)+"_") {
			t.Errorf("full key %q lacks the gh_%s_ prefix", k.Full, env)
		}
		if !strings.HasPrefix(k.Full, k.Prefix) {
			t.Errorf("prefix %q is not a prefix of the full key", k.Prefix)
		}
		// The secret must never be recoverable from what we store.
		secret := strings.TrimPrefix(k.Full, k.Prefix+"_")
		if strings.Contains(k.SecretHash, secret) {
			t.Fatal("the stored digest contains the secret in plaintext")
		}
		if strings.Contains(k.Prefix, secret) {
			t.Fatal("the public prefix contains the secret")
		}
	}
}

func TestKeysAreUnique(t *testing.T) {
	seen := make(map[string]bool, 200)
	for i := 0; i < 200; i++ {
		k, err := Generate(EnvLive)
		if err != nil {
			t.Fatal(err)
		}
		if seen[k.Prefix] {
			t.Fatalf("prefix collision after %d keys: %s", i, k.Prefix)
		}
		seen[k.Prefix] = true
	}
}

func TestVerifySecretRoundTrip(t *testing.T) {
	k, err := Generate(EnvLive)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseKey(k.Full)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := VerifySecret(parsed.Secret, k.SecretHash); err != nil {
		t.Fatalf("the freshly generated secret did not verify: %v", err)
	}
	if err := VerifySecret("not-the-secret", k.SecretHash); !errors.Is(err, ErrWrongSecret) {
		t.Fatalf("a wrong secret must be rejected, got %v", err)
	}
}

func TestParseKeyRejectsMalformedInput(t *testing.T) {
	bad := []string{
		"", "gh_live", "gh_live_abc", "xx_live_abc_def",
		"gh_prod_abc_def", // unknown environment
		"gh_live__def",    // empty prefix
		"gh_live_abc_",    // empty secret
		"just-a-string",
	}
	for _, in := range bad {
		if _, err := ParseKey(in); err == nil {
			t.Errorf("ParseKey(%q) should have failed", in)
		}
	}
}

func TestParseKeyAcceptsBearerPrefix(t *testing.T) {
	k, _ := Generate(EnvTest)
	p, err := ParseKey("Bearer " + k.Full)
	if err != nil {
		t.Fatalf("a Bearer-prefixed header should parse: %v", err)
	}
	if p.Prefix != k.Prefix {
		t.Errorf("prefix = %q, want %q", p.Prefix, k.Prefix)
	}
}

func TestRedactNeverLeaksTheSecret(t *testing.T) {
	k, _ := Generate(EnvLive)
	secret := strings.TrimPrefix(k.Full, k.Prefix+"_")
	red := Redact(k.Full)
	if strings.Contains(red, secret) {
		t.Fatalf("Redact leaked the secret: %s", red)
	}
	if !strings.HasPrefix(red, k.Prefix) {
		t.Errorf("Redact should keep the traceable prefix, got %q", red)
	}
	if Redact("garbage") != "<malformed key>" {
		t.Error("malformed input should redact to a fixed string")
	}
}

// A browser key ships inside a client bundle, so it is public by construction.
// Granting it a server-only scope, or leaving it unrestricted by origin, would
// hand anyone who views source a working server credential (Spec §32.5).
func TestBrowserKeySafetyRules(t *testing.T) {
	base := APIKey{Name: "web", Class: ClassBrowser, AllowedOrigins: []string{"https://example.gh"}}

	ok := base
	ok.Scopes = []Scope{ScopeLocationsRead, ScopeSearchRead}
	if err := ok.Validate(); err != nil {
		t.Fatalf("a safe browser key was rejected: %v", err)
	}

	unsafe := base
	unsafe.Scopes = []Scope{ScopeLocationsRead, ScopeGRPCAccess}
	if err := unsafe.Validate(); !errors.Is(err, ErrUnsafeBrowserKey) {
		t.Errorf("grpc:access on a browser key must be rejected, got %v", err)
	}

	noOrigin := base
	noOrigin.AllowedOrigins = nil
	noOrigin.Scopes = []Scope{ScopeLocationsRead}
	if err := noOrigin.Validate(); !errors.Is(err, ErrOriginRequired) {
		t.Errorf("a browser key without origins must be rejected, got %v", err)
	}

	// The same scopes are fine on a server key.
	server := APIKey{Name: "backend", Class: ClassServer, Scopes: []Scope{ScopeGRPCAccess}}
	if err := server.Validate(); err != nil {
		t.Errorf("a server key may hold grpc:access: %v", err)
	}
}

func TestOriginAllowListFailsClosed(t *testing.T) {
	k := APIKey{Class: ClassBrowser, AllowedOrigins: []string{"https://example.gh/"}}
	if !k.OriginAllowed("https://example.gh") {
		t.Error("a trailing slash should not change the decision")
	}
	if k.OriginAllowed("https://evil.example") {
		t.Error("an unlisted origin must be denied")
	}
	empty := APIKey{Class: ClassBrowser}
	if empty.OriginAllowed("https://anything") {
		t.Error("a browser key with no origins must deny everything, not allow everything")
	}
	server := APIKey{Class: ClassServer}
	if !server.OriginAllowed("https://anything") {
		t.Error("a server key is not origin-restricted")
	}
}

func TestUsableRejectsRevokedAndExpired(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	live := APIKey{ExpiresAt: &future}
	if err := live.Usable(now); err != nil {
		t.Errorf("an unexpired key should be usable: %v", err)
	}
	expired := APIKey{ExpiresAt: &past}
	if err := expired.Usable(now); !errors.Is(err, ErrKeyExpired) {
		t.Errorf("expected ErrKeyExpired, got %v", err)
	}
	revoked := APIKey{RevokedAt: &past, ExpiresAt: &future}
	if err := revoked.Usable(now); !errors.Is(err, ErrKeyRevoked) {
		t.Errorf("revocation must take effect immediately, got %v", err)
	}
}

// GhanaGeo is free, so an unauthenticated caller is a first-class identity with
// real capability — not a locked-out one (§24 F1).
func TestAnonymousIsAFirstClassIdentity(t *testing.T) {
	a := Anonymous("41.66.0.1")
	if !a.Anonymous {
		t.Fatal("expected an anonymous identity")
	}
	for _, s := range []Scope{ScopeLocationsRead, ScopeSearchRead, ScopeGeocodeRead, ScopeDatasetsRead} {
		if !a.HasScope(s) {
			t.Errorf("anonymous callers must hold %s — the API is free", s)
		}
	}
	if a.HasScope(ScopeGRPCAccess) {
		t.Error("anonymous should not hold grpc:access, which implies a backend integration")
	}
	if a.RateKey != "ip:41.66.0.1" {
		t.Errorf("anonymous should be limited per IP, got %q", a.RateKey)
	}
}

func TestParseScope(t *testing.T) {
	if _, err := ParseScope("LOCATIONS:READ"); err != nil {
		t.Errorf("scope parsing should be case-insensitive: %v", err)
	}
	if _, err := ParseScope("locations:write"); !errors.Is(err, ErrUnknownScope) {
		t.Error("an invented scope must be rejected")
	}
}

// Regression: the secret is base64url encoded and so legitimately contains
// "_", which is also the key delimiter. Splitting on every underscore rejected
// about half of all generated keys.
func TestSecretsContainingUnderscoresParse(t *testing.T) {
	raw := "gh_live_a1b2c3d4e5f6_abc_def-ghi_jkl"
	p, err := ParseKey(raw)
	if err != nil {
		t.Fatalf("a secret containing underscores must parse: %v", err)
	}
	if p.Prefix != "gh_live_a1b2c3d4e5f6" {
		t.Errorf("prefix = %q", p.Prefix)
	}
	if p.Secret != "abc_def-ghi_jkl" {
		t.Errorf("secret = %q, want the whole remainder including underscores", p.Secret)
	}
}

// Generate/Parse must round-trip for many keys, since whether a secret happens
// to contain an underscore is down to chance.
func TestGenerateParseRoundTripsForManyKeys(t *testing.T) {
	for i := 0; i < 300; i++ {
		k, err := Generate(EnvLive)
		if err != nil {
			t.Fatal(err)
		}
		p, err := ParseKey(k.Full)
		if err != nil {
			t.Fatalf("generated key %q failed to parse: %v", k.Full, err)
		}
		if p.Prefix != k.Prefix {
			t.Fatalf("round-trip prefix mismatch: %q vs %q", p.Prefix, k.Prefix)
		}
		if err := VerifySecret(p.Secret, k.SecretHash); err != nil {
			t.Fatalf("round-tripped secret failed to verify: %v", err)
		}
	}
}
