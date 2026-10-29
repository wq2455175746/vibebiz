package llmmodel

import (
	"context"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu    sync.Mutex
	next  int64
	items []Model
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

func (r *MemoryRepository) List(context.Context) ([]Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Model, len(r.items))
	copy(out, r.items)
	return out, nil
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput, apiKeyCipher, _ string) (Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.ModelCode == input.ModelCode {
			return Model{}, ErrConflict
		}
	}
	if input.IsDefault {
		for index := range r.items {
			if r.items[index].ModelType == input.ModelType {
				r.items[index].IsDefault = false
			}
		}
	}
	r.next++
	now := time.Now().UTC()
	item := Model{
		ID: r.next, ModelName: input.ModelName, ModelCode: input.ModelCode,
		APIURL: input.APIURL, ProviderCode: input.ProviderCode, ModelType: input.ModelType,
		MaxTokens: input.MaxTokens, IsDefault: input.IsDefault, Status: input.Status,
		HasAPIKey: apiKeyCipher != "", ExtraParams: input.ExtraParams, CreatedAt: now, UpdatedAt: now,
	}
	r.items = append([]Model{item}, r.items...)
	return item, nil
}
