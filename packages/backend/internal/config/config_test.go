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
	t.Setenv("S3_REMOTE_DIR_PREFIX", "tmp/vibebiz/")
	t.Setenv("S3_FORCE_PATH_STYLE", "")
	t.Setenv("S3_PRESIGN_EXPIRY_MINUTES", "20")

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
	if cfg.S3RemoteDirPrefix != "tmp/vibebiz/" {
		t.Fatalf("s3 prefix = %q", cfg.S3RemoteDirPrefix)
	}
	if !cfg.S3ForcePathStyle {
		t.Fatal("force path style = false, want true for custom endpoint")
	}
	if cfg.S3PresignExpiryMinutes != 20 || cfg.S3PresignExpiry() != 20*time.Minute {
		t.Fatalf("presign expiry = %d/%v", cfg.S3PresignExpiryMinutes, cfg.S3PresignExpiry())
	}
}

func TestLoadS3Defaults(t *testing.T) {
	t.Setenv("S3_ENDPOINT", "")
	t.Setenv("S3_FORCE_PATH_STYLE", "")
	t.Setenv("S3_PRESIGN_EXPIRY_MINUTES", "")

	cfg := Load("cp")
	if cfg.S3ForcePathStyle {
		t.Fatal("force path style = true, want false without endpoint")
	}
	if cfg.S3PresignExpiryMinutes != 15 || cfg.S3PresignExpiry() != 15*time.Minute {
		t.Fatalf("presign expiry = %d/%v", cfg.S3PresignExpiryMinutes, cfg.S3PresignExpiry())
	}
}

func TestLoadSandboxRegistryConfig(t *testing.T) {
	t.Setenv("SANDBOX_REGISTRY_TOKEN", "secret")
	t.Setenv("SANDBOX_HEARTBEAT_TTL", "20s")
	t.Setenv("SANDBOX_HEALTH_INTERVAL", "12s")
	t.Setenv("SANDBOX_HEALTH_TIMEOUT", "3s")
	t.Setenv("ASYNQ_REDIS_DB", "2")
	t.Setenv("ASYNQ_CONCURRENCY", "6")

	cfg := Load("cp")
	if cfg.SandboxRegistryToken != "secret" {
		t.Fatalf("token = %q", cfg.SandboxRegistryToken)
	}
	if cfg.SandboxHeartbeatTTL != 20*time.Second || cfg.SandboxHealthInterval != 12*time.Second || cfg.SandboxHealthTimeout != 3*time.Second {
		t.Fatalf("durations = %v/%v/%v", cfg.SandboxHeartbeatTTL, cfg.SandboxHealthInterval, cfg.SandboxHealthTimeout)
	}
	if cfg.AsynqRedisDB != 2 || cfg.AsynqConcurrency != 6 {
		t.Fatalf("asynq = %d/%d", cfg.AsynqRedisDB, cfg.AsynqConcurrency)
	}
}

func TestLoadSandboxRegistryDefaults(t *testing.T) {
	t.Setenv("SANDBOX_REGISTRY_TOKEN", "")
	t.Setenv("SANDBOX_HEARTBEAT_TTL", "")
	t.Setenv("SANDBOX_HEALTH_INTERVAL", "")
	t.Setenv("SANDBOX_HEALTH_TIMEOUT", "")
	t.Setenv("ASYNQ_REDIS_DB", "")
	t.Setenv("ASYNQ_CONCURRENCY", "")

	cfg := Load("cp")
	if cfg.SandboxHeartbeatTTL != 15*time.Second || cfg.SandboxHealthInterval != 10*time.Second || cfg.SandboxHealthTimeout != 2*time.Second {
		t.Fatalf("durations = %v/%v/%v", cfg.SandboxHeartbeatTTL, cfg.SandboxHealthInterval, cfg.SandboxHealthTimeout)
	}
	if cfg.AsynqRedisDB != 1 || cfg.AsynqConcurrency != 4 {
		t.Fatalf("asynq = %d/%d", cfg.AsynqRedisDB, cfg.AsynqConcurrency)
	}
}

func TestLoadModelSecretKey(t *testing.T) {
	t.Setenv("MODEL_SECRET_KEY", "abc")
	if cfg := Load("cp"); cfg.ModelSecretKey != "abc" {
		t.Fatalf("model secret key = %q", cfg.ModelSecretKey)
	}
}

func TestLoadEnterpriseHarnessConfig(t *testing.T) {
	t.Setenv("ENTERPRISE_HARNESS_S3_KEY", "")
	if cfg := Load("cp"); cfg.EnterpriseHarnessS3Key != "tmp/vibebiz_prod/vibebiz-harness.zip" {
		t.Fatalf("harness key = %q", cfg.EnterpriseHarnessS3Key)
	}

	t.Setenv("ENTERPRISE_HARNESS_S3_KEY", "enterprise/harness.zip")
	if cfg := Load("cp"); cfg.EnterpriseHarnessS3Key != "enterprise/harness.zip" {
		t.Fatalf("harness key = %q", cfg.EnterpriseHarnessS3Key)
	}
}

func TestLoadNotificationChannelConfig(t *testing.T) {
	t.Setenv("MODEL_SECRET_KEY", "model-key")
	t.Setenv("NOTIFICATION_SECRET_KEY", "")
	if cfg := Load("cp"); cfg.NotificationSecretKey != "model-key" {
		t.Fatalf("notification secret fallback = %q", cfg.NotificationSecretKey)
	}
	t.Setenv("NOTIFICATION_SECRET_KEY", "channel-key")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "465")
	t.Setenv("SMTP_USERNAME", "u")
	t.Setenv("SMTP_PASSWORD", "p")
	t.Setenv("SMTP_FROM", "noreply@example.com")
	t.Setenv("SMTP_SECURITY", "tls")
	cfg := Load("cp")
	if cfg.NotificationSecretKey != "channel-key" || cfg.SMTPHost != "smtp.example.com" || cfg.SMTPPort != 465 || cfg.SMTPSecurity != "tls" {
		t.Fatalf("notification config = %+v", cfg)
	}
	if cfg.SMTPUsername != "u" || cfg.SMTPPassword != "p" || cfg.SMTPFrom != "noreply@example.com" {
		t.Fatalf("smtp config = %+v", cfg)
	}
}
