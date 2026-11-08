package channelguide

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
)

type daoNotFoundRepository struct {
	*MemoryRepository
}

func (daoNotFoundRepository) Get(context.Context, string) (*model.ChannelGuide, error) {
	return nil, dao.ErrNotFound
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc := NewService(NewMemoryRepository())
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestEnsureDefaultsSeedsBuiltinChannels(t *testing.T) {
	svc := newTestService(t)
	items, err := svc.List(context.Background(), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	for _, item := range items {
		if !item.Builtin || !item.Enabled || strings.TrimSpace(item.Markdown) == "" {
			t.Fatalf("bad builtin item: %+v", item)
		}
	}
}

func TestEnsureDefaultsDoesNotOverwriteEdits(t *testing.T) {
	svc := newTestService(t)
	item, err := svc.Update(context.Background(), core.ChannelWeChat, Input{
		Title: "编辑后的微信", Summary: "s", Markdown: "# edited", Enabled: false, SortOrder: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(context.Background(), core.ChannelWeChat)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != item.Title || got.Markdown != "# edited" || got.Enabled {
		t.Fatalf("seed overwrote edits: %+v", got)
	}
}

func TestCreateCustomChannelAndPublicList(t *testing.T) {
	svc := newTestService(t)
	created, err := svc.Create(context.Background(), Input{
		Channel: "taptap_mobile", Title: "TapTap", Summary: "",
		Markdown: "# TapTap", Steps: []string{"准备资质"}, Requirements: []string{"企业主体"},
		Enabled: true, SortOrder: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Packageable {
		t.Fatal("custom channel must not be packageable")
	}
	items, err := svc.List(context.Background(), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	if _, err := svc.Create(context.Background(), Input{Channel: "taptap_mobile", Title: "dup"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestBuiltinCannotBeDeleted(t *testing.T) {
	svc := newTestService(t)
	if err := svc.Delete(context.Background(), core.ChannelWeChat); !errors.Is(err, ErrBuiltinDelete) {
		t.Fatalf("err = %v, want ErrBuiltinDelete", err)
	}
}

func TestDeleteCustomChannel(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Create(context.Background(), Input{Channel: "custom_channel", Title: "自定义", Markdown: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), "custom_channel"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(context.Background(), "custom_channel"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUploadMarkdownNormalizesAndValidates(t *testing.T) {
	svc := newTestService(t)
	item, err := svc.UploadMarkdown(context.Background(), core.ChannelWeChat, "guide.md", []byte("\xEF\xBB\xBF# 新指引\r\n第二行"))
	if err != nil {
		t.Fatal(err)
	}
	if item.Markdown != "# 新指引\n第二行" {
		t.Fatalf("markdown = %q", item.Markdown)
	}
	if _, err := svc.UploadMarkdown(context.Background(), core.ChannelWeChat, "guide.txt", []byte("x")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if _, err := svc.UploadMarkdown(context.Background(), core.ChannelWeChat, "guide.md", []byte{0xff, 0xfe}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestMarkdownFallsBackToEmbedded(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	got, err := svc.Markdown(context.Background(), core.ChannelWeChat)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "微信") {
		t.Fatalf("markdown = %q", got)
	}
}

func TestServiceRejectsInvalidInput(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	steps := make([]string, 21)
	for i := range steps {
		steps[i] = "step"
	}
	cases := []struct {
		name  string
		input Input
	}{
		{name: "blank title", input: Input{Channel: "custom_channel", Title: "   "}},
		{name: "too many steps", input: Input{Channel: "custom_channel", Title: "ok", Steps: steps}},
		{name: "step too long", input: Input{Channel: "custom_channel", Title: "ok", Steps: []string{strings.Repeat("a", 121)}}},
		{name: "sort order too large", input: Input{Channel: "custom_channel", Title: "ok", SortOrder: 10000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), tc.input); !errors.Is(err, ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestBuiltinBlankMarkdownFallsBackToEmbedded(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Update(context.Background(), core.ChannelWeChat, Input{
		Title: "微信小游戏", Summary: "s", Markdown: "", Enabled: true, SortOrder: 10,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(context.Background(), core.ChannelWeChat)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(got.Markdown) == "" {
		t.Fatal("builtin blank markdown must fall back to embedded content")
	}
	items, err := svc.List(context.Background(), true, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Channel == core.ChannelWeChat && strings.TrimSpace(item.Markdown) == "" {
			t.Fatal("list returned blank builtin markdown")
		}
	}

	created, err := svc.Create(context.Background(), Input{Channel: "custom_blank", Title: "自定义", Markdown: ""})
	if err != nil {
		t.Fatal(err)
	}
	if created.Markdown != "" {
		t.Fatalf("custom blank markdown = %q, want empty", created.Markdown)
	}
}

func TestMarkdownIgnoresDisabledBuiltin(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Update(context.Background(), core.ChannelWeChat, Input{
		Title: "微信小游戏", Summary: "s", Markdown: "# disabled guide", Enabled: false, SortOrder: 10,
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Markdown(context.Background(), core.ChannelWeChat)
	if err != nil {
		t.Fatal(err)
	}
	if got != "# disabled guide" {
		t.Fatalf("markdown = %q, want DB content", got)
	}
}

func TestServiceMapsDAOErrNotFound(t *testing.T) {
	svc := NewService(daoNotFoundRepository{NewMemoryRepository()})
	got, err := svc.Markdown(context.Background(), core.ChannelWeChat)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "微信") {
		t.Fatalf("markdown = %q", got)
	}
	if _, err := svc.Get(context.Background(), core.ChannelWeChat); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
