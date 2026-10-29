package llmmodel

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return gormDB, mock
}

func TestMySQLListModels(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `llm_model`").
		WithArgs("platform", 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "model_name", "model_code", "api_url", "api_key_cipher", "provider_code", "model_type", "max_tokens", "is_default", "status", "created_at", "updated_at"}).
			AddRow(1, "DeepSeek", "deepseek-chat", "https://x", "cipher", "DeepSeek", "LLM", 4096, 1, 1, now, now))

	repo := NewMySQLRepository(db)
	items, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].IsDefault || !items[0].HasAPIKey {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
