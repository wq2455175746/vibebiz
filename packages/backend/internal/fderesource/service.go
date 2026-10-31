package fderesource

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"vibebiz/internal/dao/model"
)

var (
	ErrInvalid  = errors.New("invalid fde resource")
	ErrNotFound = errors.New("fde resource not found")
)

type Resource struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Contact   string    `json:"contact"`
	Skills    []string  `json:"skills"`
	Region    string    `json:"region"`
	Note      string    `json:"note"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Input struct {
	Name    string
	Contact string
	Skills  []string
	Region  string
	Note    string
	Enabled bool
}

type Store interface {
	List(ctx context.Context, enabledOnly bool) ([]model.FdeResource, error)
	Get(ctx context.Context, id int64) (*model.FdeResource, error)
	Create(ctx context.Context, item *model.FdeResource) error
	Update(ctx context.Context, item *model.FdeResource) error
}

type Service struct {
	Store Store
}

func NewService(store Store) *Service {
	return &Service{Store: store}
}

func (s *Service) List(ctx context.Context, enabledOnly bool) ([]Resource, error) {
	items, err := s.Store.List(ctx, enabledOnly)
	if err != nil {
		return nil, err
	}
	resources := make([]Resource, 0, len(items))
	for _, item := range items {
		resources = append(resources, toResource(item))
	}
	return resources, nil
}

func (s *Service) Create(ctx context.Context, input Input) (Resource, error) {
	item, err := normalize(input)
	if err != nil {
		return Resource{}, err
	}
	if err := s.Store.Create(ctx, item); err != nil {
		return Resource{}, err
	}
	return toResource(*item), nil
}

func (s *Service) Update(ctx context.Context, id int64, input Input) (Resource, error) {
	item, err := normalize(input)
	if err != nil {
		return Resource{}, err
	}
	item.ID = id
	if err := s.Store.Update(ctx, item); err != nil {
		return Resource{}, err
	}
	saved, err := s.Store.Get(ctx, id)
	if err != nil {
		return Resource{}, err
	}
	return toResource(*saved), nil
}

func normalize(input Input) (*model.FdeResource, error) {
	name := strings.TrimSpace(input.Name)
	contact := strings.TrimSpace(input.Contact)
	region := strings.TrimSpace(input.Region)
	note := strings.TrimSpace(input.Note)
	if name == "" || contact == "" {
		return nil, ErrInvalid
	}
	if len([]rune(name)) > 64 || len([]rune(contact)) > 128 || len([]rune(region)) > 64 || len([]rune(note)) > 500 {
		return nil, ErrInvalid
	}
	skills := make([]string, 0, len(input.Skills))
	seen := make(map[string]struct{}, len(input.Skills))
	for _, skill := range input.Skills {
		skill = strings.TrimSpace(skill)
		if skill == "" {
			continue
		}
		if len([]rune(skill)) > 32 {
			return nil, ErrInvalid
		}
		if _, exists := seen[skill]; exists {
			continue
		}
		seen[skill] = struct{}{}
		skills = append(skills, skill)
	}
	if len(skills) > 20 {
		return nil, ErrInvalid
	}
	raw, err := json.Marshal(skills)
	if err != nil {
		return nil, err
	}
	return &model.FdeResource{
		Name: name, Contact: contact, Skills: raw, Region: region, Note: note, Enabled: input.Enabled,
	}, nil
}

func toResource(item model.FdeResource) Resource {
	skills := make([]string, 0)
	if len(item.Skills) > 0 {
		_ = json.Unmarshal(item.Skills, &skills)
	}
	return Resource{
		ID: item.ID, Name: item.Name, Contact: item.Contact, Skills: skills, Region: item.Region,
		Note: item.Note, Enabled: item.Enabled, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
