package account

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters for PASSWORDS.
//
// Deliberately far more expensive than the API-key parameters in the identity
// package. A key is 256 bits of true randomness with no dictionary to attack;
// a human-chosen password has perhaps 30 bits of entropy and must be made
// expensive to guess offline. These follow the OWASP argon2id guidance of
// 19MiB with one iteration, which is the memory-hard corner of the trade-off.
const (
	pwTime    = 1
	pwMemory  = 19 * 1024 // 19 MiB
	pwThreads = 1
	pwKeyLen  = 32
	pwSaltLen = 16
)

// MinPasswordLength follows NIST SP 800-63B: length is what matters, and
// composition rules ("one capital, one symbol") push people toward
// "Password1!" — predictable and no stronger.
const MinPasswordLength = 12

var ErrPasswordMismatch = errors.New("password does not match")

// ValidatePassword enforces length and rejects the handful of values that are
// long but worthless. It deliberately imposes no composition rules.
func ValidatePassword(pw string) error {
	if utf8.RuneCountInString(pw) < MinPasswordLength {
		return fmt.Errorf("%w: at least %d characters", ErrWeakPassword, MinPasswordLength)
	}
	lower := strings.ToLower(strings.TrimSpace(pw))
	for _, bad := range []string{
		"password", "passwordpassword", "123456789012", "qwertyuiopas",
		"ghanageo", "administrator", "letmeinletmein",
	} {
		if lower == bad {
			return fmt.Errorf("%w: that password is too common", ErrWeakPassword)
		}
	}
	// A single repeated character is long but has almost no entropy.
	if distinctRunes(lower) <= 2 {
		return fmt.Errorf("%w: too few distinct characters", ErrWeakPassword)
	}
	return nil
}

func distinctRunes(s string) int {
	seen := map[rune]struct{}{}
	for _, r := range s {
		seen[r] = struct{}{}
	}
	return len(seen)
}

// HashPassword returns an encoded argon2id digest carrying its own parameters,
// so the cost can be raised later without invalidating existing hashes.
func HashPassword(pw string) (string, error) {
	if err := ValidatePassword(pw); err != nil {
		return "", err
	}
	salt := make([]byte, pwSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(pw), salt, pwTime, pwMemory, pwThreads, pwKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, pwMemory, pwTime, pwThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword compares a candidate against an encoded digest in constant
// time.
func VerifyPassword(pw, encoded string) error {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return ErrPasswordMismatch
	}
	var version, memory, time32, threads int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return ErrPasswordMismatch
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time32, &threads); err != nil {
		return ErrPasswordMismatch
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return ErrPasswordMismatch
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return ErrPasswordMismatch
	}
	got := argon2.IDKey([]byte(pw), salt, uint32(time32), uint32(memory), uint8(threads), uint32(len(want)))
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

// DummyHash is a real argon2id digest of a random value, used to keep the
// timing of "no such account" indistinguishable from "wrong password".
//
// Without it, a missing account returns immediately while a wrong password
// pays for a hash, and the difference is a reliable account-enumeration
// oracle even when both return the same error text.
var DummyHash = func() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	h, err := HashPassword(base64.RawURLEncoding.EncodeToString(b))
	if err != nil {
		return ""
	}
	return h
}()
