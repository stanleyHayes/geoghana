// Package account orchestrates registration, sign-in, MFA and sessions.
//
// The domain owns the rules; this package sequences them and talks to
// persistence. Anything that decides whether a caller MAY act belongs in the
// domain, so all three transports get the same answer.
package account

import (
	"context"
	"errors"
	"log/slog"
	"time"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/passkey"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/securityalert"
)

// Mailer delivers out-of-band tokens. It is an interface so the use cases can
// be tested without sending mail, and so a real provider can be swapped in
// without touching this package.
type Mailer interface {
	SendVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
}

type Service struct {
	accounts *mongoadapter.AccountRepo
	sessions *mongoadapter.SessionRepo
	tokens   *mongoadapter.OneTimeTokenRepo
	audit    *mongoadapter.AuditRepo
	mail     Mailer
	log      *slog.Logger
	issuer   string
	alerts   securityalert.Reporter

	passkeys   *passkey.RP
	challenges *mongoadapter.ChallengeRepo
}

func (s *Service) WithSecurityAlerts(reporter securityalert.Reporter) *Service {
	s.alerts = reporter
	return s
}

func NewService(
	accounts *mongoadapter.AccountRepo, sessions *mongoadapter.SessionRepo,
	tokens *mongoadapter.OneTimeTokenRepo, auditRepo *mongoadapter.AuditRepo,
	mail Mailer, log *slog.Logger, issuer string,
) *Service {
	return &Service{
		accounts: accounts, sessions: sessions, tokens: tokens,
		audit: auditRepo, mail: mail, log: log, issuer: issuer,
	}
}

// RegisterResult is what a caller learns after signing up.
//
// It deliberately carries no account id and no indication of whether the
// address was already taken: the response to "register" is identical either
// way, so the endpoint cannot be used to discover who has an account.
type RegisterResult struct {
	Message string
}

func (s *Service) Register(ctx context.Context, email, password string) (RegisterResult, error) {
	const sameEitherWay = "If that address can receive mail, a verification link is on its way."

	if err := account.ValidateEmail(email); err != nil {
		return RegisterResult{}, apierr.New(apierr.InvalidArgument, "That email address is not valid.")
	}
	if err := account.ValidatePassword(password); err != nil {
		// Password feedback IS returned — the user chose it, so it leaks
		// nothing about anyone else and withholding it just frustrates them.
		return RegisterResult{}, apierr.New(apierr.InvalidArgument, err.Error())
	}

	hash, err := account.HashPassword(password)
	if err != nil {
		return RegisterResult{}, apierr.Wrap(apierr.Internal, "Could not create the account.", err)
	}

	id, err := account.NewID("acc")
	if err != nil {
		return RegisterResult{}, apierr.Wrap(apierr.Internal, "Could not create the account.", err)
	}

	a := account.Account{
		ID: id, Email: account.NormalizeEmail(email),
		PasswordHash: hash, Role: account.RoleDeveloper,
	}
	if err := s.accounts.Create(ctx, a); err != nil {
		// An address already registered returns the SAME message as a fresh
		// one. The existing owner still gets no mail, so a stranger cannot
		// use registration to confirm who has an account.
		s.log.InfoContext(ctx, "registration for an existing address", "email", account.NormalizeEmail(email))
		return RegisterResult{Message: sameEitherWay}, nil
	}

	if err := s.issueAndSend(ctx, a, account.PurposeEmailVerification); err != nil {
		return RegisterResult{}, err
	}
	return RegisterResult{Message: sameEitherWay}, nil
}

func (s *Service) issueAndSend(ctx context.Context, a account.Account, p account.Purpose) error {
	iss, err := account.IssueToken(a.ID, p, time.Now().UTC())
	if err != nil {
		return apierr.Wrap(apierr.Internal, "Could not issue a token.", err)
	}
	if err := s.tokens.Create(ctx, iss.Token); err != nil {
		return apierr.Wrap(apierr.Internal, "Could not issue a token.", err)
	}
	if s.mail == nil {
		// No mailer configured (local development). The token is logged so a
		// developer can complete the flow — and it is logged at WARN with an
		// explicit note, so this can never be mistaken for production
		// behaviour or quietly left on.
		s.log.WarnContext(ctx, "NO MAILER CONFIGURED — token logged for local use only",
			"purpose", p, "email", a.Email, "token", iss.Value)
		return nil
	}
	switch p {
	case account.PurposePasswordReset:
		return s.mail.SendPasswordReset(ctx, a.Email, iss.Value)
	default:
		return s.mail.SendVerification(ctx, a.Email, iss.Value)
	}
}

// VerifyEmail consumes a verification token.
func (s *Service) VerifyEmail(ctx context.Context, value string) error {
	t, err := s.tokens.ByValue(ctx, value)
	if err != nil {
		return apierr.New(apierr.InvalidArgument, "That verification link is not valid.")
	}
	if err := t.Redeem(value, account.PurposeEmailVerification, time.Now().UTC()); err != nil {
		return apierr.New(apierr.InvalidArgument, "That verification link has expired or been used.")
	}
	// Consume FIRST: if marking the account fails afterwards the user can ask
	// for another link, whereas consuming last would leave a token replayable.
	if err := s.tokens.MarkUsed(ctx, t.ID); err != nil {
		return apierr.New(apierr.InvalidArgument, "That verification link has already been used.")
	}
	if err := s.accounts.MarkEmailVerified(ctx, t.AccountID); err != nil {
		return apierr.Wrap(apierr.Internal, "Could not verify the address.", err)
	}
	return nil
}

// LoginResult carries the session and what the caller must do next.
type LoginResult struct {
	Token string
	// MFARequired is true when the session is pending a second factor. The
	// token is real but can do nothing else until MFA completes.
	MFARequired bool
	// MFAEnrolmentRequired means a privileged account has no factor yet and
	// must enrol one before it can act.
	MFAEnrolmentRequired bool
	ExpiresAt            time.Time
}

// Login authenticates with email and password.
func (s *Service) Login(ctx context.Context, email, password, ua, ip string) (LoginResult, error) {
	deny := apierr.New(apierr.Unauthenticated, "Email or password is incorrect.")
	now := time.Now().UTC()

	a, err := s.accounts.ByEmail(ctx, email)
	if err != nil {
		// Hash against a dummy digest anyway. Returning early here would make
		// "no such account" measurably faster than "wrong password", which is
		// an enumeration oracle no matter how identical the error text is.
		_ = account.VerifyPassword(password, account.DummyHash)
		return LoginResult{}, deny
	}
	if a.PasswordHash == "" {
		_ = account.VerifyPassword(password, account.DummyHash)
		return LoginResult{}, deny
	}
	if err := account.VerifyPassword(password, a.PasswordHash); err != nil {
		s.recordSecurityEvent(ctx, *a, "auth.login_failed", ip, err)
		return LoginResult{}, deny
	}
	if err := a.CanSignIn(); err != nil {
		if errors.Is(err, account.ErrEmailNotVerified) {
			return LoginResult{}, apierr.New(apierr.PermissionDenied,
				"Confirm your email address before signing in.")
		}
		return LoginResult{}, apierr.New(apierr.PermissionDenied, "This account is not active.")
	}

	// A privileged role with no second factor cannot proceed. Spec §12.1
	// makes MFA mandatory, so this blocks rather than warns.
	if err := a.RequireMFAEnrolment(); err != nil {
		iss, ierr := account.Issue(*a, account.StagePendingMFA, now, ua, ip)
		if ierr != nil {
			return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not start a session.", ierr)
		}
		if err := s.sessions.Create(ctx, iss.Session); err != nil {
			return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not start a session.", err)
		}
		return LoginResult{
			Token: iss.Token, MFARequired: true, MFAEnrolmentRequired: true,
			ExpiresAt: iss.Session.ExpiresAt,
		}, nil
	}

	stage := account.StageAuthenticated
	if a.Role.RequiresMFA() {
		stage = account.StagePendingMFA
	}
	iss, err := account.Issue(*a, stage, now, ua, ip)
	if err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not start a session.", err)
	}
	if err := s.sessions.Create(ctx, iss.Session); err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not start a session.", err)
	}
	s.recordSecurityEvent(ctx, *a, "auth.login_succeeded", ip, nil)

	return LoginResult{
		Token:       iss.Token,
		MFARequired: stage == account.StagePendingMFA,
		ExpiresAt:   iss.Session.ExpiresAt,
	}, nil
}

// CompleteTOTP finishes a pending-MFA session with an authenticator code.
func (s *Service) CompleteTOTP(ctx context.Context, token, code, ua, ip string) (LoginResult, error) {
	deny := apierr.New(apierr.Unauthenticated, "That code is not valid.")
	now := time.Now().UTC()

	sess, a, err := s.resolve(ctx, token)
	if err != nil {
		return LoginResult{}, err
	}
	if sess.Stage != account.StagePendingMFA {
		return LoginResult{}, apierr.New(apierr.InvalidArgument, "This session is not awaiting a code.")
	}

	step, err := account.VerifyTOTP(a.TOTPSecret, code, now)
	if err != nil {
		s.recordSecurityEvent(ctx, a, "auth.mfa_failed", ip, err)
		return LoginResult{}, deny
	}
	last, err := s.accounts.TOTPLastStep(ctx, a.ID)
	if err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not verify the code.", err)
	}
	// A valid code is not enough: it stays valid for its whole window, so a
	// code seen over a shoulder or in a log would otherwise work again.
	if err := account.CheckTOTPReplay(step, last); err != nil {
		s.recordSecurityEvent(ctx, a, "auth.mfa_replayed", ip, err)
		return LoginResult{}, deny
	}
	if err := s.accounts.RecordTOTPStep(ctx, a.ID, step); err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not verify the code.", err)
	}

	return s.elevate(ctx, sess, a, now, ua, ip)
}

// RecoverMFA completes MFA with a single-use recovery code.
func (s *Service) RecoverMFA(ctx context.Context, token, code, ua, ip string) (LoginResult, error) {
	deny := apierr.New(apierr.Unauthenticated, "That recovery code is not valid.")
	now := time.Now().UTC()

	sess, a, err := s.resolve(ctx, token)
	if err != nil {
		return LoginResult{}, err
	}
	if sess.Stage != account.StagePendingMFA {
		return LoginResult{}, apierr.New(apierr.InvalidArgument, "This session is not awaiting a code.")
	}

	hashes, err := s.accounts.RecoveryCodes(ctx, a.ID)
	if err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not check the code.", err)
	}
	remaining, err := account.RedeemRecoveryCode(code, hashes)
	if err != nil {
		s.recordSecurityEvent(ctx, a, "auth.recovery_failed", ip, err)
		return LoginResult{}, deny
	}
	// Consumed immediately, so a code cannot be spent twice even if the rest
	// of the flow fails afterwards.
	if err := s.accounts.SetRecoveryCodes(ctx, a.ID, remaining); err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not consume the code.", err)
	}
	s.recordSecurityEvent(ctx, a, "auth.recovery_used", ip, nil)

	return s.elevate(ctx, sess, a, now, ua, ip)
}

func (s *Service) elevate(
	ctx context.Context, sess account.Session, a account.Account,
	now time.Time, ua, ip string,
) (LoginResult, error) {
	// A brand-new session, never a promotion of the pending one — the
	// fixation defence at the MFA boundary.
	iss, err := sess.Elevate(a, now, ua, ip)
	if err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not complete sign-in.", err)
	}
	if err := s.sessions.Create(ctx, iss.Session); err != nil {
		return LoginResult{}, apierr.Wrap(apierr.Internal, "Could not complete sign-in.", err)
	}
	// The pending session is spent.
	_ = s.sessions.Revoke(ctx, sess.ID)

	return LoginResult{Token: iss.Token, ExpiresAt: iss.Session.ExpiresAt}, nil
}

// Authenticate verifies a session token and rotates it.
//
// Rotation on every use is what makes a stolen token short-lived and its reuse
// detectable. The caller must send the returned token back to the client.
func (s *Service) Authenticate(ctx context.Context, token string) (account.Session, account.Account, string, error) {
	now := time.Now().UTC()

	sess, a, err := s.resolve(ctx, token)
	if err != nil {
		return account.Session{}, account.Account{}, "", err
	}
	if err := sess.Verify(a, now); err != nil {
		return account.Session{}, account.Account{}, "", mapSessionErr(err)
	}

	next, newToken, err := sess.Rotate(now)
	if err != nil {
		return account.Session{}, account.Account{}, "", apierr.Wrap(apierr.Internal, "Could not refresh the session.", err)
	}
	if err := s.sessions.Rotate(ctx, sess.ID, next); err != nil {
		return account.Session{}, account.Account{}, "", apierr.Wrap(apierr.Internal, "Could not refresh the session.", err)
	}
	return next, a, newToken, nil
}

// AuthenticateForEnrolment verifies a session that may still be pending MFA,
// but only for an account with no second factor yet.
//
// It is a narrow, explicit exception rather than a flag on Authenticate,
// because a boolean parameter on the main authentication path is exactly how
// an "allow unauthenticated" branch ends up reachable from somewhere it should
// not be.
func (s *Service) AuthenticateForEnrolment(
	ctx context.Context, token string,
) (account.Session, account.Account, string, error) {
	now := time.Now().UTC()

	sess, a, err := s.resolve(ctx, token)
	if err != nil {
		return account.Session{}, account.Account{}, "", err
	}

	if sess.Stage == account.StagePendingMFA {
		// Only while the account genuinely has nothing enrolled. Once a factor
		// exists, re-enrolment demands a fully authenticated session, or a
		// stolen password alone would let an attacker replace the owner's
		// authenticator.
		if a.MFAEnrolled() {
			return account.Session{}, account.Account{}, "", apierr.New(apierr.PermissionDenied,
				"Complete two-factor authentication before changing it.")
		}
		// Everything except the MFA stage still applies.
		if err := sess.VerifyPreMFA(a, now); err != nil {
			return account.Session{}, account.Account{}, "", mapSessionErr(err)
		}
	} else if err := sess.Verify(a, now); err != nil {
		return account.Session{}, account.Account{}, "", mapSessionErr(err)
	}

	next, newToken, err := sess.Rotate(now)
	if err != nil {
		return account.Session{}, account.Account{}, "", apierr.Wrap(apierr.Internal, "Could not refresh the session.", err)
	}
	if err := s.sessions.Rotate(ctx, sess.ID, next); err != nil {
		return account.Session{}, account.Account{}, "", apierr.Wrap(apierr.Internal, "Could not refresh the session.", err)
	}
	return next, a, newToken, nil
}

// resolve loads a session and its account, treating a replayed token as theft.
func (s *Service) resolve(ctx context.Context, token string) (account.Session, account.Account, error) {
	sess, err := s.sessions.ByToken(ctx, token)
	if errors.Is(err, mongoadapter.ErrSessionReplayed) {
		// A superseded token was presented. Either the real user raced
		// themselves or the chain leaked; we cannot tell, so we assume the
		// worst and end every session for the account.
		s.log.WarnContext(ctx, "superseded session token presented — revoking all sessions",
			"accountId", sess.AccountID, "sessionId", sess.ID)
		_ = s.accounts.BumpSessionEpoch(ctx, sess.AccountID)
		_, _ = s.sessions.RevokeAllForAccount(ctx, sess.AccountID)
		if a, aerr := s.accounts.ByID(ctx, sess.AccountID); aerr == nil {
			s.recordSecurityEvent(ctx, *a, "auth.session_token_replayed", sess.IP, err)
		}
		return account.Session{}, account.Account{}, apierr.New(apierr.Unauthenticated,
			"Your session ended for security reasons. Please sign in again.")
	}
	if err != nil {
		return account.Session{}, account.Account{}, apierr.New(apierr.Unauthenticated, "Not signed in.")
	}
	a, err := s.accounts.ByID(ctx, sess.AccountID)
	if err != nil {
		return account.Session{}, account.Account{}, apierr.New(apierr.Unauthenticated, "Not signed in.")
	}
	return sess, *a, nil
}

// Logout ends one session.
func (s *Service) Logout(ctx context.Context, token string) error {
	sess, err := s.sessions.ByToken(ctx, token)
	if err != nil {
		// Already gone: logout is idempotent, and reporting an error would
		// tell a caller whether a token was ever real.
		return nil
	}
	return s.sessions.Revoke(ctx, sess.ID)
}

// LogoutEverywhere ends every session for the account.
func (s *Service) LogoutEverywhere(ctx context.Context, accountID string) error {
	// The epoch bump is what actually revokes; marking the rows keeps a
	// session listing honest immediately afterwards.
	if err := s.accounts.BumpSessionEpoch(ctx, accountID); err != nil {
		return apierr.Wrap(apierr.Internal, "Could not sign out everywhere.", err)
	}
	if _, err := s.sessions.RevokeAllForAccount(ctx, accountID); err != nil {
		return apierr.Wrap(apierr.Internal, "Could not sign out everywhere.", err)
	}
	return nil
}

// EnrolTOTPResult is shown once.
type EnrolTOTPResult struct {
	Secret        string
	URI           string
	RecoveryCodes []string
}

// EnrolTOTP sets up an authenticator and issues recovery codes.
func (s *Service) EnrolTOTP(ctx context.Context, accountID string) (EnrolTOTPResult, error) {
	a, err := s.accounts.ByID(ctx, accountID)
	if err != nil {
		return EnrolTOTPResult{}, apierr.New(apierr.NotFound, "No such account.")
	}
	e, err := account.EnrolTOTP(s.issuer, a.Email)
	if err != nil {
		return EnrolTOTPResult{}, apierr.Wrap(apierr.Internal, "Could not set up the authenticator.", err)
	}
	plain, hashes, err := account.GenerateRecoveryCodes()
	if err != nil {
		return EnrolTOTPResult{}, apierr.Wrap(apierr.Internal, "Could not generate recovery codes.", err)
	}
	// Secret and codes are written together, so an account can never end up
	// with a second factor and no way to recover it.
	if err := s.accounts.SetTOTP(ctx, a.ID, e.Secret, hashes); err != nil {
		return EnrolTOTPResult{}, apierr.Wrap(apierr.Internal, "Could not save the authenticator.", err)
	}
	s.recordSecurityEvent(ctx, *a, "auth.mfa_enrolled", "", nil)

	return EnrolTOTPResult{Secret: e.Secret, URI: e.URI, RecoveryCodes: plain}, nil
}

// ActiveSessions lists what a user can revoke.
func (s *Service) ActiveSessions(ctx context.Context, accountID string) ([]account.Session, error) {
	out, err := s.sessions.ListActive(ctx, accountID)
	if err != nil {
		return nil, apierr.Wrap(apierr.Internal, "Could not list sessions.", err)
	}
	return out, nil
}

// recordSecurityEvent appends to the audit log. A failure to record is logged
// but never blocks the flow — an audit gap is bad, refusing a legitimate
// sign-in because of one is worse.
func (s *Service) recordSecurityEvent(ctx context.Context, a account.Account, action, ip string, cause error) {
	if cause != nil && a.Role.RequiresMFA() {
		securityalert.ReportBestEffort(ctx, s.alerts, s.log, securityalert.Event{
			Kind: securityalert.AdminAuthFailed, ActorID: a.ID, SourceIP: ip,
			Details: map[string]any{"action": action, "role": a.Role},
		})
	}
	if s.audit == nil {
		return
	}
	e, err := audit.New(
		audit.Actor{Kind: audit.ActorAdmin, ID: a.ID, Label: a.Email, IP: ip},
		audit.Action(action),
		audit.Target{Kind: "account", ID: a.ID, Label: a.Email},
	)
	if err != nil {
		return
	}
	if cause != nil {
		e = e.Failed(cause)
	}
	if _, err := s.audit.Append(ctx, e); err != nil {
		s.log.WarnContext(ctx, "security event not recorded", "action", action, "err", err)
	}
}

func mapSessionErr(err error) error {
	switch {
	case errors.Is(err, account.ErrMFANotSatisfied):
		return apierr.New(apierr.PermissionDenied, "Complete two-factor authentication to continue.")
	case errors.Is(err, account.ErrAccountDisabled):
		return apierr.New(apierr.PermissionDenied, "This account is not active.")
	default:
		return apierr.New(apierr.Unauthenticated, "Your session has ended. Please sign in again.")
	}
}
