package account

import (
	"context"
	"errors"
	"net/http"
	"time"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/passkey"
)

// WithPasskeys attaches the WebAuthn relying party and its challenge store.
// Absent, the passkey endpoints report that they are not configured — a
// deployment without an RPID must not silently accept ceremonies.
func (s *Service) WithPasskeys(rp *passkey.RP, chal *mongoadapter.ChallengeRepo) *Service {
	s.passkeys = rp
	s.challenges = chal
	return s
}

// Ceremony is what the browser needs to call navigator.credentials with.
type Ceremony struct {
	// ChallengeID is the opaque handle for the server-side challenge. The
	// challenge itself never leaves the server: a client that could choose it
	// could replay a captured assertion.
	ChallengeID string
	Options     any
}

func (s *Service) passkeysReady() error {
	if s.passkeys == nil || s.challenges == nil {
		return apierr.New(apierr.Internal, "Passkeys are not configured on this deployment.")
	}
	return nil
}

// BeginPasskeyRegistration starts enrolling a credential.
func (s *Service) BeginPasskeyRegistration(ctx context.Context, accountID string) (Ceremony, error) {
	if err := s.passkeysReady(); err != nil {
		return Ceremony{}, err
	}
	a, err := s.accounts.ByID(ctx, accountID)
	if err != nil {
		return Ceremony{}, apierr.New(apierr.NotFound, "No such account.")
	}
	opts, session, err := s.passkeys.BeginRegistration(*a)
	if err != nil {
		return Ceremony{}, apierr.Wrap(apierr.Internal, "Could not start passkey registration.", err)
	}
	id, err := s.challenges.Put(ctx, a.ID, "registration", session)
	if err != nil {
		return Ceremony{}, apierr.Wrap(apierr.Internal, "Could not start passkey registration.", err)
	}
	return Ceremony{ChallengeID: id, Options: opts}, nil
}

// FinishPasskeyRegistration validates the authenticator response and stores it.
func (s *Service) FinishPasskeyRegistration(
	ctx context.Context, accountID, challengeID, name string, r *http.Request,
) (account.Passkey, error) {
	if err := s.passkeysReady(); err != nil {
		return account.Passkey{}, err
	}
	owner, session, err := s.challenges.Take(ctx, challengeID, "registration")
	if err != nil {
		return account.Passkey{}, apierr.New(apierr.InvalidArgument,
			"That registration attempt has expired. Start again.")
	}
	// The challenge is bound to the account that started it. Without this, a
	// challenge obtained by one account could be completed by another.
	if owner != accountID {
		return account.Passkey{}, apierr.New(apierr.PermissionDenied,
			"That registration attempt belongs to a different account.")
	}
	a, err := s.accounts.ByID(ctx, accountID)
	if err != nil {
		return account.Passkey{}, apierr.New(apierr.NotFound, "No such account.")
	}

	pk, err := s.passkeys.FinishRegistration(*a, session, r)
	if err != nil {
		s.recordSecurityEvent(ctx, *a, "auth.passkey_registration_failed", "", err)
		return account.Passkey{}, apierr.New(apierr.InvalidArgument,
			"That authenticator response could not be verified.")
	}
	if name == "" {
		name = "Passkey"
	}
	pk.Name = name
	if err := s.accounts.AddPasskey(ctx, accountID, pk); err != nil {
		return account.Passkey{}, apierr.Wrap(apierr.Internal, "Could not save the passkey.", err)
	}
	s.recordSecurityEvent(ctx, *a, "auth.passkey_registered", "", nil)
	return pk, nil
}

// BeginPasskeyLogin starts an assertion for an email address.
//
// An unknown address still returns a ceremony. Refusing here would turn the
// endpoint into an account-existence oracle, which is exactly what the
// registration flow already goes to some length to avoid.
func (s *Service) BeginPasskeyLogin(ctx context.Context, email string) (Ceremony, error) {
	if err := s.passkeysReady(); err != nil {
		return Ceremony{}, err
	}
	a, err := s.accounts.ByEmail(ctx, email)
	if err != nil || len(a.Passkeys) == 0 {
		// A decoy ceremony: real and unguessable, but with nothing to match
		// against. The browser reports no usable credential, which is exactly
		// what a genuine account on the wrong device would report — so the
		// endpoint cannot be used to discover who has an account.
		opts, session, berr := s.passkeys.BeginDecoyLogin()
		if berr != nil {
			return Ceremony{}, apierr.Wrap(apierr.Internal, "Could not start sign-in.", berr)
		}
		id, cerr := s.challenges.Put(ctx, "", "login", session)
		if cerr != nil {
			return Ceremony{}, apierr.Wrap(apierr.Internal, "Could not start sign-in.", cerr)
		}
		return Ceremony{ChallengeID: id, Options: opts}, nil
	}
	opts, session, err := s.passkeys.BeginLogin(*a)
	if err != nil {
		return Ceremony{}, apierr.Wrap(apierr.Internal, "Could not start sign-in.", err)
	}
	id, err := s.challenges.Put(ctx, a.ID, "login", session)
	if err != nil {
		return Ceremony{}, apierr.Wrap(apierr.Internal, "Could not start sign-in.", err)
	}
	return Ceremony{ChallengeID: id, Options: opts}, nil
}

// FinishPasskeyLogin validates an assertion and issues a session.
//
// A passkey is a possession factor verified by the authenticator, usually with
// a biometric or PIN. It therefore satisfies MFA on its own, and a successful
// assertion produces an AUTHENTICATED session even for a privileged role —
// demanding a TOTP code as well would be asking for a weaker factor after a
// stronger one.
func (s *Service) FinishPasskeyLogin(
	ctx context.Context, challengeID, ua, ip string, r *http.Request,
) (LoginResult, error) {
	if err := s.passkeysReady(); err != nil {
		return LoginResult{}, err
	}
	deny := apierr.New(apierr.Unauthenticated, "That passkey could not be verified.")
	now := time.Now().UTC()

	accountID, session, err := s.challenges.Take(ctx, challengeID, "login")
	if err != nil {
		return LoginResult{}, apierr.New(apierr.InvalidArgument,
			"That sign-in attempt has expired. Start again.")
	}
	if accountID == "" {
		// The decoy path: no such account, but the timing and shape match.
		return LoginResult{}, deny
	}
	a, err := s.accounts.ByID(ctx, accountID)
	if err != nil {
		return LoginResult{}, deny
	}
	if err := a.CanSignIn(); err != nil {
		if errors.Is(err, account.ErrEmailNotVerified) {
			return LoginResult{}, apierr.New(apierr.PermissionDenied,
				"Confirm your email address before signing in.")
		}
		return LoginResult{}, apierr.New(apierr.PermissionDenied, "This account is not active.")
	}

	credID, signCount, err := s.passkeys.FinishLogin(*a, session, r)
	if err != nil {
		s.recordSecurityEvent(ctx, *a, "auth.passkey_login_failed", ip, err)
		return LoginResult{}, deny
	}

	stored, ok := a.FindPasskey(credID)
	if !ok {
		return LoginResult{}, deny
	}
	// A counter that has not advanced means the credential was copied off its
	// authenticator. Refusing the sign-in AND revoking every session is the
	// right response: the passkey can no longer be trusted.
	if err := account.CheckSignCount(signCount, stored.SignCount); err != nil {
		s.recordSecurityEvent(ctx, *a, "auth.passkey_cloned", ip, err)
		_ = s.accounts.BumpSessionEpoch(ctx, a.ID)
		_, _ = s.sessions.RevokeAllForAccount(ctx, a.ID)
		return LoginResult{}, apierr.New(apierr.PermissionDenied,
			"That passkey has been disabled for security reasons. Use another sign-in method.")
	}
	if err := s.accounts.UpdatePasskeyUse(ctx, a.ID, credID, signCount); err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not complete sign-in.", err)
	}

	iss, err := account.Issue(*a, account.StageAuthenticated, now, ua, ip)
	if err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not start a session.", err)
	}
	if err := s.sessions.Create(ctx, iss.Session); err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not start a session.", err)
	}
	s.recordSecurityEvent(ctx, *a, "auth.passkey_login_succeeded", ip, nil)

	return LoginResult{Token: iss.Token, ExpiresAt: iss.Session.ExpiresAt}, nil
}

// ListPasskeys returns an account's credentials.
func (s *Service) ListPasskeys(ctx context.Context, accountID string) ([]account.Passkey, error) {
	a, err := s.accounts.ByID(ctx, accountID)
	if err != nil {
		return nil, apierr.New(apierr.NotFound, "No such account.")
	}
	return a.Passkeys, nil
}

// RemovePasskey deletes a credential.
//
// A privileged account may not remove its LAST factor: MFA is mandatory for
// those roles, so allowing it would leave an account that cannot satisfy its
// own requirement and is locked out.
func (s *Service) RemovePasskey(ctx context.Context, accountID, passkeyID string) error {
	a, err := s.accounts.ByID(ctx, accountID)
	if err != nil {
		return apierr.New(apierr.NotFound, "No such account.")
	}
	if _, ok := a.FindPasskey(passkeyID); !ok {
		return apierr.New(apierr.NotFound, "No such passkey.")
	}
	if a.Role.RequiresMFA() && len(a.Passkeys) == 1 && a.TOTPSecret == "" {
		return apierr.New(apierr.PermissionDenied,
			"This is your only second factor and your role requires one. Add another first.")
	}
	if err := s.accounts.RemovePasskey(ctx, accountID, passkeyID); err != nil {
		return apierr.Wrap(apierr.Internal, "Could not remove the passkey.", err)
	}
	s.recordSecurityEvent(ctx, *a, "auth.passkey_removed", "", nil)
	return nil
}
