package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Host           string
	Port           int
	Debug          bool
	SandboxBaseURL string
	CorsOrigins    []string
	Plane          string
	ServiceName    string

	MySQLHost         string
	MySQLUsername     string
	MySQLPassword     string
	MySQLDatabase     string
	MySQLMaxOpenConns int
	MySQLMaxIdleConns int

	RedisHost     string
	RedisPassword string
	RedisDB       int

	CookieSecure   bool
	SessionUserTTL time.Duration
	SessionOpsTTL  time.Duration

	S3Bucket               string
	S3Region               string
	S3Endpoint             string
	S3AccessKey            string
	S3SecretKey            string
	S3ForcePathStyle       bool
	S3PresignExpiryMinutes int
}

func Load(plane string) Config {
	plane = normalizePlane(plane)
	return Config{
		Host:           getString("HOST", "127.0.0.1"),
		Port:           getInt("PORT", defaultPort(plane)),
		Debug:          getBool("DEBUG", true),
		SandboxBaseURL: getString("SANDBOX_BASE_URL", "http://localhost:8000"),
		CorsOrigins:    splitCSV(getString("CORS_ORIGINS", "http://vibebiz-dev.com:5173,http://vibebiz-dev-ops.com:5174")),
		Plane:          plane,
		ServiceName:    "vibebiz-" + plane,

		MySQLHost:         getString("MYSQL_HOST", "127.0.0.1:3306"),
		MySQLUsername:     getString("MYSQL_USERNAME", ""),
		MySQLPassword:     getString("MYSQL_PASSWORD", ""),
		MySQLDatabase:     getString("MYSQL_DATABASE", "vibebiz"),
		MySQLMaxOpenConns: getInt("MYSQL_MAX_OPEN_CONNS", 20),
		MySQLMaxIdleConns: getInt("MYSQL_MAX_IDLE_CONNS", 10),

		RedisHost:     getString("REDIS_HOST", "127.0.0.1:6379"),
		RedisPassword: getString("REDIS_PASSWORD", ""),
		RedisDB:       getInt("REDIS_DB", 0),

		CookieSecure:   getBool("COOKIE_SECURE", false),
		SessionUserTTL: getDuration("SESSION_USER_TTL", 168*time.Hour),
		SessionOpsTTL:  getDuration("SESSION_OPS_TTL", 12*time.Hour),

		S3Bucket:               getString("S3_BUCKET", "vibebiz"),
		S3Region:               getString("S3_REGION", "default"),
		S3Endpoint:             getString("S3_ENDPOINT", ""),
		S3AccessKey:            getString("S3_ACCESS_KEY", ""),
		S3SecretKey:            getString("S3_SECRET_KEY", ""),
		S3ForcePathStyle:       getBool("S3_FORCE_PATH_STYLE", getString("S3_ENDPOINT", "") != ""),
		S3PresignExpiryMinutes: getInt("S3_PRESIGN_EXPIRY_MINUTES", 15),
	}
}

func (c Config) S3PresignExpiry() time.Duration {
	minutes := c.S3PresignExpiryMinutes
	if minutes <= 0 {
		minutes = 15
	}
	return time.Duration(minutes) * time.Minute
}

func normalizePlane(plane string) string {
	if strings.EqualFold(strings.TrimSpace(plane), "dp") {
		return "dp"
	}
	return "cp"
}

func defaultPort(plane string) int {
	if plane == "dp" {
		return 8081
	}
	return 8080
}

func getString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v, err := strconv.Atoi(getString(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func getBool(key string, fallback bool) bool {
	v, err := strconv.ParseBool(getString(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(getString(key, ""))
	if err != nil {
		return fallback
	}
	return v
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
