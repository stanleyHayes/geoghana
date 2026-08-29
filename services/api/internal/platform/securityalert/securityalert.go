// Package securityalert delivers security signals to an operator-controlled
// webhook and always mirrors them to structured logs.
package securityalert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Kind string

const (
	UnusualKeyUsage Kind = "security.key.unusual_usage"
	QuotaSpike      Kind = "security.quota.spike"
	AdminAuthFailed Kind = "security.admin.authentication_failed"
)

type Event struct {
	Kind       Kind           `json:"kind"`
	Severity   string         `json:"severity"`
	ActorID    string         `json:"actorId,omitempty"`
	SourceIP   string         `json:"sourceIp,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
	Details    map[string]any `json:"details,omitempty"`
}

type Reporter interface {
	Report(context.Context, Event) error
}

// WebhookReporter posts signed JSON when a webhook is configured. With no
// URL it remains useful as a structured-log reporter for local development.
type WebhookReporter struct {
	url    string
	secret string
	client *http.Client
	log    *slog.Logger
	now    func() time.Time
}

func New(url, secret string, log *slog.Logger) *WebhookReporter {
	if log == nil {
		log = slog.Default()
	}
	return &WebhookReporter{
		url: strings.TrimSpace(url), secret: secret, log: log,
		client: &http.Client{Timeout: 3 * time.Second}, now: time.Now,
	}
}

func (r *WebhookReporter) Report(ctx context.Context, event Event) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = r.now().UTC()
	}
	if event.Severity == "" {
		event.Severity = "warning"
	}
	r.log.WarnContext(ctx, "security alert",
		"kind", event.Kind, "severity", event.Severity,
		"actor_id", event.ActorID, "source_ip", event.SourceIP,
		"details", event.Details)

	if r.url == "" {
		return nil
	}
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode security alert: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create security alert request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GhanaGeo-Security-Alerts/1.0")
	if r.secret != "" {
		mac := hmac.New(sha256.New, []byte(r.secret))
		_, _ = mac.Write(body)
		req.Header.Set("X-GhanaGeo-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}
	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("deliver security alert: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("deliver security alert: webhook returned %s", res.Status)
	}
	return nil
}

// ReportBestEffort keeps alert-provider failures out of authentication and
// request paths while ensuring the failure itself remains observable.
func ReportBestEffort(ctx context.Context, reporter Reporter, log *slog.Logger, event Event) {
	if reporter == nil {
		return
	}
	if err := reporter.Report(ctx, event); err != nil && !errors.Is(err, context.Canceled) {
		if log == nil {
			log = slog.Default()
		}
		log.ErrorContext(ctx, "security alert delivery failed", "kind", event.Kind, "err", err)
	}
}
