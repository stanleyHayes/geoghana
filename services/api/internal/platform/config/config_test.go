package config

import "testing"

func TestManagedPlatformPortFollowsServeMode(t *testing.T) {
	t.Setenv("PORT", "10000")
	t.Setenv("GHANAGEO_SERVE_MODE", "grpc")
	t.Setenv("API_HTTP_PORT", "8180")
	t.Setenv("API_GRPC_PORT", "9190")

	cfg := Load()
	if cfg.GRPCPort != "10000" || cfg.HTTPPort != "8180" {
		t.Fatalf("grpc mode ports = http:%s grpc:%s", cfg.HTTPPort, cfg.GRPCPort)
	}

	t.Setenv("GHANAGEO_SERVE_MODE", "http")
	cfg = Load()
	if cfg.HTTPPort != "10000" || cfg.GRPCPort != "9190" {
		t.Fatalf("http mode ports = http:%s grpc:%s", cfg.HTTPPort, cfg.GRPCPort)
	}
}
