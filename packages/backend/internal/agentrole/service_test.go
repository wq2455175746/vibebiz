package agentrole

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type fakePresigner struct {
	putKey  string
	getKey  string
	headErr error
	putErr  error
}

func (f *fakePresigner) PresignPut(_ context.Context, key, _ string, _ int64, _ time.Duration) (string, map[string]string, error) {
	f.putKey = key
	return "https://upload/" + key, map[string]string{"Content-Type": "image/png"}, f.putErr
}

func (f *fakePresigner) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	f.getKey = key
	return "https://read/" + key, nil
}

func (f *fakePresigner) Put(_ context.Context, key, _ string, _ int64, _ io.Reader) error {
	f.putKey = key
	return f.putErr
}

func (f *fakePresigner) Head(_ context.Context, key string) (int64, string, error) {
	f.getKey = key
	return 1, "image/png", f.headErr
}

func validRole() CreateInput {
	return CreateInput{
		RoleCode:     "frank",
		RoleNameEN:   "Frank",
		RoleNameZH:   "测试工程师",
		Description:  "负责测试策略与用例。",
		SystemPrompt: "你是 Frank，测试工程师。",
		Status:       1,
		SortOrder:    60,
	}
}

func TestCreateRole(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	role, err := service.Create(context.Background(), validRole(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if role.RoleCode != "frank" || role.IsBuiltin || role.Version != 1 {
		t.Fatalf("role = %+v", role)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	cases := []CreateInput{
		{},
		{RoleCode: "Frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x"},
		{RoleCode: "bad code", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x"},
		{RoleCode: "frank", RoleNameEN: "", RoleNameZH: "F", SystemPrompt: "x"},
		{RoleCode: "frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: ""},
		{RoleCode: "frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x", Status: 2},
		{RoleCode: "frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x", AvatarKey: "avatar/x.png"},
	}
	for index, input := range cases {
		if _, err := service.Create(context.Background(), input, "admin"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d err = %v", index, err)
		}
	}
}

func TestDuplicateRoleCodeReturnsConflict(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	if _, err := service.Create(context.Background(), validRole(), "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), validRole(), "admin"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestListSortsBySortOrderThenID(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	second := validRole()
	second.RoleCode = "bob"
	second.SortOrder = 20
	first := validRole()
	first.RoleCode = "alex"
	first.SortOrder = 10
	if _, err := service.Create(context.Background(), second, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), first, "admin"); err != nil {
		t.Fatal(err)
	}
	roles, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 || roles[0].RoleCode != "alex" || roles[1].RoleCode != "bob" {
		t.Fatalf("roles = %+v", roles)
	}
}

func TestAvatarPresignAndUploadRequireStorage(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	if _, err := service.PresignAvatar(context.Background(), "a.png", "image/png", 10); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("presign err = %v", err)
	}
	if _, err := service.UploadAvatar(context.Background(), "image/png", 10, nil); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("upload err = %v", err)
	}
}

func TestAvatarKeyPrefixIsValidated(t *testing.T) {
	service := NewService(NewMemoryRepository(), &fakePresigner{}, 0)
	input := validRole()
	input.AvatarKey = "roles/avatar/ok.png"
	if _, err := service.Create(context.Background(), input, "admin"); err != nil {
		t.Fatal(err)
	}
	input.RoleCode = "other"
	input.AvatarKey = "avatar/bad.png"
	if _, err := service.Create(context.Background(), input, "admin"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestPresignAvatarBuildsKey(t *testing.T) {
	s3 := &fakePresigner{}
	service := NewService(NewMemoryRepository(), s3, time.Minute)
	result, err := service.PresignAvatar(context.Background(), "frank.png", "image/png", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if result.Key == "" || s3.putKey == "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestListActiveAndGetActive(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	active := validRole()
	active.RoleCode = "alex"
	active.Status = 1
	if _, err := service.Create(context.Background(), active, "admin"); err != nil {
		t.Fatal(err)
	}
	disabled := validRole()
	disabled.RoleCode = "old"
	disabled.Status = 0
	disabledRole, err := service.Create(context.Background(), disabled, "admin")
	if err != nil {
		t.Fatal(err)
	}

	roles, err := service.ListActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].RoleCode != "alex" {
		t.Fatalf("active roles = %+v", roles)
	}

	role, err := service.GetActive(context.Background(), roles[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if role.RoleCode != "alex" {
		t.Fatalf("role = %+v", role)
	}
	if _, err := service.GetActive(context.Background(), disabledRole.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disabled role err = %v", err)
	}
	if _, err := service.GetActive(context.Background(), 999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
