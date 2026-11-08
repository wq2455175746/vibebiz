package channelguide

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"vibebiz/internal/dao/model"
)

type MemoryRepository struct {
	mu    sync.Mutex
	items map[string]model.ChannelGuide
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{items: make(map[string]model.ChannelGuide)}
}

func (r *MemoryRepository) List(_ context.Context, includeDisabled bool, keyword string) ([]model.ChannelGuide, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.ChannelGuide, 0, len(r.items))
	for _, item := range r.items {
		if !includeDisabled && !item.Enabled {
			continue
		}
		if keyword != "" && !strings.Contains(item.Channel+item.Title+item.Summary, keyword) {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Channel < out[j].Channel
	})
	return out, nil
}

func (r *MemoryRepository) Get(_ context.Context, channel string) (*model.ChannelGuide, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[channel]
	if !ok {
		return nil, ErrNotFound
	}
	return &item, nil
}

func (r *MemoryRepository) Create(_ context.Context, item *model.ChannelGuide) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[item.Channel]; ok {
		return ErrConflict
	}
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now
	r.items[item.Channel] = *item
	return nil
}

func (r *MemoryRepository) Update(_ context.Context, item *model.ChannelGuide) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[item.Channel]; !ok {
		return ErrNotFound
	}
	item.UpdatedAt = time.Now().UTC()
	r.items[item.Channel] = *item
	return nil
}

func (r *MemoryRepository) Delete(_ context.Context, channel string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[channel]; !ok {
		return ErrNotFound
	}
	delete(r.items, channel)
	return nil
}

func (r *MemoryRepository) SeedDefaults(_ context.Context, items []model.ChannelGuide) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, item := range items {
		if _, ok := r.items[item.Channel]; ok {
			continue
		}
		item.CreatedAt = now
		item.UpdatedAt = now
		r.items[item.Channel] = item
	}
	return nil
}
