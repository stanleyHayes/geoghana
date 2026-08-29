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
	HTTPPort     string
	GRPCPort     string
	LogLevel     string
}

func Load() Config {
	return Config{
		Env:          env("GHANAGEO_ENV", "local"),
		MongoURI:     env("MONGO_URI", "mongodb://localhost:27117/ghanageo?replicaSet=rs0&directConnection=true"),
		MongoDB:      env("MONGO_DB", "ghanageo"),
		RedisURL:     env("REDIS_URL", "redis://localhost:6679"),
		TypesenseURL: env("TYPESENSE_URL", "http://localhost:8108"),
		TypesenseKey: env("TYPESENSE_API_KEY", "ghanageo_local_dev_only"),
		HTTPPort:     env("API_HTTP_PORT", "8080"),
		GRPCPort:     env("API_GRPC_PORT", "9090"),
		LogLevel:     env("API_LOG_LEVEL", "info"),
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
