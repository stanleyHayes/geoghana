package account

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// Session lifetimes. Short by design: a stolen session token is only useful
// until it expires, and rotation shortens that further.
const (
	// IdleTimeout ends a session that has not been used.
	IdleTimeout = 30 * time.Minute
	// AbsoluteTimeout ends a session regardless of activity, so a token that
	// is being kept alive by an attacker still dies.
	AbsoluteTimeout = 12 * time.Hour
	// PrivilegedAbsoluteTimeout is shorter for roles that can change data.
	PrivilegedAbsoluteTimeout = 4 * time.Hour

	tokenBytes = 32 // 256 bits
)

// Stage is how far a session has got through authentication.
//
// A session that has passed a password but not yet a second factor is a real
// session with real state, and modelling it explicitly is what stops it being
// mistaken for an authenticated one.
type Stage string

const (
	// StagePendingMFA has proven a first factor only. It may do nothing except
	// complete or abandon MFA.
	StagePendingMFA Stage = "pending_mfa"
	// StageAuthenticated has satisfied every factor its role requires.
	StageAuthenticated Stage = "authenticated"
)

// Session is a signed-in browser session.
type Session struct {
	ID        string
	AccountID string
	Role      Role
	Stage     Stage
	// TokenHash is the SHA-256 of the token handed to the client. The token
	// itself is never stored: a database leak must not hand an attacker live
	// sessions, and unlike a password the token is already high-entropy, so a
	// fast hash is the right choice.
	TokenHash string
	// Epoch is the account's SessionEpoch at issue time. A global revocation
	// bumps the account and every older session stops verifying.
	Epoch     int
	IssuedAt  time.Time
	ExpiresAt time.Time
	// LastUsedAt drives the idle timeout.
	LastUsedAt time.Time
	// RotatedFrom links to the session this one replaced, so a reused old
	// token can be recognised as theft rather than treated as merely expired.
	RotatedFrom string
	RevokedAt   *time.Time
	UserAgent   string
	IP          string
}

// IssuedSession is returned once at creation. Token is never persisted.
type IssuedSession struct {
	Session Session
	// Token goes to the client and is never stored or logged.
	Token string
}

// NewToken returns a high-entropy session token and its storage hash.
func NewToken() (token, hash string, err error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate session token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken is SHA-256 over the token. Deliberately NOT argon2: the token is
// 256 bits of randomness, so there is no dictionary to attack, and session
// lookup happens on every request.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// TokenMatches compares in constant time, so an attacker cannot learn the
// stored hash one byte at a time from response timing.
func TokenMatches(token, storedHash string) bool {
	return subtle.ConstantTimeCompare([]byte(HashToken(token)), []byte(storedHash)) == 1
}

// absoluteTimeoutFor gives privileged roles a shorter ceiling.
func absoluteTimeoutFor(r Role) time.Duration {
	if r.RequiresMFA() {
		return PrivilegedAbsoluteTimeout
	}
	return AbsoluteTimeout
}

// Issue creates a brand-new session for an account.
//
// This is ALWAYS a new id and a new token. Session fixation is prevented by
// construction: there is no code path that promotes an existing session's
// identifier across an authentication boundary, so a token an attacker planted
// before sign-in can never become an authenticated one.
func Issue(a Account, stage Stage, now time.Time, ua, ip string) (IssuedSession, error) {
	token, hash, err := NewToken()
	if err != nil {
		return IssuedSession{}, err
	}
	id, err := randomID("sess")
	if err != nil {
		return IssuedSession{}, err
	}
	return IssuedSession{
		Session: Session{
			ID: id, AccountID: a.ID, Role: a.Role, Stage: stage,
			TokenHash: hash, Epoch: a.SessionEpoch,
			IssuedAt: now, LastUsedAt: now,
			ExpiresAt: now.Add(absoluteTimeoutFor(a.Role)),
			UserAgent: ua, IP: ip,
		},
		Token: token,
	}, nil
}

// Verify reports whether the session may act right now.
//
// It takes the ACCOUNT as well, because three of the checks — disabled,
// role change and global revocation — are properties of the account that a
// session cannot know on its own. A session validated without its account is
// a session that keeps working after the account is disabled.
func (s Session) Verify(a Account, now time.Time) error {
	if s.RevokedAt != nil {
		return ErrSessionRevoked
	}
	if a.Disabled {
		return ErrAccountDisabled
	}
	// Global revocation: one write on the account invalidates every session.
	if s.Epoch != a.SessionEpoch {
		return ErrSessionRevoked
	}
	if now.After(s.ExpiresAt) {
		return ErrSessionExpired
	}
	if now.Sub(s.LastUsedAt) > IdleTimeout {
		return ErrSessionExpired
	}
	// A role promoted since issue must re-authenticate: a session created as
	// a developer must not silently gain steward powers.
	if s.Role != a.Role {
		return ErrSessionRevoked
	}
	if s.Stage != StageAuthenticated {
		return ErrMFANotSatisfied
	}
	return nil
}

// VerifyPreMFA runs every session check EXCEPT the MFA stage.
//
// It exists for exactly one caller: enrolling a first authenticator on a
// privileged account, which necessarily happens before MFA can be satisfied.
// Splitting it out keeps Verify's contract absolute — Verify never returns nil
// for a session that has not completed MFA — instead of adding a parameter
// that could be passed wrongly somewhere else.
func (s Session) VerifyPreMFA(a Account, now time.Time) error {
	if s.RevokedAt != nil {
		return ErrSessionRevoked
	}
	if a.Disabled {
		return ErrAccountDisabled
	}
	if s.Epoch != a.SessionEpoch {
		return ErrSessionRevoked
	}
	if now.After(s.ExpiresAt) {
		return ErrSessionExpired
	}
	if now.Sub(s.LastUsedAt) > IdleTimeout {
		return ErrSessionExpired
	}
	if s.Role != a.Role {
		return ErrSessionRevoked
	}
	return nil
}

// Rotate issues a replacement token for the same logical session.
//
// Rotation is what turns a stolen token into a detectable event: the thief and
// the legitimate user cannot both keep using the chain, and whoever presents
// the superseded token afterwards is caught by RotatedFrom.
func (s Session) Rotate(now time.Time) (Session, string, error) {
	token, hash, err := NewToken()
	if err != nil {
		return Session{}, "", err
	}
	id, err := randomID("sess")
	if err != nil {
		return Session{}, "", err
	}
	next := s
	next.ID = id
	next.TokenHash = hash
	next.LastUsedAt = now
	next.RotatedFrom = s.ID
	// The absolute expiry is NOT extended. Rotation refreshes the idle clock
	// only; otherwise a session could be kept alive forever and the absolute
	// timeout would mean nothing.
	return next, token, nil
}

// Elevate moves a pending session to authenticated after a second factor.
//
// It returns a NEW session rather than mutating this one, so the token that
// existed before MFA cannot be used after it — the same fixation defence as
// at sign-in.
func (s Session) Elevate(a Account, now time.Time, ua, ip string) (IssuedSession, error) {
	if s.Stage != StagePendingMFA {
		return IssuedSession{}, fmt.Errorf("session is not awaiting MFA")
	}
	return Issue(a, StageAuthenticated, now, ua, ip)
}

// NewID mints a prefixed random identifier for an account, session or token.
func NewID(prefix string) (string, error) { return randomID(prefix) }

func randomID(prefix string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}
