package llmmodel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"vibebiz/internal/secret"
)

func testCipher(t *testing.T) *secret.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	c, err := secret.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func validInput() CreateInput {
	return CreateInput{
		ModelName:    "DeepSeek Chat",
		ModelCode:    "deepseek-chat",
		APIURL:       "https://api.deepseek.com/anthropic",
		APIKey:       "sk-test",
		ProviderCode: "DeepSeek",
		ModelType:    "LLM",
		MaxTokens:    4096,
		Status:       1,
	}
}

func TestCreateEncryptsKeyAndStoresModel(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo, testCipher(t))

	model, err := service.Create(context.Background(), validInput(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if model.ModelCode != "deepseek-chat" || !model.HasAPIKey {
		t.Fatalf("model = %+v", model)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	service := NewService(NewMemoryRepository(), testCipher(t))
	cases := []CreateInput{
		{},
		{ModelName: "x", ModelCode: "bad code", APIURL: "https://x", APIKey: "k", ModelType: "LLM"},
		{ModelName: "x", ModelCode: "x", APIURL: "ftp://x", APIKey: "k", ModelType: "LLM"},
		{ModelName: "x", ModelCode: "x", APIURL: "https://x", APIKey: "k", ModelType: "VIDEO"},
		{ModelName: "x", ModelCode: "x", APIURL: "https://x", APIKey: "k", ModelType: "EMBEDDING", ExtraParams: json.RawMessage(`[]`)},
	}
	for index, input := range cases {
		if _, err := service.Create(context.Background(), input, "admin"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d err = %v, want ErrInvalid", index, err)
		}
	}
}

func TestCreateWithoutCipherReturnsKeyMissing(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil)
	if _, err := service.Create(context.Background(), validInput(), "admin"); !errors.Is(err, ErrKeyMissing) {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultIsUniquePerType(t *testing.T) {
	service := NewService(NewMemoryRepository(), testCipher(t))
	first := validInput()
	first.IsDefault = true
	if _, err := service.Create(context.Background(), first, "admin"); err != nil {
		t.Fatal(err)
	}
	second := validInput()
	second.ModelCode = "deepseek-reasoner"
	second.IsDefault = true
	if _, err := service.Create(context.Background(), second, "admin"); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	defaults := map[string]bool{}
	for _, item := range items {
		defaults[item.ModelCode] = item.IsDefault
	}
	if !defaults["deepseek-reasoner"] || defaults["deepseek-chat"] {
		t.Fatalf("default flags = %+v", items)
	}
}

func TestDuplicateCodeReturnsConflict(t *testing.T) {
	service := NewService(NewMemoryRepository(), testCipher(t))
	if _, err := service.Create(context.Background(), validInput(), "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), validInput(), "admin"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}
