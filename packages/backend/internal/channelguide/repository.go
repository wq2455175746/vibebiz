package channelguide

import (
	"context"
	"errors"

	"vibebiz/internal/dao/model"
)

var (
	ErrInvalid       = errors.New("invalid channel guide")
	ErrNotFound      = errors.New("channel guide not found")
	ErrConflict      = errors.New("channel guide already exists")
	ErrBuiltinDelete = errors.New("builtin channel guide cannot be deleted")
	ErrTooLarge      = errors.New("channel guide markdown is too large")
)

type Repository interface {
	List(ctx context.Context, includeDisabled bool, keyword string) ([]model.ChannelGuide, error)
	Get(ctx context.Context, channel string) (*model.ChannelGuide, error)
	Create(ctx context.Context, item *model.ChannelGuide) error
	Update(ctx context.Context, item *model.ChannelGuide) error
	Delete(ctx context.Context, channel string) error
	SeedDefaults(ctx context.Context, items []model.ChannelGuide) error
}
