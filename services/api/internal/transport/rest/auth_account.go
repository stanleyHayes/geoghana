package rest

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	appaccount "github.com/ghanageo/ghanageo/services/api/internal/app/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Account authentication endpoints (Spec §12.1, story GEO-9.2).
//
// The session token travels in a cookie rather than a JSON body so that
// browser JavaScript cannot read it: an XSS bug then cannot exfiltrate a live
// session. That choice brings CSRF into scope; SameSite=Lax and the router's
// explicit origin check protect every cookie-authenticated mutation.

const sessionCookie = "gg_session"

// setSessionCookie writes the session token.
//
// HttpOnly so script cannot read it, SameSite=Lax so another origin cannot
// silently POST with it, Secure whenever the request is not plain local HTTP,
// and Path=/ so it accompanies every API call. MaxAge tracks the session's own
// absolute expiry rather than being a fixed number, so the cookie and the
// server-side session cannot disagree about when it ends.
func (h *Handler) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   SecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
	})
}

func (h *Handler) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/",
		HttpOnly: true, Secure: SecureRequest(r),
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

// isSecureRequest reports whether the connection is TLS, honouring a proxy's
// X-Forwarded-Proto. Local plain HTTP must NOT set Secure, or the cookie is
// silently dropped and nobody can sign in during development.
// SecureRequest reports the effective transport security after trusted-edge
// normalization. It is exported so the process composition can be regression
// tested without duplicating the cookie policy.
func SecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func sessionTokenFrom(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

func decodeJSON(r *http.Request, dst any) error {
	// A bounded reader: an unbounded JSON body on an unauthenticated endpoint
	// is a trivial memory-exhaustion vector.
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apierr.New(apierr.InvalidArgument, "The request body could not be read.")
	}
	return nil
}

func (h *Handler) authRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := h.accounts.Register(r.Context(), body.Email, body.Password)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"message": res.Message})
}

func (h *Handler) authVerifyEmail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := h.accounts.VerifyEmail(r.Context(), body.Token); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Email address confirmed. You can sign in now."})
}

func (h *Handler) authRequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res := h.accounts.RequestPasswordReset(r.Context(), body.Email, clientIP(r))
	writeJSON(w, http.StatusAccepted, map[string]any{"message": res.Message})
}

func (h *Handler) authConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := h.accounts.ConfirmPasswordReset(r.Context(), body.Token, body.Password, clientIP(r)); err != nil {
		writeErr(w, r, err)
		return
	}
	h.clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"message": "Password changed. Sign in again on every device."})
}

func (h *Handler) authLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := h.accounts.Login(r.Context(), body.Email, body.Password,
		r.UserAgent(), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h.setSessionCookie(w, r, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"mfaRequired":          res.MFARequired,
		"mfaEnrolmentRequired": res.MFAEnrolmentRequired,
		"expiresAt":            res.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) authTOTP(w http.ResponseWriter, r *http.Request) {
	h.completeMFA(w, r, false)
}

func (h *Handler) authRecover(w http.ResponseWriter, r *http.Request) {
	h.completeMFA(w, r, true)
}

func (h *Handler) completeMFA(w http.ResponseWriter, r *http.Request, recovery bool) {
	var body struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	token := sessionTokenFrom(r)
	if token == "" {
		writeErr(w, r, apierr.New(apierr.Unauthenticated, "Sign in first."))
		return
	}

	var (
		res appaccount.LoginResult
		err error
	)
	if recovery {
		res, err = h.accounts.RecoverMFA(r.Context(), token, body.Code, r.UserAgent(), clientIP(r))
	} else {
		res, err = h.accounts.CompleteTOTP(r.Context(), token, body.Code, r.UserAgent(), clientIP(r))
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	// A NEW cookie: the pre-MFA token is dead, which is the fixation defence.
	h.setSessionCookie(w, r, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"expiresAt": res.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) authLogout(w http.ResponseWriter, r *http.Request) {
	if token := sessionTokenFrom(r); token != "" {
		_ = h.accounts.Logout(r.Context(), token)
	}
	// The cookie is cleared regardless, so a client is never left holding one
	// it believes is live.
	h.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) authLogoutAll(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if err := h.accounts.LogoutEverywhere(r.Context(), sess.AccountID); err != nil {
		writeErr(w, r, err)
		return
	}
	h.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) authSession(w http.ResponseWriter, r *http.Request) {
	sess, a, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"accountId":   a.ID,
		"email":       a.Email,
		"role":        string(a.Role),
		"mfaEnrolled": a.MFAEnrolled(),
		"expiresAt":   sess.ExpiresAt.Format(time.RFC3339),
	}})
}

func (h *Handler) authSessions(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	list, err := h.accounts.ActiveSessions(r.Context(), sess.AccountID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, s := range list {
		out = append(out, map[string]any{
			"id": s.ID, "current": s.ID == sess.ID,
			"userAgent": s.UserAgent, "ip": s.IP,
			"lastUsedAt": s.LastUsedAt.Format(time.RFC3339),
			"expiresAt":  s.ExpiresAt.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) authEnrolTOTP(w http.ResponseWriter, r *http.Request) {
	// Enrolment is the ONE action a pending-MFA session may take.
	//
	// Without this a privileged account that has no second factor is locked
	// out permanently: it cannot enrol because enrolling needs an
	// authenticated session, and it cannot authenticate because it has no
	// factor. Requiring a first factor already proves possession of the
	// password, which is what makes granting this narrow exception safe.
	sess, _, ok := h.requireSessionAllowingEnrolment(w, r)
	if !ok {
		return
	}
	res, err := h.accounts.EnrolTOTP(r.Context(), sess.AccountID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	// Shown exactly once. The response is explicitly not cached anywhere.
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"secret":        res.Secret,
		"uri":           res.URI,
		"recoveryCodes": res.RecoveryCodes,
		"notice":        "These recovery codes are shown once. Store them somewhere safe.",
	}})
}

// requireSessionAllowingEnrolment accepts a session that has passed a first
// factor but not yet MFA, and ONLY when the account still has no factor
// enrolled. A pending session on an account that already has an authenticator
// is rejected — otherwise anyone with a stolen password could re-enrol their
// own device and lock the owner out.
func (h *Handler) requireSessionAllowingEnrolment(
	w http.ResponseWriter, r *http.Request,
) (account.Session, account.Account, bool) {
	if h.accounts == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Accounts are not configured."))
		return account.Session{}, account.Account{}, false
	}
	token := sessionTokenFrom(r)
	if token == "" {
		writeErr(w, r, apierr.New(apierr.Unauthenticated, "Not signed in."))
		return account.Session{}, account.Account{}, false
	}
	sess, a, newToken, err := h.accounts.AuthenticateForEnrolment(r.Context(), token)
	if err != nil {
		h.clearSessionCookie(w, r)
		writeErr(w, r, err)
		return account.Session{}, account.Account{}, false
	}
	if newToken != "" {
		h.setSessionCookie(w, r, newToken, sess.ExpiresAt)
	}
	return sess, a, true
}

// requireSession authenticates the cookie and refreshes it only when the
// rotation interval is due. This keeps parallel dashboard requests on one
// chain instead of making ordinary browser concurrency look like theft.
func (h *Handler) requireSession(
	w http.ResponseWriter, r *http.Request,
) (account.Session, account.Account, bool) {
	if h.accounts == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Accounts are not configured."))
		return account.Session{}, account.Account{}, false
	}
	token := sessionTokenFrom(r)
	if token == "" {
		writeErr(w, r, apierr.New(apierr.Unauthenticated, "Not signed in."))
		return account.Session{}, account.Account{}, false
	}
	sess, a, newToken, err := h.accounts.Authenticate(r.Context(), token)
	if err != nil {
		// The cookie is cleared on any failure, so a client stops presenting a
		// token the server has already rejected.
		h.clearSessionCookie(w, r)
		writeErr(w, r, err)
		return account.Session{}, account.Account{}, false
	}
	if newToken != "" {
		h.setSessionCookie(w, r, newToken, sess.ExpiresAt)
	}
	return sess, a, true
}

func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		if i := strings.IndexByte(f, ','); i > 0 {
			return strings.TrimSpace(f[:i])
		}
		return strings.TrimSpace(f)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		return host[:i]
	}
	return host
}
