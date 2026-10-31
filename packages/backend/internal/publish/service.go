package publish

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid publish request")
	ErrNotFound = errors.New("publish source not found")
	ErrTooLarge = errors.New("publish source is too large")
)

const defaultMaxBytes int64 = 200 << 20

var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$`)

//go:embed share.html
var assets embed.FS

var shareTemplate = template.Must(template.ParseFS(assets, "share.html"))

type Input struct {
	Title       string
	Description string
	Tags        []string
}

type Result struct {
	Slug        string    `json:"slug"`
	Entry       string    `json:"entry"`
	PublishedAt time.Time `json:"publishedAt"`
}

type Published struct {
	ProjectID   string    `json:"projectId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	Entry       string    `json:"entry"`
	PublishedAt time.Time `json:"publishedAt"`
}

type Service struct {
	WorkspacesDir string
	PublishDir    string
	MaxBytes      int64
	Now           func() time.Time
}

func NewService(workspacesDir, publishDir string) *Service {
	return &Service{
		WorkspacesDir: workspacesDir,
		PublishDir:    publishDir,
		MaxBytes:      defaultMaxBytes,
		Now:           time.Now,
	}
}

func (s *Service) Publish(ctx context.Context, projectID string, input Input) (Result, error) {
	if !slugPattern.MatchString(projectID) {
		return Result{}, ErrInvalid
	}
	source := filepath.Join(s.WorkspacesDir, projectID)
	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		return Result{}, ErrNotFound
	}
	entry, err := publishEntry(source)
	if err != nil {
		return Result{}, err
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "未命名作品"
	}
	if len([]rune(title)) > 80 || len([]rune(input.Description)) > 500 {
		return Result{}, ErrInvalid
	}
	tags := normalizeTags(input.Tags)
	now := s.now().UTC()

	if err := os.MkdirAll(s.PublishDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create publish dir: %w", err)
	}
	staging, err := os.MkdirTemp(s.PublishDir, ".publish-"+projectID+"-")
	if err != nil {
		return Result{}, fmt.Errorf("create staging dir: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()

	gameDir := filepath.Join(staging, "game")
	if err := s.copyTree(ctx, source, gameDir); err != nil {
		return Result{}, err
	}
	gameEntry, err := publishEntry(gameDir)
	if err != nil {
		return Result{}, err
	}
	if err := writeSharePage(staging, shareData{
		Title: title, Description: strings.TrimSpace(input.Description), Tags: tags,
		GameURL: "game/" + gameEntry,
	}); err != nil {
		return Result{}, err
	}
	meta, err := json.MarshalIndent(map[string]any{
		"projectId": projectID, "title": title, "description": strings.TrimSpace(input.Description),
		"tags": tags, "entry": gameEntry, "publishedAt": now,
	}, "", "  ")
	if err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(filepath.Join(staging, "publish.json"), meta, 0o644); err != nil {
		return Result{}, fmt.Errorf("write publish metadata: %w", err)
	}

	target := filepath.Join(s.PublishDir, projectID)
	if err := os.RemoveAll(target); err != nil {
		return Result{}, fmt.Errorf("replace published app: %w", err)
	}
	if err := os.Rename(staging, target); err != nil {
		return Result{}, fmt.Errorf("commit published app: %w", err)
	}
	committed = true
	return Result{Slug: projectID, Entry: entry, PublishedAt: now}, nil
}

func (s *Service) List(ctx context.Context) ([]Published, error) {
	entries, err := os.ReadDir(s.PublishDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Published{}, nil
		}
		return nil, fmt.Errorf("read publish dir: %w", err)
	}
	items := make([]Published, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.PublishDir, entry.Name(), "publish.json"))
		if err != nil {
			continue
		}
		var item Published
		if err := json.Unmarshal(data, &item); err != nil {
			continue
		}
		if item.ProjectID == "" {
			item.ProjectID = entry.Name()
		}
		if !slugPattern.MatchString(item.ProjectID) || item.ProjectID != entry.Name() || item.Title == "" {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].PublishedAt.Equal(items[j].PublishedAt) {
			return items[i].ProjectID < items[j].ProjectID
		}
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})
	return items, nil
}

func (s *Service) RefreshSharePages(ctx context.Context) error {
	items, err := s.List(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return err
		}
		entry := item.Entry
		if entry == "" {
			entry, err = publishEntry(filepath.Join(s.PublishDir, item.ProjectID, "game"))
			if err != nil {
				continue
			}
		}
		tmp, err := os.MkdirTemp(s.PublishDir, ".share-refresh-")
		if err != nil {
			return fmt.Errorf("create share refresh dir: %w", err)
		}
		data := shareData{
			Title: item.Title, Description: item.Description, Tags: item.Tags,
			GameURL: "game/" + entry,
		}
		if err := writeSharePage(tmp, data); err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
		if err := os.Rename(filepath.Join(tmp, "index.html"), filepath.Join(s.PublishDir, item.ProjectID, "index.html")); err != nil {
			_ = os.RemoveAll(tmp)
			return fmt.Errorf("replace share page: %w", err)
		}
		if err := os.RemoveAll(tmp); err != nil {
			return fmt.Errorf("clean share refresh dir: %w", err)
		}
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, projectID string) error {
	if !slugPattern.MatchString(projectID) {
		return ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	target := filepath.Join(s.PublishDir, projectID)
	info, err := os.Stat(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return fmt.Errorf("stat published app: %w", err)
	}
	if !info.IsDir() {
		return ErrNotFound
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("delete published app: %w", err)
	}
	return nil
}

func (s *Service) FilePath(slug, urlPath string) (string, error) {
	if !slugPattern.MatchString(slug) {
		return "", ErrInvalid
	}
	root := filepath.Join(s.PublishDir, slug)
	relative := strings.TrimPrefix(filepath.Clean("/"+urlPath), string(filepath.Separator))
	if relative == "." || relative == "" {
		relative = "index.html"
	}
	full := filepath.Join(root, filepath.FromSlash(relative))
	inside, err := filepath.Rel(root, full)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", ErrInvalid
	}
	info, err := os.Stat(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrNotFound
		}
		return "", err
	}
	if info.IsDir() {
		full = filepath.Join(full, "index.html")
		if _, err := os.Stat(full); err != nil {
			return "", ErrNotFound
		}
	}
	return full, nil
}

func (s *Service) copyTree(ctx context.Context, source, target string) error {
	var total int64
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(target, 0o755)
		}
		if excluded(relative) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if total > s.maxBytes() {
			return ErrTooLarge
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		return copyFile(path, destination)
	})
}

func (s *Service) maxBytes() int64 {
	if s.MaxBytes > 0 {
		return s.MaxBytes
	}
	return defaultMaxBytes
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func publishEntry(dir string) (string, error) {
	for _, name := range []string{"index.html", "app.html"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() {
			return name, nil
		}
	}
	return "", ErrInvalid
}

func excluded(relative string) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return parts[0] == "docs" || parts[0] == "node_modules"
}

func normalizeTags(tags []string) []string {
	out := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if len([]rune(tag)) > 20 {
			tag = string([]rune(tag)[:20])
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
		if len(out) == 5 {
			break
		}
	}
	return out
}

func copyFile(source, destination string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

type shareData struct {
	Title       string
	Description string
	Tags        []string
	GameURL     string
}

func writeSharePage(dir string, data shareData) error {
	file, err := os.Create(filepath.Join(dir, "index.html"))
	if err != nil {
		return fmt.Errorf("create share page: %w", err)
	}
	defer file.Close()
	if err := shareTemplate.Execute(file, data); err != nil {
		return fmt.Errorf("render share page: %w", err)
	}
	return file.Close()
}
