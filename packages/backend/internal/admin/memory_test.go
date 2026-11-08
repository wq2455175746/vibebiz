package admin

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryCreateUser(t *testing.T) {
	repo := NewMemoryRepository().(*MemoryRepository)
	at := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	detail, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username:     "tester01",
		Email:        "tester01@example.com",
		Nickname:     "Tester",
		PasswordHash: "hash",
		Plan:         "pro",
		Status:       1,
		Operator:     "admin",
		At:           at,
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != "3" || detail.Username != "tester01" || detail.Name != "Tester" ||
		detail.Email != "tester01@example.com" || detail.Plan != "pro" ||
		detail.Source != "ADMIN" || detail.Status != "active" ||
		detail.EmailVerifiedAt == nil {
		t.Fatalf("detail = %+v", detail)
	}
	if repo.details[detail.ID].Username != "tester01" || repo.passwordHash[detail.ID] != "hash" {
		t.Fatalf("memory state = %+v %+v", repo.details, repo.passwordHash)
	}
}

func TestMemoryCreateUserRejectsDuplicate(t *testing.T) {
	repo := NewMemoryRepository().(*MemoryRepository)
	_, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username: "wanghg11", Email: "new@example.com", Nickname: "dup", Plan: "free", Status: 1,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}
