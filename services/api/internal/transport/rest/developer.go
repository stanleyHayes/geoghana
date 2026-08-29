package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	appdeveloper "github.com/ghanageo/ghanageo/services/api/internal/app/developer"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

func (h *Handler) developerOrganizations(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	items, err := h.developer.Organizations(r.Context(), account.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, organizationResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) developerCreateOrganization(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	org, err := h.developer.CreateOrganization(r.Context(), account.ID, account.Email, body.Name, middleware.GetReqID(r.Context()), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": organizationResponse(org)})
}

func (h *Handler) developerInvitations(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	items, err := h.developer.Invitations(r.Context(), account.ID, chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, v := range items {
		out = append(out, invitationResponse(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) developerInviteMember(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	var body struct {
		Email string                    `json:"email"`
		Role  identity.OrganizationRole `json:"role"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.developer.InviteMember(r.Context(), account.ID, chi.URLParam(r, "orgId"), body.Email, body.Role, middleware.GetReqID(r.Context()), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	data := invitationResponse(created.Invitation)
	data["token"] = created.Token
	data["notice"] = "Share this invitation securely. Its token is shown once and expires in seven days."
	writeJSON(w, http.StatusCreated, map[string]any{"data": data})
}

func (h *Handler) developerAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	org, err := h.developer.AcceptInvitation(r.Context(), account.ID, account.Email, body.Token, middleware.GetReqID(r.Context()), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": organizationResponse(org)})
}

func (h *Handler) developerRevokeInvitation(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	if err := h.developer.RevokeInvitation(r.Context(), account.ID, chi.URLParam(r, "orgId"), chi.URLParam(r, "inviteId")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Invitation revoked."})
}

func (h *Handler) developerTransferOwnership(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	var body struct {
		AccountID string `json:"accountId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := h.developer.TransferOwnership(r.Context(), account.ID, chi.URLParam(r, "orgId"), body.AccountID, middleware.GetReqID(r.Context()), clientIP(r)); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Organization ownership transferred."})
}

func (h *Handler) developerApplications(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	items, err := h.developer.Applications(r.Context(), account.ID, chi.URLParam(r, "orgId"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, applicationResponse(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) developerCreateApplication(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	var body struct {
		Name         string                 `json:"name"`
		Description  string                 `json:"description"`
		Environments []identity.Environment `json:"environments"`
		Domains      []string               `json:"domains"`
		CallbackURL  string                 `json:"callbackUrl"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	app, err := h.developer.CreateApplication(r.Context(), account.ID, chi.URLParam(r, "orgId"), appdeveloper.CreateApplicationInput{Name: body.Name, Description: body.Description, Environments: body.Environments, Domains: body.Domains, CallbackURL: body.CallbackURL}, middleware.GetReqID(r.Context()), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": applicationResponse(app)})
}

func (h *Handler) developerKeys(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	keys, err := h.developer.Keys(r.Context(), account.ID, chi.URLParam(r, "orgId"), chi.URLParam(r, "appId"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, keyResponse(key))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

type keyRequest struct {
	Name           string               `json:"name"`
	Class          identity.KeyClass    `json:"class"`
	Environment    identity.Environment `json:"environment"`
	Scopes         []string             `json:"scopes"`
	AllowedOrigins []string             `json:"allowedOrigins"`
	AllowedIPs     []string             `json:"allowedIps"`
	ExpiresAt      string               `json:"expiresAt"`
}

func (h *Handler) developerCreateKey(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	var body keyRequest
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	in, err := parseKeyRequest(body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	created, err := h.developer.CreateKey(r.Context(), account.ID, chi.URLParam(r, "orgId"), chi.URLParam(r, "appId"), in, middleware.GetReqID(r.Context()), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"key": keyResponse(created.Key), "secret": created.Secret, "notice": "Copy this key now. The secret will not be shown again."}})
}

func (h *Handler) developerRotateKey(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	created, err := h.developer.RotateKey(r.Context(), account.ID, chi.URLParam(r, "orgId"), chi.URLParam(r, "appId"), chi.URLParam(r, "keyId"), middleware.GetReqID(r.Context()), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"key": keyResponse(created.Key), "secret": created.Secret, "notice": "Copy this replacement now. The previous key is revoked."}})
}

func (h *Handler) developerRevokeKey(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	err := h.developer.RevokeKey(r.Context(), account.ID, chi.URLParam(r, "orgId"), chi.URLParam(r, "appId"), chi.URLParam(r, "keyId"), middleware.GetReqID(r.Context()), clientIP(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "API key revoked."})
}

func parseKeyRequest(body keyRequest) (appdeveloper.CreateKeyInput, error) {
	scopes := make([]identity.Scope, 0, len(body.Scopes))
	for _, raw := range body.Scopes {
		scope, err := identity.ParseScope(raw)
		if err != nil {
			return appdeveloper.CreateKeyInput{}, apierr.Wrap(apierr.InvalidArgument, err.Error(), err)
		}
		scopes = append(scopes, scope)
	}
	var expires *time.Time
	if body.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, body.ExpiresAt)
		if err != nil {
			return appdeveloper.CreateKeyInput{}, apierr.New(apierr.InvalidArgument, "expiresAt must be RFC3339.")
		}
		expires = &parsed
	}
	return appdeveloper.CreateKeyInput{Name: body.Name, Class: body.Class, Environment: body.Environment, Scopes: scopes, AllowedOrigins: body.AllowedOrigins, AllowedIPs: body.AllowedIPs, ExpiresAt: expires}, nil
}

func keyResponse(key identity.APIKey) map[string]any {
	return map[string]any{
		"id": key.ID, "applicationId": key.ApplicationID, "organizationId": key.OrganizationID,
		"name": key.Name, "class": key.Class, "environment": key.Environment, "prefix": key.Prefix,
		"scopes": key.Scopes, "allowedOrigins": key.AllowedOrigins, "allowedIps": key.AllowedIPs,
		"createdAt": key.CreatedAt, "expiresAt": key.ExpiresAt, "lastUsedAt": key.LastUsedAt, "revokedAt": key.RevokedAt,
	}
}

func organizationResponse(org identity.Organization) map[string]any {
	members := make([]map[string]any, 0, len(org.Members))
	for _, member := range org.Members {
		members = append(members, map[string]any{
			"accountId": member.AccountID, "email": member.Email, "role": member.Role, "joinedAt": member.JoinedAt,
		})
	}
	return map[string]any{
		"id": org.ID, "name": org.Name, "ownerId": org.OwnerID, "members": members, "createdAt": org.CreatedAt,
	}
}
func applicationResponse(app identity.Application) map[string]any {
	return map[string]any{
		"id": app.ID, "organizationId": app.OrganizationID, "name": app.Name,
		"description": app.Description, "environments": app.Environments, "domains": app.Domains, "callbackUrl": app.CallbackURL, "plan": app.Plan, "createdAt": app.CreatedAt,
	}
}
func invitationResponse(v identity.OrganizationInvitation) map[string]any {
	return map[string]any{"id": v.ID, "organizationId": v.OrganizationID, "email": v.Email, "role": v.Role, "status": v.Status, "invitedBy": v.InvitedBy, "createdAt": v.CreatedAt, "expiresAt": v.ExpiresAt, "acceptedAt": v.AcceptedAt}
}
