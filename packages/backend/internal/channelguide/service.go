package channelguide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/channelpkg/guides"
	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
)

const (
	MaxMarkdownBytes = 512 << 10
	maxLines         = 20
	maxLineLength    = 120
)

var channelPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

var packageableChannels = map[string]bool{
	core.ChannelWeChat: true,
	core.ChannelDouyin: true,
	core.ChannelSteam:  true,
}

type Input struct {
	Channel      string
	Title        string
	Summary      string
	Markdown     string
	Steps        []string
	Requirements []string
	Enabled      bool
	SortOrder    int
}

type Item struct {
	Channel      string    `json:"channel"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Markdown     string    `json:"markdown"`
	Steps        []string  `json:"steps"`
	Requirements []string  `json:"requirements"`
	Enabled      bool      `json:"enabled"`
	Builtin      bool      `json:"builtin"`
	Packageable  bool      `json:"packageable"`
	SortOrder    int       `json:"sortOrder"`
	UpdatedAt    time.Time `json:"-"`
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	items := make([]model.ChannelGuide, 0, 3)
	for _, def := range guides.List() {
		items = append(items, model.ChannelGuide{
			Channel: def.Channel, Title: def.Title, Summary: def.Summary,
			Markdown: def.Markdown, Steps: encodeLines(def.Steps), Requirements: encodeLines(def.Requirements),
			Enabled: true, Builtin: true, SortOrder: defaultSort(def.Channel),
		})
	}
	return s.repo.SeedDefaults(ctx, items)
}

func defaultSort(channel string) int {
	switch channel {
	case core.ChannelWeChat:
		return 10
	case core.ChannelDouyin:
		return 20
	case core.ChannelSteam:
		return 30
	default:
		return 100
	}
}

func (s *Service) List(ctx context.Context, includeDisabled bool, keyword string) ([]Item, error) {
	items, err := s.repo.List(ctx, includeDisabled, strings.TrimSpace(keyword))
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(items))
	for i := range items {
		out = append(out, toItem(&items[i]))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, channel string) (Item, error) {
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		if isNotFound(err) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	return toItem(item), nil
}

func (s *Service) Create(ctx context.Context, input Input) (Item, error) {
	item, err := s.normalizeNew(input)
	if err != nil {
		return Item{}, err
	}
	if err := s.repo.Create(ctx, item); err != nil {
		if isDuplicate(err) {
			return Item{}, ErrConflict
		}
		return Item{}, err
	}
	return toItem(item), nil
}

func (s *Service) Update(ctx context.Context, channel string, input Input) (Item, error) {
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		if isNotFound(err) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	normalized, err := s.apply(item, input)
	if err != nil {
		return Item{}, err
	}
	if err := s.repo.Update(ctx, normalized); err != nil {
		return Item{}, err
	}
	return toItem(normalized), nil
}

func (s *Service) Delete(ctx context.Context, channel string) error {
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if item.Builtin {
		return ErrBuiltinDelete
	}
	if err := s.repo.Delete(ctx, channel); err != nil {
		if isNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (s *Service) UploadMarkdown(ctx context.Context, channel, filename string, data []byte) (Item, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".md" && ext != ".markdown" {
		return Item{}, ErrInvalid
	}
	text, err := normalizeMarkdown(data)
	if err != nil {
		return Item{}, err
	}
	if strings.TrimSpace(text) == "" {
		return Item{}, ErrInvalid
	}
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		if isNotFound(err) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	item.Markdown = text
	if err := s.repo.Update(ctx, item); err != nil {
		if isNotFound(err) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	return toItem(item), nil
}

func (s *Service) Markdown(ctx context.Context, channel string) (string, error) {
	item, err := s.repo.Get(ctx, channel)
	if isNotFound(err) {
		return guides.Markdown(channel), nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(item.Markdown) == "" {
		return guides.Markdown(channel), nil
	}
	return item.Markdown, nil
}

func (s *Service) normalizeNew(input Input) (*model.ChannelGuide, error) {
	channel := strings.TrimSpace(input.Channel)
	if !channelPattern.MatchString(channel) {
		return nil, ErrInvalid
	}
	item := &model.ChannelGuide{Channel: channel, Builtin: false}
	return s.apply(item, input)
}

func (s *Service) apply(item *model.ChannelGuide, input Input) (*model.ChannelGuide, error) {
	title := strings.TrimSpace(input.Title)
	summary := strings.TrimSpace(input.Summary)
	steps, err := normalizeLines(input.Steps)
	if err != nil {
		return nil, err
	}
	requirements, err := normalizeLines(input.Requirements)
	if err != nil {
		return nil, err
	}
	markdown, err := normalizeMarkdown([]byte(input.Markdown))
	if err != nil {
		return nil, err
	}
	if title == "" || len([]rune(title)) > 64 || len([]rune(summary)) > 255 {
		return nil, ErrInvalid
	}
	if input.SortOrder < 0 || input.SortOrder > 9999 {
		return nil, ErrInvalid
	}
	item.Title = title
	item.Summary = summary
	item.Markdown = markdown
	item.Steps = encodeLines(steps)
	item.Requirements = encodeLines(requirements)
	item.Enabled = input.Enabled
	item.SortOrder = input.SortOrder
	return item, nil
}

func normalizeLines(values []string) ([]string, error) {
	if len(values) > maxLines {
		return nil, ErrInvalid
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len([]rune(value)) > maxLineLength {
			return nil, ErrInvalid
		}
		out = append(out, value)
	}
	return out, nil
}

func normalizeMarkdown(data []byte) (string, error) {
	if len(data) > MaxMarkdownBytes {
		return "", ErrTooLarge
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return "", ErrInvalid
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text, nil
}

func isNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, dao.ErrNotFound)
}

func isDuplicate(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "1062")
}

func toItem(item *model.ChannelGuide) Item {
	markdown := item.Markdown
	if strings.TrimSpace(markdown) == "" && item.Builtin && packageableChannels[item.Channel] {
		markdown = guides.Markdown(item.Channel)
	}
	return Item{
		Channel: item.Channel, Title: item.Title, Summary: item.Summary,
		Markdown: markdown, Steps: decodeLines(item.Steps), Requirements: decodeLines(item.Requirements),
		Enabled: item.Enabled, Builtin: item.Builtin,
		Packageable: packageableChannels[item.Channel], SortOrder: item.SortOrder, UpdatedAt: item.UpdatedAt,
	}
}

func encodeLines(values []string) model.JSON {
	data, _ := json.Marshal(values)
	return data
}

func decodeLines(data model.JSON) []string {
	var out []string
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	if out == nil {
		out = []string{}
	}
	return out
}
