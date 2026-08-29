package account

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// Purpose is what a one-time token authorises. A token issued to verify an
// email must never be redeemable to reset a password, so the purpose is part
// of the record and is checked on redemption.
type Purpose string

const (
	PurposeEmailVerification Purpose = "email_verification"
	PurposePasswordReset     Purpose = "password_reset"
	PurposeMFARecovery       Purpose = "mfa_recovery"
)

// Token lifetimes. Short, because every one of these grants account access.
const (
	EmailVerificationTTL = 24 * time.Hour
	PasswordResetTTL     = 1 * time.Hour
	MFARecoveryTTL       = 15 * time.Minute
)

// OneTimeToken is a single-use, expiring credential sent out of band.
type OneTimeToken struct {
	ID        string
	AccountID string
	Purpose   Purpose
	// Hash is SHA-256 of the value that was emailed. The value itself is never
	// stored, so a database leak does not hand over live password resets.
	Hash      string
	ExpiresAt time.Time
	// UsedAt makes redemption single-use. It is recorded rather than the row
	// being deleted, so a replay is distinguishable from an unknown token —
	// one is an attack worth logging, the other is a typo.
	UsedAt    *time.Time
	CreatedAt time.Time
}

// IssuedToken carries the plaintext exactly once.
type IssuedToken struct {
	Token OneTimeToken
	// Value is emailed to the user and never persisted or logged.
	Value string
}

func ttlFor(p Purpose) time.Duration {
	switch p {
	case PurposePasswordReset:
		return PasswordResetTTL
	case PurposeMFARecovery:
		return MFARecoveryTTL
	default:
		return EmailVerificationTTL
	}
}

// IssueToken mints a one-time token for a purpose.
func IssueToken(accountID string, p Purpose, now time.Time) (IssuedToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return IssuedToken{}, fmt.Errorf("generate token: %w", err)
	}
	value := base64.RawURLEncoding.EncodeToString(b)
	id, err := randomID("ott")
	if err != nil {
		return IssuedToken{}, err
	}
	return IssuedToken{
		Token: OneTimeToken{
			ID: id, AccountID: accountID, Purpose: p,
			Hash: HashToken(value), CreatedAt: now,
			ExpiresAt: now.Add(ttlFor(p)),
		},
		Value: value,
	}, nil
}

// Redeem checks a token may be used for a purpose right now.
//
// Purpose is verified as well as expiry and single use: a token is a
// capability, and one minted to confirm an address must not be spendable to
// change a password.
func (t OneTimeToken) Redeem(value string, p Purpose, now time.Time) error {
	if t.Purpose != p {
		return fmt.Errorf("token is for %s, not %s", t.Purpose, p)
	}
	if t.UsedAt != nil {
		return ErrTokenAlreadyUsed
	}
	if now.After(t.ExpiresAt) {
		return ErrTokenExpired
	}
	if !TokenMatches(value, t.Hash) {
		return ErrInvalidCredentials
	}
	return nil
}
