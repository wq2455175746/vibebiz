package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Host           string
	Port           int
	Debug          bool
	SandboxBaseURL string
	CorsOrigins    []string
	Plane          string
	ServiceName    string
}

func Load(plane string) Config {
	plane = normalizePlane(plane)
	return Config{
		Host:           getString("HOST", "127.0.0.1"),
		Port:           getInt("PORT", defaultPort(plane)),
		Debug:          getBool("DEBUG", true),
		SandboxBaseURL: getString("SANDBOX_BASE_URL", "http://localhost:8000"),
		CorsOrigins:    splitCSV(getString("CORS_ORIGINS", "http://localhost:5173,http://localhost:5174")),
		Plane:          plane,
		ServiceName:    "vibebiz-" + plane,
	}
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
