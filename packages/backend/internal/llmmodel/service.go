package llmmodel

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"vibebiz/internal/secret"
)

var (
	ErrInvalid    = errors.New("invalid model")
	ErrConflict   = errors.New("model code already exists")
	ErrKeyMissing = errors.New("model secret key not configured")
)

var codePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var allowedTypes = map[string]struct{}{
	"LLM": {}, "EMBEDDING": {}, "RERANK": {}, "OCR": {}, "ASR": {}, "TTS": {}, "OTHER": {},
}

var allowedProviders = map[string]struct{}{
	"OpenAI": {}, "DeepSeek": {}, "Anthropic": {}, "Qwen": {}, "Other": {},
}

type Model struct {
	ID           int64           `json:"id"`
	ModelName    string          `json:"modelName"`
	ModelCode    string          `json:"modelCode"`
	APIURL       string          `json:"apiUrl"`
	ProviderCode string          `json:"providerCode"`
	ModelType    string          `json:"modelType"`
	MaxTokens    int             `json:"maxTokens"`
	IsDefault    bool            `json:"isDefault"`
	Status       int8            `json:"status"`
	HasAPIKey    bool            `json:"hasApiKey"`
	ExtraParams  json.RawMessage `json:"extraParams,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type CreateInput struct {
	ModelName    string
	ModelCode    string
	APIURL       string
	APIKey       string
	ProviderCode string
	ModelType    string
	MaxTokens    int
	IsDefault    bool
	Status       int8
	ExtraParams  json.RawMessage
}

type Repository interface {
	List(ctx context.Context) ([]Model, error)
	Create(ctx context.Context, input CreateInput, apiKeyCipher, createdBy string) (Model, error)
}

type Service struct {
	Repo   Repository
	Cipher *secret.Cipher
}

func NewService(repo Repository, cipher *secret.Cipher) *Service {
	return &Service{Repo: repo, Cipher: cipher}
}

func (s *Service) List(ctx context.Context) ([]Model, error) {
	return s.Repo.List(ctx)
}

func (s *Service) Create(ctx context.Context, input CreateInput, createdBy string) (Model, error) {
	normalized, err := normalize(input)
	if err != nil {
		return Model{}, err
	}
	if s.Cipher == nil {
		return Model{}, ErrKeyMissing
	}
	encrypted, err := s.Cipher.Encrypt(normalized.APIKey)
	if err != nil {
		return Model{}, err
	}
	return s.Repo.Create(ctx, normalized, encrypted, createdBy)
}

func normalize(input CreateInput) (CreateInput, error) {
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.ModelCode = strings.TrimSpace(input.ModelCode)
	input.APIURL = strings.TrimSpace(input.APIURL)
	input.APIKey = strings.TrimSpace(input.APIKey)
	input.ProviderCode = strings.TrimSpace(input.ProviderCode)
	input.ModelType = strings.ToUpper(strings.TrimSpace(input.ModelType))

	if input.ModelName == "" || input.ModelCode == "" || input.APIURL == "" || input.APIKey == "" {
		return CreateInput{}, ErrInvalid
	}
	if !codePattern.MatchString(input.ModelCode) {
		return CreateInput{}, ErrInvalid
	}
	if !strings.HasPrefix(input.APIURL, "http://") && !strings.HasPrefix(input.APIURL, "https://") {
		return CreateInput{}, ErrInvalid
	}
	if _, ok := allowedTypes[input.ModelType]; !ok {
		return CreateInput{}, ErrInvalid
	}
	if input.ProviderCode == "" {
		input.ProviderCode = "Other"
	}
	if _, ok := allowedProviders[input.ProviderCode]; !ok {
		return CreateInput{}, ErrInvalid
	}
	if input.Status != 0 && input.Status != 1 {
		return CreateInput{}, ErrInvalid
	}
	if input.ModelType == "LLM" {
		if input.MaxTokens <= 0 {
			input.MaxTokens = 4096
		}
	} else {
		input.MaxTokens = 0
	}
	if len(input.ExtraParams) > 0 {
		if !json.Valid(input.ExtraParams) {
			return CreateInput{}, ErrInvalid
		}
		var object map[string]any
		if err := json.Unmarshal(input.ExtraParams, &object); err != nil {
			return CreateInput{}, ErrInvalid
		}
	}
	return input, nil
}
