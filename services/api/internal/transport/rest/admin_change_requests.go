package rest

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	app "github.com/ghanageo/ghanageo/services/api/internal/app/changerequest"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	domain "github.com/ghanageo/ghanageo/services/api/internal/domain/changerequest"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

type changeEvidenceDTO struct {
	SourceID                  string   `json:"sourceId,omitempty"`
	SourceRecordIDs           []string `json:"sourceRecordIds,omitempty"`
	DuplicateCandidateIDs     []string `json:"duplicateCandidateIds,omitempty"`
	ReconciliationConflictIDs []string `json:"reconciliationConflictIds,omitempty"`
	References                []string `json:"references,omitempty"`
}

type changeSnapshotDTO struct {
	Before map[string]any `json:"before"`
	After  map[string]any `json:"after"`
	Digest string         `json:"digest"`
}

type changeRequestDTO struct {
	ID            string              `json:"id"`
	Target        changeTargetDTO     `json:"target"`
	ProposedPatch map[string]any      `json:"proposedPatch"`
	Snapshot      changeSnapshotDTO   `json:"snapshot"`
	Evidence      changeEvidenceDTO   `json:"evidence"`
	SubmitterID   string              `json:"submitterId"`
	ReviewerID    string              `json:"reviewerId,omitempty"`
	State         domain.State        `json:"state"`
	Comments      []changeCommentDTO  `json:"comments"`
	History       []changeHistoryDTO  `json:"history"`
	Revisions     []changeRevisionDTO `json:"revisions"`
	Version       int64               `json:"version"`
	CreatedAt     time.Time           `json:"createdAt"`
	UpdatedAt     time.Time           `json:"updatedAt"`
}

type changeTargetDTO struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type changeCommentDTO struct {
	ID        string    `json:"id"`
	AuthorID  string    `json:"authorId"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}

type changeHistoryDTO struct {
	ID        string       `json:"id"`
	ActorID   string       `json:"actorId"`
	Comment   string       `json:"comment,omitempty"`
	From      domain.State `json:"from,omitempty"`
	To        domain.State `json:"to"`
	CreatedAt time.Time    `json:"createdAt"`
}

type changeRevisionDTO struct {
	Number        int64             `json:"number"`
	ProposedPatch map[string]any    `json:"proposedPatch"`
	Snapshot      changeSnapshotDTO `json:"snapshot"`
	Evidence      changeEvidenceDTO `json:"evidence"`
	ActorID       string            `json:"actorId"`
	CreatedAt     time.Time         `json:"createdAt"`
}

func evidenceDTO(e domain.Evidence) changeEvidenceDTO {
	return changeEvidenceDTO{e.SourceID, e.SourceRecordIDs, e.DuplicateCandidateIDs, e.ReconciliationConflictIDs, e.References}
}

func changeRequestOut(c domain.ChangeRequest) changeRequestDTO {
	out := changeRequestDTO{ID: c.ID, Target: changeTargetDTO{Kind: c.Target.Kind, ID: c.Target.ID}, ProposedPatch: c.ProposedPatch,
		Snapshot: changeSnapshotDTO{c.Snapshot.Before(), c.Snapshot.After(), c.Snapshot.Digest}, Evidence: evidenceDTO(c.Evidence),
		SubmitterID: c.SubmitterID, ReviewerID: c.ReviewerID, State: c.State, Version: c.Version,
		CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt, Comments: []changeCommentDTO{}, History: []changeHistoryDTO{}, Revisions: []changeRevisionDTO{}}
	for _, x := range c.Comments {
		out.Comments = append(out.Comments, changeCommentDTO{x.ID, x.AuthorID, x.Body, x.CreatedAt})
	}
	for _, x := range c.History {
		out.History = append(out.History, changeHistoryDTO{x.ID, x.ActorID, x.Comment, x.From, x.To, x.CreatedAt})
	}
	for _, x := range c.Revisions {
		out.Revisions = append(out.Revisions, changeRevisionDTO{x.Number, x.ProposedPatch, changeSnapshotDTO{x.Snapshot.Before(), x.Snapshot.After(), x.Snapshot.Digest}, evidenceDTO(x.Evidence), x.ActorID, x.CreatedAt})
	}
	return out
}

func (h *Handler) changeActor(w http.ResponseWriter, r *http.Request, permission account.Permission, mutation bool) (app.Actor, bool) {
	_, a, ok := h.requireSession(w, r)
	if !ok {
		return app.Actor{}, false
	}
	if !account.Can(a.Role, permission) {
		writeErr(w, r, apierr.New(apierr.PermissionDenied, "Your role cannot perform this action.").WithDetail("requiredPermission", string(permission)))
		return app.Actor{}, false
	}
	requestID := middleware.GetReqID(r.Context())
	if mutation {
		key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if key == "" || len(key) > 128 {
			writeErr(w, r, apierr.New(apierr.InvalidArgument, "A valid Idempotency-Key header is required."))
			return app.Actor{}, false
		}
		// Namespace keys by actor so one steward cannot replay another steward's
		// result by guessing a common key. Persist only the digest: caller keys
		// may contain internal job identifiers and never belong in audit output.
		requestID = idempotencyRequestID(a.ID, key)
	}
	return app.Actor{ID: a.ID, Email: a.Email, Role: a.Role, IP: clientIP(r), RequestID: requestID}, true
}

func idempotencyRequestID(actorID, key string) string {
	digest := sha256.Sum256([]byte(actorID + "\x00" + key))
	return "idem_" + hex.EncodeToString(digest[:])
}

func (h *Handler) requireChangeService(w http.ResponseWriter, r *http.Request) bool {
	if h.changeRequests != nil {
		return true
	}
	writeErr(w, r, apierr.New(apierr.Internal, "Change-request moderation is unavailable."))
	return false
}

func (h *Handler) adminCreateChangeRequest(w http.ResponseWriter, r *http.Request) {
	if !h.requireChangeService(w, r) {
		return
	}
	a, ok := h.changeActor(w, r, account.PermProposeChange, true)
	if !ok {
		return
	}
	var body struct {
		Target        domain.Target     `json:"target"`
		ProposedPatch map[string]any    `json:"proposedPatch"`
		Before        map[string]any    `json:"before"`
		After         map[string]any    `json:"after"`
		Evidence      changeEvidenceDTO `json:"evidence"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	e := domain.Evidence(body.Evidence)
	out, err := h.changeRequests.Submit(r.Context(), a, app.SubmitCommand{Target: body.Target, ProposedPatch: body.ProposedPatch, Before: body.Before, After: body.After, Evidence: e})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": changeRequestOut(out)})
}

func (h *Handler) adminListChangeRequests(w http.ResponseWriter, r *http.Request) {
	if !h.requireChangeService(w, r) {
		return
	}
	a, ok := h.changeActor(w, r, account.PermProposeChange, false)
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil {
			writeErr(w, r, apierr.New(apierr.InvalidArgument, "Limit must be an integer."))
			return
		}
	}
	f := app.Filter{Cursor: r.URL.Query().Get("cursor"), Limit: limit, State: domain.State(r.URL.Query().Get("state")), TargetKind: r.URL.Query().Get("targetKind"), TargetID: r.URL.Query().Get("targetId"), SubmitterID: r.URL.Query().Get("submitterId"), ReviewerID: r.URL.Query().Get("reviewerId")}
	page, err := h.changeRequests.List(r.Context(), a, f)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	data := make([]changeRequestDTO, 0, len(page.Data))
	for _, c := range page.Data {
		data = append(data, changeRequestOut(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "nextCursor": page.NextCursor})
}

func (h *Handler) adminGetChangeRequest(w http.ResponseWriter, r *http.Request) {
	if !h.requireChangeService(w, r) {
		return
	}
	a, ok := h.changeActor(w, r, account.PermProposeChange, false)
	if !ok {
		return
	}
	out, err := h.changeRequests.Get(r.Context(), a, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": changeRequestOut(out)})
}

func expectedVersion(body int64) error {
	if body < 1 {
		return apierr.New(apierr.InvalidArgument, "expectedVersion must be at least 1.")
	}
	return nil
}

func (h *Handler) adminTransitionChangeRequest(w http.ResponseWriter, r *http.Request) {
	if !h.requireChangeService(w, r) {
		return
	}
	a, ok := h.changeActor(w, r, account.PermReviewChange, true)
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64        `json:"expectedVersion"`
		State           domain.State `json:"state"`
		Comment         string       `json:"comment"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := expectedVersion(body.ExpectedVersion); err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := h.changeRequests.Transition(r.Context(), a, chi.URLParam(r, "id"), body.ExpectedVersion, body.State, body.Comment)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": changeRequestOut(out)})
}

func (h *Handler) adminReviseChangeRequest(w http.ResponseWriter, r *http.Request) {
	if !h.requireChangeService(w, r) {
		return
	}
	a, ok := h.changeActor(w, r, account.PermProposeChange, true)
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64             `json:"expectedVersion"`
		ProposedPatch   map[string]any    `json:"proposedPatch"`
		After           map[string]any    `json:"after"`
		Evidence        changeEvidenceDTO `json:"evidence"`
		Comment         string            `json:"comment"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := expectedVersion(body.ExpectedVersion); err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := h.changeRequests.Revise(r.Context(), a, chi.URLParam(r, "id"), body.ExpectedVersion, app.ReviseCommand{ProposedPatch: body.ProposedPatch, After: body.After, Evidence: domain.Evidence(body.Evidence), Comment: body.Comment})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": changeRequestOut(out)})
}

func (h *Handler) adminCommentChangeRequest(w http.ResponseWriter, r *http.Request) {
	if !h.requireChangeService(w, r) {
		return
	}
	a, ok := h.changeActor(w, r, account.PermReviewChange, true)
	if !ok {
		return
	}
	var body struct {
		ExpectedVersion int64  `json:"expectedVersion"`
		Body            string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := expectedVersion(body.ExpectedVersion); err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := h.changeRequests.AddReviewerComment(r.Context(), a, chi.URLParam(r, "id"), body.ExpectedVersion, body.Body)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": changeRequestOut(out)})
}
