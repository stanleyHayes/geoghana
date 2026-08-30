package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Passkey (WebAuthn) endpoints (Spec §12.1, story GEO-9.2).
//
// Each ceremony is two calls: begin returns the options the browser passes to
// navigator.credentials, finish posts the authenticator's response back. The
// challenge stays server-side and is referenced by an opaque id — round-
// tripping it through the client would let a caller choose its own challenge.

func (h *Handler) passkeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	// Enrolment is allowed from a pending-MFA session for the same reason
	// TOTP enrolment is: otherwise a privileged account with no factor can
	// never acquire one.
	sess, _, ok := h.requireSessionAllowingEnrolment(w, r)
	if !ok {
		return
	}
	c, err := h.accounts.BeginPasskeyRegistration(r.Context(), sess.AccountID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"challengeId": c.ChallengeID,
		"options":     c.Options,
	})
}

func (h *Handler) passkeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := h.requireSessionAllowingEnrolment(w, r)
	if !ok {
		return
	}
	challengeID := r.URL.Query().Get("challengeId")
	name := r.URL.Query().Get("name")
	if challengeID == "" {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "challengeId is required."))
		return
	}
	// The body is the raw authenticator response, which the library parses
	// straight off the request — so it is passed through untouched rather
	// than decoded and re-encoded here.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	pk, err := h.accounts.FinishPasskeyRegistration(r.Context(), sess.AccountID, challengeID, name, r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{
		"id": pk.ID, "name": pk.Name, "backedUp": pk.BackedUp,
		"addedAt": pk.AddedAt.Format(time.RFC3339),
	}})
}

func (h *Handler) passkeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	c, err := h.accounts.BeginPasskeyLogin(r.Context(), body.Email)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"challengeId": c.ChallengeID,
		"options":     c.Options,
	})
}

func (h *Handler) passkeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	challengeID := r.URL.Query().Get("challengeId")
	if challengeID == "" {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "challengeId is required."))
		return
	}
	// WebAuthn consumes the raw request directly, so decodeJSON's normal body
	// cap does not apply. Bound public assertion payloads before parsing.
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	res, err := h.accounts.FinishPasskeyLogin(r.Context(), challengeID, r.UserAgent(), clientIP(r), r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	// A passkey satisfies MFA on its own, so this session is authenticated
	// outright — there is no pending stage to complete.
	h.setSessionCookie(w, r, res.Token, res.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"expiresAt": res.ExpiresAt.Format(time.RFC3339),
	})
}

func (h *Handler) passkeyList(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	keys, err := h.accounts.ListPasskeys(r.Context(), sess.AccountID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, k := range keys {
		item := map[string]any{
			"id": k.ID, "name": k.Name, "backedUp": k.BackedUp,
			"addedAt": k.AddedAt.Format(time.RFC3339),
		}
		if !k.LastUsed.IsZero() {
			item["lastUsed"] = k.LastUsed.Format(time.RFC3339)
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) passkeyDelete(w http.ResponseWriter, r *http.Request) {
	sess, _, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if err := h.accounts.RemovePasskey(r.Context(), sess.AccountID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Passkey removed."})
}
