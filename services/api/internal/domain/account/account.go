// Package account models developer and administrator accounts, their
// credentials and their sessions (Spec §12.1, story GEO-9.2).
//
// Every security decision lives HERE rather than in a handler, because a rule
// enforced in one transport is a rule missing from the other two. The domain
// answers "may this session act?" and the transports only carry the answer.
package account

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEmailRequired      = errors.New("email is required")
	ErrEmailNotVerified   = errors.New("email is not verified")
	ErrAccountDisabled    = errors.New("account is disabled")
	ErrMFARequired        = errors.New("multi-factor authentication is required for this role")
	ErrMFANotSatisfied    = errors.New("session has not satisfied multi-factor authentication")
	ErrSessionExpired     = errors.New("session has expired")
	ErrSessionRevoked     = errors.New("session has been revoked")
	ErrTokenExpired       = errors.New("token has expired")
	ErrTokenAlreadyUsed   = errors.New("token has already been used")
	ErrNoCredential       = errors.New("account has no usable credential")
	ErrWeakPassword       = errors.New("password does not meet the minimum requirements")
	ErrInvalidCredentials = errors.New("email or password is incorrect")
)

// Role mirrors the six admin roles plus the ordinary developer.
//
// Developer is the default for a self-registered account; every other role is
// granted by a steward and carries a mandatory second factor.
type Role string

const (
	RoleDeveloper        Role = "DEVELOPER"
	RoleSuperAdmin       Role = "SUPER_ADMIN"
	RoleDataAdmin        Role = "DATA_ADMIN"
	RoleDataReviewer     Role = "DATA_REVIEWER"
	RoleDataContributor  Role = "DATA_CONTRIBUTOR"
	RoleDeveloperSupport Role = "DEVELOPER_SUPPORT"
	RoleSecurityAuditor  Role = "SECURITY_AUDITOR"
)

// privilegedRoles must present a second factor. Spec §12.1 makes MFA mandatory
// for administrators and privileged data stewards; a developer managing only
// their own keys is not in that set.
//
// This is a DENY-BY-DEFAULT list in effect: RequiresMFA returns true for
// anything that is not explicitly the developer role, so a role added later
// without thinking about MFA gets the safe answer rather than the convenient
// one.
func (r Role) RequiresMFA() bool { return r != RoleDeveloper }

// Valid reports whether the role is one the system knows.
func (r Role) Valid() bool {
	switch r {
	case RoleDeveloper, RoleSuperAdmin, RoleDataAdmin, RoleDataReviewer,
		RoleDataContributor, RoleDeveloperSupport, RoleSecurityAuditor:
		return true
	}
	return false
}

// Account is a person who signs in.
type Account struct {
	ID    string
	Email string
	// EmailVerified gates everything except the verification flow itself.
	EmailVerified bool
	// PasswordHash is argon2id, or empty for a passkey-only account.
	PasswordHash string
	Role         Role
	Disabled     bool
	// TOTPSecret is the enrolled authenticator secret, encrypted at rest by
	// the adapter. Empty means TOTP is not enrolled.
	TOTPSecret string
	// Passkeys are WebAuthn credentials. Preferred over password (Spec §12.1).
	Passkeys []Passkey
	// SessionEpoch is bumped to revoke every existing session at once, without
	// having to find and delete each one. A session carrying an older epoch is
	// rejected on its next use, so "sign out everywhere" is a single write and
	// cannot half-succeed.
	SessionEpoch int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Passkey is a registered WebAuthn credential.
type Passkey struct {
	// ID is the base64url credential id the authenticator returns.
	ID   string
	Name string
	// Credential is the library's full credential record, stored as JSON.
	//
	// Keeping the whole record rather than picking fields out of it means a
	// library upgrade that starts checking an additional attribute does not
	// silently lose the data it needs. The public key alone is not enough to
	// verify an assertion.
	Credential []byte
	// SignCount is the authenticator's counter. It must never go backwards:
	// a lower value than we last saw means the credential has been cloned.
	SignCount uint32
	// BackedUp reports whether the credential is synced to a cloud keychain.
	// A synced passkey survives losing the device; a device-bound one does
	// not, which changes what recovery advice is honest.
	BackedUp bool
	AddedAt  time.Time
	LastUsed time.Time
}

// ErrPasskeyCloned is returned when an authenticator presents a sign count at
// or below the one already recorded.
var ErrPasskeyCloned = errors.New("authenticator sign count went backwards — the credential may be cloned")

// CheckSignCount enforces the monotonic counter.
//
// A zero count from the authenticator means it does not implement the counter
// at all, which is permitted by the spec and common in platform
// authenticators; there is nothing to compare, so it is accepted.
func CheckSignCount(seen, stored uint32) error {
	if seen == 0 {
		return nil
	}
	if seen <= stored {
		return ErrPasskeyCloned
	}
	return nil
}

// FindPasskey returns the credential with this id.
func (a Account) FindPasskey(id string) (Passkey, bool) {
	for _, p := range a.Passkeys {
		if p.ID == id {
			return p, true
		}
	}
	return Passkey{}, false
}

// NormalizeEmail lowercases and trims. Storing the normalized form is what
// stops "Ama@example.com" and "ama@example.com" becoming two accounts that
// each believe they own the address.
func NormalizeEmail(e string) string {
	return strings.ToLower(strings.TrimSpace(e))
}

// ValidateEmail is deliberately permissive. Deliverability is proven by the
// verification email, not by a regex, and over-strict patterns reject valid
// addresses — which for a Ghanaian audience means rejecting real users.
func ValidateEmail(e string) error {
	e = NormalizeEmail(e)
	if e == "" {
		return ErrEmailRequired
	}
	at := strings.IndexByte(e, '@')
	if at <= 0 || at == len(e)-1 || strings.Contains(e, " ") {
		return ErrEmailRequired
	}
	if !strings.Contains(e[at+1:], ".") {
		return ErrEmailRequired
	}
	return nil
}

// MFAEnrolled reports whether a second factor exists at all.
func (a Account) MFAEnrolled() bool {
	return a.TOTPSecret != "" || len(a.Passkeys) > 0
}

// CanSignIn reports whether the account may begin a session.
//
// Order matters: disabled is checked before unverified so a disabled account
// cannot be probed by watching which error comes back.
func (a Account) CanSignIn() error {
	if a.Disabled {
		return ErrAccountDisabled
	}
	if !a.EmailVerified {
		return ErrEmailNotVerified
	}
	return nil
}

// RequireMFAEnrolment reports whether this account must enrol a second factor
// before it can be used. A privileged role with no factor enrolled is a
// misconfiguration that must block, not warn.
func (a Account) RequireMFAEnrolment() error {
	if a.Role.RequiresMFA() && !a.MFAEnrolled() {
		return ErrMFARequired
	}
	return nil
}
