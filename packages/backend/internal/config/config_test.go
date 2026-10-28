package config

import (
	"testing"
	"time"
)

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
	if cfg.CorsOrigins[0] != "http://vibebiz-dev.com:5173" || cfg.CorsOrigins[1] != "http://vibebiz-dev-ops.com:5174" {
		t.Fatalf("cors origins = %v", cfg.CorsOrigins)
	}
}

func TestLoadAuthSessionConfig(t *testing.T) {
	t.Setenv("CORS_ORIGINS", "http://vibebiz-dev.com:5173,http://vibebiz-dev-ops.com:5174")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("SESSION_USER_TTL", "168h")
	t.Setenv("SESSION_OPS_TTL", "12h")

	cfg := Load("cp")
	if cfg.RedisDB != 2 {
		t.Fatalf("redis db = %d, want 2", cfg.RedisDB)
	}
	if !cfg.CookieSecure {
		t.Fatal("cookie secure = false, want true")
	}
	if cfg.SessionUserTTL != 168*time.Hour {
		t.Fatalf("session user ttl = %v, want 168h", cfg.SessionUserTTL)
	}
	if cfg.SessionOpsTTL != 12*time.Hour {
		t.Fatalf("session ops ttl = %v, want 12h", cfg.SessionOpsTTL)
	}
	if len(cfg.CorsOrigins) != 2 || cfg.CorsOrigins[0] != "http://vibebiz-dev.com:5173" || cfg.CorsOrigins[1] != "http://vibebiz-dev-ops.com:5174" {
		t.Fatalf("cors origins = %v", cfg.CorsOrigins)
	}
}

func TestLoadAuthSessionDefaults(t *testing.T) {
	t.Setenv("REDIS_DB", "")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("SESSION_USER_TTL", "")
	t.Setenv("SESSION_OPS_TTL", "")

	cfg := Load("cp")
	if cfg.RedisDB != 0 {
		t.Fatalf("redis db = %d, want 0", cfg.RedisDB)
	}
	if cfg.CookieSecure {
		t.Fatal("cookie secure = true, want false")
	}
	if cfg.SessionUserTTL != 168*time.Hour || cfg.SessionOpsTTL != 12*time.Hour {
		t.Fatalf("session ttls = %v/%v", cfg.SessionUserTTL, cfg.SessionOpsTTL)
	}
}

func TestLoadHostOverride(t *testing.T) {
	t.Setenv("HOST", "0.0.0.0")

	if cfg := Load("dp"); cfg.Host != "0.0.0.0" {
		t.Fatalf("host = %q, want 0.0.0.0", cfg.Host)
	}
}

func TestLoadBackendResourceConfig(t *testing.T) {
	t.Setenv("MYSQL_HOST", "db:3306")
	t.Setenv("MYSQL_USERNAME", "u")
	t.Setenv("MYSQL_PASSWORD", "p")
	t.Setenv("MYSQL_DATABASE", "vibebiz")
	t.Setenv("MYSQL_MAX_OPEN_CONNS", "30")
	t.Setenv("MYSQL_MAX_IDLE_CONNS", "15")
	t.Setenv("REDIS_HOST", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "r")
	t.Setenv("S3_BUCKET", "bucket")
	t.Setenv("S3_REGION", "default")
	t.Setenv("S3_ENDPOINT", "http://s3:9000")
	t.Setenv("S3_ACCESS_KEY", "ak")
	t.Setenv("S3_SECRET_KEY", "sk")

	cfg := Load("cp")
	if cfg.MySQLHost != "db:3306" || cfg.MySQLUsername != "u" || cfg.MySQLPassword != "p" || cfg.MySQLDatabase != "vibebiz" {
		t.Fatalf("mysql config = %+v", cfg)
	}
	if cfg.MySQLMaxOpenConns != 30 || cfg.MySQLMaxIdleConns != 15 {
		t.Fatalf("mysql pool = %d/%d", cfg.MySQLMaxOpenConns, cfg.MySQLMaxIdleConns)
	}
	if cfg.RedisHost != "redis:6379" || cfg.RedisPassword != "r" {
		t.Fatalf("redis config = %+v", cfg)
	}
	if cfg.S3Bucket != "bucket" || cfg.S3Region != "default" || cfg.S3Endpoint != "http://s3:9000" || cfg.S3AccessKey != "ak" || cfg.S3SecretKey != "sk" {
		t.Fatalf("s3 config = %+v", cfg)
	}
}
