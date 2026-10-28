package dao

import (
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestMySQLConfigDSN(t *testing.T) {
	cfg := MySQLConfig{
		Host:     "db:3306",
		Username: "u",
		Password: "p#1",
		Database: "vibebiz",
	}
	parsed, err := mysqldriver.ParseDSN(cfg.DSN())
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if parsed.User != "u" || parsed.Passwd != "p#1" || parsed.Addr != "db:3306" || parsed.DBName != "vibebiz" {
		t.Fatalf("parsed = %+v", parsed)
	}
	if !parsed.ParseTime {
		t.Fatal("ParseTime = false, want true")
	}
}
