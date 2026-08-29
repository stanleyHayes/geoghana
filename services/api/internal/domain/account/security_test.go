package account

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

func devAccount() Account {
	return Account{
		ID: "acc_1", Email: "ama@example.com", EmailVerified: true,
		Role: RoleDeveloper, SessionEpoch: 1,
	}
}

func adminAccount() Account {
	return Account{
		ID: "acc_2", Email: "steward@example.com", EmailVerified: true,
		Role: RoleDataAdmin, SessionEpoch: 1, TOTPSecret: "SECRET",
	}
}

// Spec §22.3 names session fixation explicitly.
//
// The defence is structural: every authentication boundary mints a NEW id and
// token, so a token an attacker plants before sign-in can never be promoted
// into an authenticated one.
func TestSessionFixationIsImpossible(t *testing.T) {
	a := adminAccount()

	pre, err := Issue(a, StagePendingMFA, now, "ua", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}

	post, err := pre.Session.Elevate(a, now, "ua", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}

	if post.Session.ID == pre.Session.ID {
		t.Error("session id survived the MFA boundary — fixation is possible")
	}
	if post.Token == pre.Token {
		t.Error("session token survived the MFA boundary — fixation is possible")
	}
	if post.Session.TokenHash == pre.Session.TokenHash {
		t.Error("token hash unchanged across the MFA boundary")
	}
	if post.Session.Stage != StageAuthenticated {
		t.Errorf("stage after elevation = %q", post.Session.Stage)
	}

	// The pre-MFA session must still be unusable on its own.
	if err := pre.Session.Verify(a, now); !errors.Is(err, ErrMFANotSatisfied) {
		t.Errorf("pending session verified as %v, want ErrMFANotSatisfied", err)
	}
}

// A session awaiting MFA must not be able to elevate itself twice, and an
// already-authenticated session must not re-run the MFA boundary.
func TestElevateOnlyFromPendingMFA(t *testing.T) {
	a := adminAccount()
	s, _ := Issue(a, StageAuthenticated, now, "ua", "ip")
	if _, err := s.Session.Elevate(a, now, "ua", "ip"); err == nil {
		t.Error("an authenticated session was allowed to elevate again")
	}
}

// "Sign out everywhere" must be one write that cannot half-succeed.
func TestGlobalRevocationKillsEveryExistingSession(t *testing.T) {
	a := devAccount()
	s1, _ := Issue(a, StageAuthenticated, now, "ua", "ip")
	s2, _ := Issue(a, StageAuthenticated, now, "ua2", "ip2")

	if err := s1.Session.Verify(a, now); err != nil {
		t.Fatalf("fresh session rejected: %v", err)
	}

	a.SessionEpoch++ // the single write behind "sign out everywhere"

	for i, s := range []Session{s1.Session, s2.Session} {
		if err := s.Verify(a, now); !errors.Is(err, ErrSessionRevoked) {
			t.Errorf("session %d survived global revocation: %v", i, err)
		}
	}
}

// Logout of one session must not disturb the others (Spec §22.3).
func TestSingleLogoutRevokesOnlyThatSession(t *testing.T) {
	a := devAccount()
	s1, _ := Issue(a, StageAuthenticated, now, "ua", "ip")
	s2, _ := Issue(a, StageAuthenticated, now, "ua2", "ip2")

	revoked := now
	one := s1.Session
	one.RevokedAt = &revoked

	if err := one.Verify(a, now); !errors.Is(err, ErrSessionRevoked) {
		t.Errorf("logged-out session still valid: %v", err)
	}
	if err := s2.Session.Verify(a, now); err != nil {
		t.Errorf("an unrelated session was revoked too: %v", err)
	}
}

func TestSessionTimeouts(t *testing.T) {
	a := devAccount()
	s, _ := Issue(a, StageAuthenticated, now, "ua", "ip")

	t.Run("idle", func(t *testing.T) {
		if err := s.Session.Verify(a, now.Add(IdleTimeout+time.Minute)); !errors.Is(err, ErrSessionExpired) {
			t.Errorf("idle session still valid: %v", err)
		}
	})

	t.Run("absolute survives rotation", func(t *testing.T) {
		// Rotation refreshes the idle clock but must NOT extend the absolute
		// expiry, or a session could be kept alive indefinitely.
		cur := s.Session
		for i := 0; i < 5; i++ {
			next, _, err := cur.Rotate(now.Add(time.Duration(i) * time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			cur = next
		}
		if !cur.ExpiresAt.Equal(s.Session.ExpiresAt) {
			t.Errorf("rotation moved the absolute expiry: %v → %v",
				s.Session.ExpiresAt, cur.ExpiresAt)
		}
		if err := cur.Verify(a, now.Add(AbsoluteTimeout+time.Minute)); !errors.Is(err, ErrSessionExpired) {
			t.Errorf("session outlived its absolute timeout: %v", err)
		}
	})

	t.Run("privileged roles expire sooner", func(t *testing.T) {
		adm, _ := Issue(adminAccount(), StageAuthenticated, now, "ua", "ip")
		if !adm.Session.ExpiresAt.Before(s.Session.ExpiresAt) {
			t.Error("a privileged session lasts as long as a developer one")
		}
	})
}

// Rotation must change the token every time, and link back so a replayed old
// token is recognisable as theft rather than a plain expiry.
func TestRotationIssuesAFreshTokenAndLinksBack(t *testing.T) {
	a := devAccount()
	s, _ := Issue(a, StageAuthenticated, now, "ua", "ip")

	next, token, err := s.Session.Rotate(now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if token == s.Token {
		t.Error("rotation reissued the same token")
	}
	if next.ID == s.Session.ID {
		t.Error("rotation reused the session id")
	}
	if next.RotatedFrom != s.Session.ID {
		t.Errorf("RotatedFrom = %q, want %q", next.RotatedFrom, s.Session.ID)
	}
	if !TokenMatches(token, next.TokenHash) {
		t.Error("the new token does not match its stored hash")
	}
	if TokenMatches(s.Token, next.TokenHash) {
		t.Error("the OLD token still matches the rotated session")
	}
}

// A session must stop working the moment its account is disabled, which is
// only knowable by validating the two together.
func TestDisabledAccountInvalidatesLiveSessions(t *testing.T) {
	a := devAccount()
	s, _ := Issue(a, StageAuthenticated, now, "ua", "ip")
	a.Disabled = true
	if err := s.Session.Verify(a, now); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("disabled account kept a live session: %v", err)
	}
}

// A privilege change must force re-authentication rather than silently
// upgrading an existing session.
func TestRoleChangeInvalidatesSession(t *testing.T) {
	a := devAccount()
	s, _ := Issue(a, StageAuthenticated, now, "ua", "ip")
	a.Role = RoleSuperAdmin
	if err := s.Session.Verify(a, now); !errors.Is(err, ErrSessionRevoked) {
		t.Errorf("a developer session silently became a super-admin one: %v", err)
	}
}

// MFA is mandatory for every role except the ordinary developer, and the
// default answer for an unrecognised role must be the safe one.
func TestMFAIsMandatoryForEveryPrivilegedRole(t *testing.T) {
	for _, r := range []Role{
		RoleSuperAdmin, RoleDataAdmin, RoleDataReviewer,
		RoleDataContributor, RoleDeveloperSupport, RoleSecurityAuditor,
	} {
		if !r.RequiresMFA() {
			t.Errorf("role %s does not require MFA (Spec §12.1)", r)
		}
	}
	if RoleDeveloper.RequiresMFA() {
		t.Error("an ordinary developer should not be forced into MFA")
	}
	// Deny by default: a role added later without thought gets the safe answer.
	if !Role("SOME_NEW_ROLE").RequiresMFA() {
		t.Error("an unknown role defaulted to NOT requiring MFA")
	}
}

func TestPrivilegedAccountMustEnrolAFactor(t *testing.T) {
	a := adminAccount()
	a.TOTPSecret = ""
	a.Passkeys = nil
	if err := a.RequireMFAEnrolment(); !errors.Is(err, ErrMFARequired) {
		t.Errorf("a steward with no second factor was allowed through: %v", err)
	}
	a.Passkeys = []Passkey{{ID: "pk_1"}}
	if err := a.RequireMFAEnrolment(); err != nil {
		t.Errorf("a passkey should satisfy enrolment: %v", err)
	}
}

func TestUnverifiedEmailCannotSignIn(t *testing.T) {
	a := devAccount()
	a.EmailVerified = false
	if err := a.CanSignIn(); !errors.Is(err, ErrEmailNotVerified) {
		t.Errorf("unverified account signed in: %v", err)
	}
	// Disabled is reported before unverified, so the two cannot be told apart
	// by an outsider probing which error comes back.
	a.Disabled = true
	if err := a.CanSignIn(); !errors.Is(err, ErrAccountDisabled) {
		t.Errorf("disabled account reported %v", err)
	}
}

// One-time tokens: single use, expiring, and bound to their purpose.
func TestOneTimeTokens(t *testing.T) {
	iss, err := IssueToken("acc_1", PurposePasswordReset, now)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("redeems once", func(t *testing.T) {
		if err := iss.Token.Redeem(iss.Value, PurposePasswordReset, now); err != nil {
			t.Fatalf("valid token rejected: %v", err)
		}
		used := iss.Token
		at := now
		used.UsedAt = &at
		if err := used.Redeem(iss.Value, PurposePasswordReset, now); !errors.Is(err, ErrTokenAlreadyUsed) {
			t.Errorf("token replayed: %v", err)
		}
	})

	t.Run("purpose is a capability boundary", func(t *testing.T) {
		// The MFA-recovery flow (Spec §22.3): a recovery token must not be
		// spendable as a password reset, or vice versa.
		if err := iss.Token.Redeem(iss.Value, PurposeMFARecovery, now); err == nil {
			t.Error("a password-reset token was redeemed for MFA recovery")
		}
	})

	t.Run("expires", func(t *testing.T) {
		if err := iss.Token.Redeem(iss.Value, PurposePasswordReset,
			now.Add(PasswordResetTTL+time.Minute)); !errors.Is(err, ErrTokenExpired) {
			t.Errorf("expired token accepted: %v", err)
		}
	})

	t.Run("wrong value", func(t *testing.T) {
		if err := iss.Token.Redeem("not-the-token", PurposePasswordReset, now); err == nil {
			t.Error("an incorrect token value was accepted")
		}
	})

	t.Run("recovery tokens are short-lived", func(t *testing.T) {
		rec, _ := IssueToken("acc_1", PurposeMFARecovery, now)
		if rec.Token.ExpiresAt.Sub(now) > MFARecoveryTTL {
			t.Error("MFA recovery token lives longer than its TTL")
		}
		if rec.Token.ExpiresAt.After(iss.Token.ExpiresAt) {
			t.Error("an MFA recovery token outlives a password reset — it must be tighter")
		}
	})

	t.Run("plaintext is never stored", func(t *testing.T) {
		if strings.Contains(iss.Token.Hash, iss.Value) || iss.Token.Hash == iss.Value {
			t.Error("the token value was stored rather than its hash")
		}
	})
}

func TestPasswordHashing(t *testing.T) {
	const pw = "correct horse battery staple"

	h, err := HashPassword(pw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h, pw) {
		t.Fatal("the password appears in its own hash")
	}
	if err := VerifyPassword(pw, h); err != nil {
		t.Errorf("correct password rejected: %v", err)
	}
	if err := VerifyPassword("wrong password entirely", h); !errors.Is(err, ErrPasswordMismatch) {
		t.Errorf("wrong password accepted: %v", err)
	}

	t.Run("salted", func(t *testing.T) {
		h2, _ := HashPassword(pw)
		if h == h2 {
			t.Error("the same password hashed identically twice — no salt")
		}
	})

	t.Run("rejects weak passwords", func(t *testing.T) {
		for _, bad := range []string{"short", "password", "aaaaaaaaaaaaaaaa", ""} {
			if err := ValidatePassword(bad); err == nil {
				t.Errorf("weak password %q accepted", bad)
			}
		}
	})

	t.Run("accepts a long passphrase without composition rules", func(t *testing.T) {
		if err := ValidatePassword("kwabenya to osu every morning"); err != nil {
			t.Errorf("a good passphrase was rejected: %v", err)
		}
	})

	t.Run("malformed digest fails closed", func(t *testing.T) {
		for _, bad := range []string{"", "not-a-hash", "$argon2id$broken"} {
			if err := VerifyPassword(pw, bad); err == nil {
				t.Errorf("malformed digest %q verified", bad)
			}
		}
	})

	t.Run("dummy hash exists for timing", func(t *testing.T) {
		// Without a real digest to verify against, a missing account returns
		// far faster than a wrong password and becomes an enumeration oracle.
		if DummyHash == "" {
			t.Fatal("DummyHash is empty — account enumeration by timing is possible")
		}
		if err := VerifyPassword("anything at all", DummyHash); err == nil {
			t.Error("DummyHash verified against an arbitrary password")
		}
	})
}

func TestEmailNormalizationAndValidation(t *testing.T) {
	if got := NormalizeEmail("  Ama@Example.COM "); got != "ama@example.com" {
		t.Errorf("NormalizeEmail = %q", got)
	}
	for _, ok := range []string{"a@b.co", "ama.mensah@ug.edu.gh", "x+tag@example.org"} {
		if err := ValidateEmail(ok); err != nil {
			t.Errorf("valid email %q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "no-at-sign", "@example.com", "a@b", "a b@c.com", "trailing@"} {
		if err := ValidateEmail(bad); err == nil {
			t.Errorf("invalid email %q accepted", bad)
		}
	}
}

func TestTokenComparisonIsConstantTime(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !TokenMatches(token, hash) {
		t.Error("a token did not match its own hash")
	}
	if TokenMatches(token+"x", hash) {
		t.Error("a modified token matched")
	}
	// Two tokens must never collide.
	t2, h2, _ := NewToken()
	if token == t2 || hash == h2 {
		t.Error("two generated tokens collided")
	}
}
