package rest

import (
	"net/http"
	"time"

	appfairuse "github.com/ghanageo/ghanageo/services/api/internal/app/fairuse"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/go-chi/chi/v5/middleware"
)

type allowanceJSON struct {
	BurstUnits      int     `json:"burstUnits"`
	RefillPerSecond float64 `json:"refillPerSecond"`
	WindowSeconds   int64   `json:"windowSeconds"`
}
type policyJSON struct {
	ID            string         `json:"id"`
	Revision      int64          `json:"revision"`
	Anonymous     allowanceJSON  `json:"anonymous"`
	Authenticated allowanceJSON  `json:"authenticated"`
	Sandbox       allowanceJSON  `json:"sandbox"`
	CostCeilings  map[string]int `json:"costCeilings"`
	Reason        string         `json:"reason"`
	ActorID       string         `json:"actorId"`
	CreatedAt     time.Time      `json:"createdAt"`
	EffectiveFrom time.Time      `json:"effectiveFrom"`
}

func toAllowance(a identity.Allowance) allowanceJSON {
	return allowanceJSON{a.BurstUnits, a.RefillPerSecond, int64(a.Window.Seconds())}
}
func fromAllowance(a allowanceJSON) identity.Allowance {
	return identity.Allowance{BurstUnits: a.BurstUnits, RefillPerSecond: a.RefillPerSecond, Window: time.Duration(a.WindowSeconds) * time.Second}
}
func toCeilings(v map[identity.CostClass]int) map[string]int {
	o := map[string]int{}
	for k, n := range v {
		o[string(k)] = n
	}
	return o
}
func fromCeilings(v map[string]int) map[identity.CostClass]int {
	o := map[identity.CostClass]int{}
	for k, n := range v {
		o[identity.CostClass(k)] = n
	}
	return o
}
func toPolicy(p identity.FairUsePolicy) policyJSON {
	return policyJSON{ID: p.ID, Revision: p.Revision, Anonymous: toAllowance(p.Anonymous), Authenticated: toAllowance(p.Authenticated), Sandbox: toAllowance(p.Sandbox), CostCeilings: toCeilings(p.CostCeilings), Reason: p.Reason, ActorID: p.ActorID, CreatedAt: p.CreatedAt, EffectiveFrom: p.EffectiveFrom}
}

func (h *Handler) fairUseActor(w http.ResponseWriter, r *http.Request) (appfairuse.Actor, bool) {
	_, a, ok := h.requireSession(w, r)
	if !ok {
		return appfairuse.Actor{}, false
	}
	if h.fairUse == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Fair-use policy management is not configured."))
		return appfairuse.Actor{}, false
	}
	return appfairuse.Actor{ID: a.ID, Email: a.Email, Role: a.Role, IP: clientIP(r), RequestID: middleware.GetReqID(r.Context())}, true
}

func (h *Handler) adminFairUsePolicy(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireSteward(w, r, account.PermManageFairUse); !ok {
		return
	}
	if h.fairUse == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Fair-use policy management is not configured."))
		return
	}
	p, err := h.fairUse.CurrentPolicy(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": toPolicy(p)})
}

func (h *Handler) adminAppendFairUsePolicy(w http.ResponseWriter, r *http.Request) {
	a, ok := h.fairUseActor(w, r)
	if !ok {
		return
	}
	var b policyJSON
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, r, err)
		return
	}
	p := identity.FairUsePolicy{Revision: b.Revision, Anonymous: fromAllowance(b.Anonymous), Authenticated: fromAllowance(b.Authenticated), Sandbox: fromAllowance(b.Sandbox), CostCeilings: fromCeilings(b.CostCeilings), Reason: b.Reason, EffectiveFrom: b.EffectiveFrom}
	created, err := h.fairUse.AppendPolicy(r.Context(), a, p)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": toPolicy(created)})
}

func (h *Handler) adminAppendFairUseOverride(w http.ResponseWriter, r *http.Request) {
	a, ok := h.fairUseActor(w, r)
	if !ok {
		return
	}
	var b struct {
		ID            string         `json:"id"`
		ApplicationID string         `json:"applicationId"`
		Allowance     allowanceJSON  `json:"allowance"`
		CostCeilings  map[string]int `json:"costCeilings"`
		Enabled       bool           `json:"enabled"`
		Reason        string         `json:"reason"`
		ExpiresAt     time.Time      `json:"expiresAt"`
		SupersedesID  string         `json:"supersedesId"`
	}
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, r, err)
		return
	}
	o := identity.FairUseOverride{ID: b.ID, ApplicationID: b.ApplicationID, Allowance: fromAllowance(b.Allowance), CostCeilings: fromCeilings(b.CostCeilings), Enabled: b.Enabled, Reason: b.Reason, ExpiresAt: b.ExpiresAt, SupersedesID: b.SupersedesID}
	created, err := h.fairUse.AppendOverride(r.Context(), a, o)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"id": created.ID, "applicationId": created.ApplicationID, "enabled": created.Enabled, "expiresAt": created.ExpiresAt}})
}
