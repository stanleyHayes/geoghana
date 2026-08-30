package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	appaccount "github.com/ghanageo/ghanageo/services/api/internal/app/account"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/config"
)

type resendMailer struct {
	apiKey    string
	from      string
	portalURL string
	endpoint  string
	client    *http.Client
}

const productionResendEndpoint = "https://api.resend.com/emails"

func newAccountMailer(cfg config.Config) (appaccount.Mailer, error) {
	if !cfg.IsProduction() && cfg.ResendAPIKey == "" {
		return nil, nil
	}
	missing := make([]string, 0, 3)
	if cfg.ResendAPIKey == "" {
		missing = append(missing, "RESEND_API_KEY")
	}
	if cfg.ResendFromEmail == "" {
		missing = append(missing, "RESEND_FROM_EMAIL")
	}
	if cfg.PortalURL == "" {
		missing = append(missing, "GHANAGEO_PORTAL_URL")
	}
	if len(missing) != 0 {
		return nil, fmt.Errorf("transactional mail configuration missing: %s", strings.Join(missing, ", "))
	}
	portal, err := absoluteHTTPSURL(cfg.PortalURL)
	if err != nil {
		return nil, errors.New("GHANAGEO_PORTAL_URL must be an absolute https URL with a host")
	}
	endpoint := cfg.ResendAPIURL
	if endpoint == "" {
		endpoint = productionResendEndpoint
	}
	parsedEndpoint, endpointErr := absoluteHTTPSURL(endpoint)
	testOverride := !cfg.IsProduction() && cfg.AllowTestResendEndpoint
	if testOverride {
		parsedEndpoint, endpointErr = url.Parse(endpoint)
		if endpointErr == nil && (parsedEndpoint.Scheme == "" || parsedEndpoint.Host == "") {
			endpointErr = errors.New("test endpoint must be absolute")
		}
	}
	if endpointErr != nil || !testOverride && parsedEndpoint.String() != productionResendEndpoint {
		return nil, errors.New("RESEND_API_URL must be the canonical Resend HTTPS endpoint")
	}
	return &resendMailer{
		apiKey: cfg.ResendAPIKey, from: cfg.ResendFromEmail,
		portalURL: strings.TrimRight(portal.String(), "/"), endpoint: parsedEndpoint.String(),
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func absoluteHTTPSURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("URL must be absolute HTTPS without user information")
	}
	return parsed, nil
}

func validateProductionIdentityConfig(cfg config.Config) error {
	if !cfg.IsProduction() {
		return nil
	}
	rpID := strings.ToLower(strings.TrimSpace(cfg.PasskeyRPID))
	if rpID == "" || rpID == "localhost" || net.ParseIP(rpID) != nil || strings.ContainsAny(rpID, "/:") {
		return errors.New("API_PASSKEY_RPID must be a production registrable domain")
	}
	if len(cfg.PasskeyOrigins) == 0 {
		return errors.New("API_PASSKEY_ORIGINS must contain at least one production HTTPS origin")
	}
	for _, value := range cfg.PasskeyOrigins {
		origin, err := absoluteHTTPSURL(strings.TrimSpace(value))
		if err != nil || origin.Path != "" && origin.Path != "/" || origin.RawQuery != "" || origin.Fragment != "" {
			return errors.New("API_PASSKEY_ORIGINS must contain only absolute HTTPS origins")
		}
		host := strings.ToLower(origin.Hostname())
		if host != rpID && !strings.HasSuffix(host, "."+rpID) {
			return errors.New("API_PASSKEY_ORIGINS host must equal or be a subdomain of API_PASSKEY_RPID")
		}
	}
	return nil
}

func (m *resendMailer) SendVerification(ctx context.Context, email, token string) error {
	return m.send(ctx, email, "Verify your GhanaGeo email", "verify-email", token,
		"Verify email address")
}

func (m *resendMailer) SendPasswordReset(ctx context.Context, email, token string) error {
	return m.send(ctx, email, "Reset your GhanaGeo password", "reset-password", token,
		"Reset password")
}

func (m *resendMailer) send(ctx context.Context, email, subject, path, token, action string) error {
	link := m.portalURL + "/" + path + "?token=" + url.QueryEscape(token)
	payload, err := json.Marshal(map[string]any{
		"from": m.from, "to": []string{email}, "subject": subject,
		"html": fmt.Sprintf(`<p>%s</p><p><a href="%s">%s</a></p><p>If you did not request this, you can ignore this email.</p>`, subject, link, action),
	})
	if err != nil {
		return fmt.Errorf("encode transactional email: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create transactional email request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("send transactional email: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 32<<10))
		return fmt.Errorf("transactional email provider returned status %d", resp.StatusCode)
	}
	return nil
}

func proxySecurity(next http.Handler, cfg config.Config) (http.Handler, error) {
	trustedNetworks, err := parseTrustedProxyCIDRs(cfg.TrustedProxyCIDRs)
	if err != nil {
		return nil, err
	}
	if cfg.TrustProxyHeaders && len(trustedNetworks) == 0 {
		return nil, errors.New("API_TRUSTED_PROXY_CIDRS is required when API_TRUST_PROXY_HEADERS is enabled")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trustedPeer := cfg.TrustProxyHeaders && peerInNetworks(r.RemoteAddr, trustedNetworks)
		trustedHTTPS := trustedPeer && forwardedProto(r) == "https"
		secure := r.TLS != nil || trustedHTTPS
		if trustedPeer {
			// Render's public edge overwrites CF-Connecting-IP, whereas a caller
			// can control the left side of X-Forwarded-For. Normalize the socket
			// address from the single edge-owned value, then discard every proxy
			// header before chi's RealIP middleware runs. Missing or malformed
			// edge data fails safe by retaining the proxy socket address.
			if ip := trustedEdgeClientIP(r.Header.Get("CF-Connecting-IP")); ip != "" {
				r.RemoteAddr = net.JoinHostPort(ip, "0")
			}
		}
		stripForwardingHeaders(r.Header)
		if trustedHTTPS {
			r.TLS = &tls.ConnectionState{}
		}
		if cfg.RequireHTTPS && !secure && r.URL.Path != "/health" {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Connection", "close")
			w.WriteHeader(http.StatusUpgradeRequired)
			_, _ = io.WriteString(w, `{"error":{"code":"HTTPS_REQUIRED","message":"HTTPS is required."}}`)
			return
		}
		next.ServeHTTP(w, r)
	}), nil
}

func parseTrustedProxyCIDRs(values []string) ([]*net.IPNet, error) {
	result := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return nil, fmt.Errorf("invalid API_TRUSTED_PROXY_CIDRS entry %q: %w", value, err)
		}
		result = append(result, network)
	}
	return result, nil
}

func peerInNetworks(remoteAddr string, networks []*net.IPNet) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func trustedEdgeClientIP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, ",") {
		return ""
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func stripForwardingHeaders(header http.Header) {
	for _, name := range []string{
		"CF-Connecting-IP", "Forwarded", "X-Forwarded-For", "X-Forwarded-Host",
		"X-Forwarded-Proto", "X-Real-IP",
	} {
		header.Del(name)
	}
}

func forwardedProto(r *http.Request) string {
	value, _, _ := strings.Cut(r.Header.Get("X-Forwarded-Proto"), ",")
	return strings.ToLower(strings.TrimSpace(value))
}
