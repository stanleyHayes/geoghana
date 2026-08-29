package rest

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	usageDomain "github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
)

func (h *Handler) captureUsage(protocol string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			writer := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(writer, r)

			caller := auth.FromContext(r.Context())
			if caller.Key == nil {
				return
			}
			id, err := identity.NewID("use")
			if err != nil {
				h.log.Warn("usage event id failed", "request_id", middleware.GetReqID(r.Context()), "err", err)
				return
			}
			decision := auth.DecisionFromContext(r.Context())
			operation := r.Method + " " + r.URL.Path
			if route := chi.RouteContext(r.Context()).RoutePattern(); route != "" {
				operation = r.Method + " " + route
			}
			event := usageDomain.Event{
				ID: id, RequestID: middleware.GetReqID(r.Context()), OrganizationID: caller.Key.OrganizationID,
				ApplicationID: caller.Key.ApplicationID, KeyID: caller.Key.ID, Protocol: protocol,
				Operation: operation, Status: strconv.Itoa(writer.Status()), Success: writer.Status() < 400,
				LatencyMS: time.Since(started).Milliseconds(), QuotaCost: costOf(r).Units(), QuotaLimit: decision.Limit,
				QuotaRemaining: decision.Remaining, Geography: requestGeography(r), At: time.Now().UTC(),
			}
			ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), time.Second)
			defer cancel()
			if err := h.usage.Record(ctx, event); err != nil {
				h.log.Warn("usage event persistence failed", "request_id", event.RequestID, "err", err)
			}
		})
	}
}

func requestGeography(r *http.Request) string {
	if value := strings.TrimSpace(r.URL.Query().Get("regionId")); value != "" {
		return "region:" + value
	}
	if value := strings.TrimSpace(r.URL.Query().Get("districtId")); value != "" {
		return "district:" + value
	}
	id := chi.URLParam(r, "id")
	switch {
	case id != "" && strings.Contains(r.URL.Path, "/regions/"):
		return "region:" + id
	case id != "" && strings.Contains(r.URL.Path, "/districts/"):
		return "district:" + id
	default:
		return ""
	}
}

func (h *Handler) developerUsageSummary(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	orgID, appID := chi.URLParam(r, "orgId"), chi.URLParam(r, "appId")
	if err := h.developer.ApplicationAccess(r.Context(), account.ID, orgID, appID); err != nil {
		writeErr(w, r, err)
		return
	}
	days := intParam(r, "days", 30)
	if days < 1 || days > 30 {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "days must be between 1 and 30."))
		return
	}
	summary, err := h.usage.Summary(r.Context(), orgID, appID, time.Now().UTC().Add(-time.Duration(days)*24*time.Hour))
	if err != nil {
		writeErr(w, r, apierr.Wrap(apierr.Internal, "Could not load usage analytics.", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": summaryResponse(summary)})
}

func (h *Handler) developerRequestLogs(w http.ResponseWriter, r *http.Request) {
	_, account, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	orgID, appID := chi.URLParam(r, "orgId"), chi.URLParam(r, "appId")
	if err := h.developer.ApplicationAccess(r.Context(), account.ID, orgID, appID); err != nil {
		writeErr(w, r, err)
		return
	}
	var before *time.Time
	if raw := r.URL.Query().Get("before"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeErr(w, r, apierr.New(apierr.InvalidArgument, "before must be an RFC3339 timestamp."))
			return
		}
		before = &parsed
	}
	limit := intParam(r, "limit", 25)
	page, err := h.usage.List(r.Context(), orgID, appID, before, limit)
	if err != nil {
		writeErr(w, r, apierr.Wrap(apierr.Internal, "Could not load request logs.", err))
		return
	}
	data := make([]map[string]any, 0, len(page.Data))
	for _, event := range page.Data {
		data = append(data, eventResponse(event))
	}
	var nextBefore any
	if page.NextBefore != nil {
		nextBefore = page.NextBefore.Format(time.RFC3339Nano)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "pagination": map[string]any{"nextBefore": nextBefore, "limit": min(100, max(1, limit))}})
}

func breakdownResponse(items []usageDomain.Breakdown) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"label": item.Label, "requests": item.Requests, "errors": item.Errors, "quotaCost": item.QuotaCost, "avgLatencyMs": item.AvgLatencyMS})
	}
	return out
}

func summaryResponse(summary usageDomain.Summary) map[string]any {
	return map[string]any{"since": summary.Since, "requests": summary.Requests, "errors": summary.Errors, "quotaCost": summary.QuotaCost, "avgLatencyMs": summary.AvgLatencyMS, "byProtocol": breakdownResponse(summary.ByProtocol), "byEndpoint": breakdownResponse(summary.ByEndpoint), "byGeography": breakdownResponse(summary.ByGeography)}
}

func eventResponse(event usageDomain.Event) map[string]any {
	return map[string]any{"requestId": event.RequestID, "protocol": event.Protocol, "operation": event.Operation, "status": event.Status, "success": event.Success, "latencyMs": event.LatencyMS, "quotaCost": event.QuotaCost, "quotaLimit": event.QuotaLimit, "quotaRemaining": event.QuotaRemaining, "geography": event.Geography, "at": event.At}
}
