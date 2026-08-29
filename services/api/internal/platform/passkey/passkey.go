// Package passkey wraps the WebAuthn relying party.
//
// It exists so the rest of the codebase never imports the WebAuthn library
// directly: registration and assertion are the only two operations anyone
// needs, and keeping the library behind them means an upgrade touches one
// file rather than every caller.
package passkey

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
)

// RP is the relying party.
type RP struct{ w *webauthn.WebAuthn }

// Config describes this deployment to the authenticator.
type Config struct {
	// RPID is the domain the credential is bound to, without scheme or port.
	// A credential registered for one RPID cannot be used on another, which
	// is what makes passkeys phishing-resistant.
	RPID string
	// DisplayName is shown by the authenticator when prompting.
	DisplayName string
	// Origins must be fully qualified and are checked on every assertion.
	Origins []string
}

func New(c Config) (*RP, error) {
	w, err := webauthn.New(&webauthn.Config{
		RPID:          c.RPID,
		RPDisplayName: c.DisplayName,
		RPOrigins:     c.Origins,
		Timeouts: webauthn.TimeoutsConfig{
			Registration: webauthn.TimeoutConfig{
				Enforce: true, Timeout: 5 * time.Minute, TimeoutUVD: 5 * time.Minute,
			},
			Login: webauthn.TimeoutConfig{
				Enforce: true, Timeout: 3 * time.Minute, TimeoutUVD: 3 * time.Minute,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("configure webauthn: %w", err)
	}
	return &RP{w: w}, nil
}

// user adapts a domain account to the library's User interface.
type user struct {
	a account.Account
}

// WebAuthnID must be a stable, opaque handle — and NOT the email.
//
// The spec is explicit that authorization decisions are made on this value, so
// it must not change when a user changes their address, and it must not leak
// personal data to an authenticator that may display or sync it.
func (u user) WebAuthnID() []byte { return []byte(u.a.ID) }

// WebAuthnName is what the authenticator shows in its credential list.
func (u user) WebAuthnName() string        { return u.a.Email }
func (u user) WebAuthnDisplayName() string { return u.a.Email }

func (u user) WebAuthnCredentials() []webauthn.Credential {
	out := make([]webauthn.Credential, 0, len(u.a.Passkeys))
	for _, p := range u.a.Passkeys {
		var c webauthn.Credential
		if err := json.Unmarshal(p.Credential, &c); err != nil {
			// A credential we cannot decode is skipped rather than fatal: one
			// corrupt row must not lock a user out of their other passkeys.
			continue
		}
		out = append(out, c)
	}
	return out
}

// BeginRegistration starts enrolling a new passkey.
//
// The returned session data must be stored server-side and handed back to
// FinishRegistration. It carries the challenge, and a challenge a client
// could choose for itself would defeat the whole protocol.
func (r *RP) BeginRegistration(a account.Account) (options any, session []byte, err error) {
	// Excluding existing credentials stops the same authenticator being
	// enrolled twice, which would otherwise look like two passkeys that
	// mysteriously fail together when the device is lost.
	creation, sd, err := r.w.BeginRegistration(user{a},
		webauthn.WithExclusions(credentialDescriptors(user{a})),
		webauthn.WithResidentKeyRequirement(protocolResidentKeyPreferred()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("begin passkey registration: %w", err)
	}
	raw, err := json.Marshal(sd)
	if err != nil {
		return nil, nil, fmt.Errorf("encode webauthn session: %w", err)
	}
	return creation, raw, nil
}

// FinishRegistration validates the authenticator's response.
func (r *RP) FinishRegistration(
	a account.Account, session []byte, req *http.Request,
) (account.Passkey, error) {
	var sd webauthn.SessionData
	if err := json.Unmarshal(session, &sd); err != nil {
		return account.Passkey{}, fmt.Errorf("decode webauthn session: %w", err)
	}
	cred, err := r.w.FinishRegistration(user{a}, sd, req)
	if err != nil {
		return account.Passkey{}, fmt.Errorf("finish passkey registration: %w", err)
	}
	raw, err := json.Marshal(cred)
	if err != nil {
		return account.Passkey{}, fmt.Errorf("encode credential: %w", err)
	}
	return account.Passkey{
		ID:         base64.RawURLEncoding.EncodeToString(cred.ID),
		Credential: raw,
		SignCount:  cred.Authenticator.SignCount,
		BackedUp:   cred.Flags.BackupEligible,
		AddedAt:    time.Now().UTC(),
	}, nil
}

// BeginLogin starts an assertion for a known account.
func (r *RP) BeginLogin(a account.Account) (options any, session []byte, err error) {
	assertion, sd, err := r.w.BeginLogin(user{a})
	if err != nil {
		return nil, nil, fmt.Errorf("begin passkey login: %w", err)
	}
	raw, err := json.Marshal(sd)
	if err != nil {
		return nil, nil, fmt.Errorf("encode webauthn session: %w", err)
	}
	return assertion, raw, nil
}

// BeginDecoyLogin produces a well-formed ceremony for no particular user.
//
// It exists so an unknown address is answered identically to a known one.
// BeginLogin refuses a user with no credentials, so using it for the decoy
// would turn the endpoint into an account-existence oracle — the exact thing
// the registration flow already works to avoid. A discoverable-login challenge
// is real, unguessable, and simply has nothing to match against.
func (r *RP) BeginDecoyLogin() (options any, session []byte, err error) {
	assertion, sd, err := r.w.BeginDiscoverableLogin()
	if err != nil {
		return nil, nil, fmt.Errorf("begin decoy login: %w", err)
	}
	raw, err := json.Marshal(sd)
	if err != nil {
		return nil, nil, fmt.Errorf("encode webauthn session: %w", err)
	}
	return assertion, raw, nil
}

// FinishLogin validates an assertion and returns the credential used.
func (r *RP) FinishLogin(
	a account.Account, session []byte, req *http.Request,
) (credentialID string, signCount uint32, err error) {
	var sd webauthn.SessionData
	if err := json.Unmarshal(session, &sd); err != nil {
		return "", 0, fmt.Errorf("decode webauthn session: %w", err)
	}
	cred, err := r.w.FinishLogin(user{a}, sd, req)
	if err != nil {
		return "", 0, fmt.Errorf("finish passkey login: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(cred.ID), cred.Authenticator.SignCount, nil
}
