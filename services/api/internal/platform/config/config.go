// Package config loads runtime configuration from the environment only.
// No secret is ever read from source (plan rule R12).
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Env          string
	MongoURI     string
	MongoDB      string
	RedisURL     string
	TypesenseURL string
	TypesenseKey string
	// ServeMode is all for local development, http for the public REST/GraphQL
	// process, or grpc for a separately deployed HTTP/2-native process.
	ServeMode string
	HTTPPort  string
	GRPCPort  string
	// DatasetVersion is the canonical geography release exposed by every
	// transport and used when rebuilding the search index.
	DatasetVersion string
	// ExportDir is where dataset download artifacts are written and served
	// from. Relative to the process working directory.
	ExportDir string
	// PasskeyRPID is the domain WebAuthn credentials are bound to. A
	// credential registered for one RPID cannot be used on another, which is
	// what makes passkeys phishing-resistant, so it must be the real
	// registrable domain and never a wildcard.
	PasskeyRPID                string
	PasskeyOrigins             []string
	LogLevel                   string
	SecurityAlertWebhookURL    string
	SecurityAlertWebhookSecret string
	// AllowedOrigins is a CORS allow-list, never a wildcard (Spec 12.4).
	AllowedOrigins          []string
	TrustProxyHeaders       bool
	TrustedProxyCIDRs       []string
	RequireHTTPS            bool
	ResendAPIKey            string
	ResendFromEmail         string
	ResendAPIURL            string
	AllowTestResendEndpoint bool
	PortalURL               string
}

func Load() Config {
	serveMode := env("GHANAGEO_SERVE_MODE", "all")
	httpPort := env("API_HTTP_PORT", "8180")
	grpcPort := env("API_GRPC_PORT", "9190")
	// Managed hosts expose one assigned public port. Split deployments bind
	// the selected transport to it; local all-in-one mode keeps distinct ports.
	if platformPort := os.Getenv("PORT"); platformPort != "" {
		switch strings.ToLower(serveMode) {
		case "http":
			httpPort = platformPort
		case "grpc":
			grpcPort = platformPort
		}
	}
	return Config{
		Env:            env("GHANAGEO_ENV", "local"),
		MongoURI:       env("MONGO_URI", "mongodb://localhost:27117/ghanageo?replicaSet=rs0&directConnection=true"),
		MongoDB:        env("MONGO_DB", "ghanageo"),
		RedisURL:       env("REDIS_URL", "redis://localhost:6679"),
		TypesenseURL:   env("TYPESENSE_URL", "http://localhost:8108"),
		TypesenseKey:   env("TYPESENSE_API_KEY", "ghanageo_local_dev_only"),
		ServeMode:      serveMode,
		HTTPPort:       httpPort,
		GRPCPort:       grpcPort,
		DatasetVersion: env("DATASET_VERSION", "2026.08.3-ulid"),
		ExportDir:      env("API_EXPORT_DIR", "../../data/exports"),
		PasskeyRPID:    env("API_PASSKEY_RPID", "localhost"),
		PasskeyOrigins: strings.Split(env("API_PASSKEY_ORIGINS",
			"http://localhost:3100,http://localhost:3102,http://localhost:3103,http://localhost:8180"), ","),
		LogLevel:                   env("API_LOG_LEVEL", "info"),
		SecurityAlertWebhookURL:    os.Getenv("SECURITY_ALERT_WEBHOOK_URL"),
		SecurityAlertWebhookSecret: os.Getenv("SECURITY_ALERT_WEBHOOK_SECRET"),
		AllowedOrigins: strings.Split(env("API_ALLOWED_ORIGINS",
			"http://localhost:3103,http://localhost:3102,http://localhost:3101,http://localhost:3100"), ","),
		TrustProxyHeaders:       envBool("API_TRUST_PROXY_HEADERS", false),
		TrustedProxyCIDRs:       splitNonEmpty(os.Getenv("API_TRUSTED_PROXY_CIDRS")),
		RequireHTTPS:            envBool("API_REQUIRE_HTTPS", false),
		ResendAPIKey:            os.Getenv("RESEND_API_KEY"),
		ResendFromEmail:         os.Getenv("RESEND_FROM_EMAIL"),
		ResendAPIURL:            env("RESEND_API_URL", "https://api.resend.com/emails"),
		AllowTestResendEndpoint: envBool("GHANAGEO_ALLOW_TEST_RESEND_ENDPOINT", false),
		PortalURL:               env("GHANAGEO_PORTAL_URL", "http://localhost:3102"),
	}
}

// IsProduction gates behaviour that must never run outside production paths.
func (c Config) IsProduction() bool { return strings.EqualFold(c.Env, "production") }

func (c Config) String() string {
	return fmt.Sprintf("env=%s mongo=%s db=%s http=:%s", c.Env, redact(c.MongoURI), c.MongoDB, c.HTTPPort)
}

// redact strips credentials from a URI before it reaches a log line.
func redact(uri string) string {
	at := strings.LastIndex(uri, "@")
	scheme := strings.Index(uri, "://")
	if at > 0 && scheme > 0 {
		return uri[:scheme+3] + "***@" + uri[at+1:]
	}
	return uri
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func splitNonEmpty(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
