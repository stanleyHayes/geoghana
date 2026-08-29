package identity

import (
	"strings"
	"testing"
)

// Key verification runs on EVERY authenticated request, so an unbounded
// argon2 parameter read back from a stored digest is worse here than on the
// sign-in path: one row carrying m=4294967295 would ask for a four-terabyte
// allocation on every call that presents that key.
func TestVerifySecretBoundsStoredParameters(t *testing.T) {
	gen, err := Generate(EnvLive)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseKey(gen.Full)
	if err != nil {
		t.Fatal(err)
	}
	secret := parsed.Secret
	if err := VerifySecret(secret, gen.SecretHash); err != nil {
		t.Fatalf("a well-formed digest was rejected: %v", err)
	}

	parts := strings.Split(gen.SecretHash, "$")
	for _, params := range []string{
		"m=4294967295,t=1,p=1", "m=16384,t=999999,p=1",
		"m=16384,t=1,p=99999", "m=0,t=1,p=1",
	} {
		bad := strings.Join([]string{parts[0], parts[1], parts[2], params, parts[4], parts[5]}, "$")
		if err := VerifySecret(secret, bad); err == nil {
			t.Errorf("hostile parameters %q were accepted", params)
		}
	}
}
