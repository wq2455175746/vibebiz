package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"

	"vibebiz/internal/config"
	"vibebiz/internal/dao"
	"vibebiz/migrations"
)

const usage = `usage: migrate <create-db|up|down|status|version|redo>

  create-db  create the configured database if it does not exist
  up         apply all pending migrations
  down       roll back one migration
  status     print migration status
  version    print current database version
  redo       roll back one migration and re-apply it`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf(usage)
	}

	cfg := config.Load("cp")
	mysqlCfg := dao.MySQLConfig{
		Host:         cfg.MySQLHost,
		Username:     cfg.MySQLUsername,
		Password:     cfg.MySQLPassword,
		Database:     cfg.MySQLDatabase,
		MaxOpenConns: cfg.MySQLMaxOpenConns,
		MaxIdleConns: cfg.MySQLMaxIdleConns,
	}

	command := strings.TrimSpace(args[0])
	if command == "create-db" {
		return createDatabase(mysqlCfg)
	}
	if !validCommand(command) {
		return fmt.Errorf(usage)
	}

	db, err := sql.Open("mysql", mysqlCfg.DSN())
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("mysql"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	switch command {
	case "up":
		return goose.Up(db, ".")
	case "down":
		return goose.Down(db, ".")
	case "status":
		return goose.Status(db, ".")
	case "version":
		version, err := goose.GetDBVersion(db)
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, version)
		return nil
	case "redo":
		if err := goose.Down(db, "."); err != nil {
			return err
		}
		return goose.Up(db, ".")
	default:
		return fmt.Errorf(usage)
	}
}

func validCommand(command string) bool {
	switch command {
	case "up", "down", "status", "version", "redo":
		return true
	default:
		return false
	}
}

func createDatabase(cfg dao.MySQLConfig) error {
	dsnCfg, err := mysqldriver.ParseDSN(cfg.DSN())
	if err != nil {
		return fmt.Errorf("parse dsn: %w", err)
	}
	database := dsnCfg.DBName
	if database == "" {
		return fmt.Errorf("MYSQL_DATABASE is empty")
	}

	dsnCfg.DBName = ""
	admin, err := sql.Open("mysql", dsnCfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("open mysql admin: %w", err)
	}
	defer admin.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql admin: %w", err)
	}

	name := strings.ReplaceAll(database, "`", "``")
	query := "CREATE DATABASE IF NOT EXISTS `" + name + "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
	if _, err := admin.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	fmt.Fprintf(os.Stdout, "database ready: %s\n", database)
	return nil
}
