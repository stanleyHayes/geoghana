package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Key format (Spec §12.2):
//
//	gh_live_<prefix>_<secret>
//	gh_test_<prefix>_<secret>
//
// The prefix is public: it is stored in plaintext, shown in the portal, and
// written to logs so a key can be traced without ever exposing the secret.
// The secret is shown to the user exactly once and stored only as an argon2id
// digest. There is no recovery path — that is the point.

const (
	prefixBytes = 6  // 12 hex characters, enough to be unique and greppable
	secretBytes = 32 // 256 bits
)

var (
	ErrMalformedKey = errors.New("malformed API key")
	ErrWrongSecret  = errors.New("secret does not match")
)

// argon2id parameters. Deliberately cheap relative to password hashing: an API
// key is 256 bits of true randomness, not a human-chosen password, so it needs
// no protection against dictionary attack — only against a leaked-database
// attacker, and it must stay fast enough to run on every request.
const (
	argonTime    = 1
	argonMemory  = 16 * 1024 // 16 MiB
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// GeneratedKey is returned once at creation. Secret is never persisted.
type GeneratedKey struct {
	// Full is the complete key. Show it once, then discard it.
	Full string
	// Prefix is safe to store and display.
	Prefix string
	// SecretHash is what goes in the database.
	SecretHash string
}

// Generate creates a new key for an environment.
func Generate(env Environment) (GeneratedKey, error) {
	prefixRaw := make([]byte, prefixBytes)
	if _, err := rand.Read(prefixRaw); err != nil {
		return GeneratedKey{}, fmt.Errorf("generate prefix: %w", err)
	}
	secretRaw := make([]byte, secretBytes)
	if _, err := rand.Read(secretRaw); err != nil {
		return GeneratedKey{}, fmt.Errorf("generate secret: %w", err)
	}

	prefix := hex.EncodeToString(prefixRaw)
	secret := base64.RawURLEncoding.EncodeToString(secretRaw)

	hash, err := HashSecret(secret)
	if err != nil {
		return GeneratedKey{}, err
	}
	return GeneratedKey{
		Full:       fmt.Sprintf("gh_%s_%s_%s", env, prefix, secret),
		Prefix:     fmt.Sprintf("gh_%s_%s", env, prefix),
		SecretHash: hash,
	}, nil
}

// NewID creates a non-secret, collision-resistant identifier for identity
// resources. The prefix keeps logs and support conversations readable.
func NewID(prefix string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(raw), nil
}

func GenerateInvitationToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate invitation token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	return token, hex.EncodeToString(sum[:]), nil
}
func InvitationTokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// HashSecret produces a self-describing argon2id digest, so the parameters can
// change later without invalidating existing keys.
func HashSecret(secret string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	sum := argon2.IDKey([]byte(secret), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("argon2id$%d$%d$%d$%s$%s",
		argonTime, argonMemory, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(sum),
	), nil
}

// VerifySecret compares a presented secret against a stored digest in constant
// time. A non-constant comparison here would leak the digest one byte at a time.
func VerifySecret(secret, stored string) error {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[0] != "argon2id" {
		return ErrMalformedKey
	}
	var t, m uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[1], "%d", &t); err != nil {
		return ErrMalformedKey
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &m); err != nil {
		return ErrMalformedKey
	}
	if _, err := fmt.Sscanf(parts[3], "%d", &p); err != nil {
		return ErrMalformedKey
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrMalformedKey
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrMalformedKey
	}

	got := argon2.IDKey([]byte(secret), salt, t, m, p, uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrWrongSecret
	}
	return nil
}

// ParsedKey is a presented key split into its lookup prefix and its secret.
type ParsedKey struct {
	Prefix      string
	Secret      string
	Environment Environment
}

// ParseKey splits a presented key. It performs no I/O and no comparison, so it
// is safe to call on untrusted input before any database lookup.
func ParseKey(raw string) (ParsedKey, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "Bearer ")
	raw = strings.TrimSpace(raw)

	// SplitN with a limit of 4, NOT Split: the secret is base64url encoded and
	// therefore legitimately contains "_", which is also the delimiter. Using
	// Split here silently rejected roughly half of all generated keys.
	parts := strings.SplitN(raw, "_", 4)
	if len(parts) != 4 || parts[0] != "gh" {
		return ParsedKey{}, ErrMalformedKey
	}
	env := Environment(parts[1])
	if env != EnvLive && env != EnvTest {
		return ParsedKey{}, ErrMalformedKey
	}
	if parts[2] == "" || parts[3] == "" {
		return ParsedKey{}, ErrMalformedKey
	}
	return ParsedKey{
		Prefix:      "gh_" + parts[1] + "_" + parts[2],
		Secret:      parts[3],
		Environment: env,
	}, nil
}

// Redact renders a key safe for a log line or an error message.
func Redact(raw string) string {
	p, err := ParseKey(raw)
	if err != nil {
		return "<malformed key>"
	}
	return p.Prefix + "_" + strings.Repeat("*", 8)
}
