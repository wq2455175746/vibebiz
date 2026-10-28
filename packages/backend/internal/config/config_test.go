package config

import "testing"

func TestLoadDefaultsBindLoopbackAndAllowBothFrontends(t *testing.T) {
	t.Setenv("HOST", "")
	t.Setenv("CORS_ORIGINS", "")

	cfg := Load("cp")
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", cfg.Host)
	}
	if len(cfg.CorsOrigins) != 2 {
		t.Fatalf("cors origins = %v, want two defaults", cfg.CorsOrigins)
	}
	if cfg.CorsOrigins[0] != "http://localhost:5173" || cfg.CorsOrigins[1] != "http://localhost:5174" {
		t.Fatalf("cors origins = %v", cfg.CorsOrigins)
	}
}

func TestLoadHostOverride(t *testing.T) {
	t.Setenv("HOST", "0.0.0.0")

	if cfg := Load("dp"); cfg.Host != "0.0.0.0" {
		t.Fatalf("host = %q, want 0.0.0.0", cfg.Host)
	}
}
