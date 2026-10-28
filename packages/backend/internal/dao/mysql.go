package dao

import (
	"context"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type MySQLConfig struct {
	Host            string
	Username        string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func (c MySQLConfig) DSN() string {
	cfg := mysqldriver.Config{
		User:      c.Username,
		Passwd:    c.Password,
		Net:       "tcp",
		Addr:      c.Host,
		DBName:    c.Database,
		ParseTime: true,
		Loc:       time.Local,
		Collation: "utf8mb4_unicode_ci",
	}
	return cfg.FormatDSN()
}

func OpenMySQL(ctx context.Context, cfg MySQLConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	sqlDB.SetMaxOpenConns(defaultInt(cfg.MaxOpenConns, 20))
	sqlDB.SetMaxIdleConns(defaultInt(cfg.MaxIdleConns, 10))
	sqlDB.SetConnMaxLifetime(defaultDuration(cfg.ConnMaxLifetime, 30*time.Minute))

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	return db, nil
}

func CloseMySQL(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	return sqlDB.Close()
}

func defaultInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func defaultDuration(value, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}
	return fallback
}
