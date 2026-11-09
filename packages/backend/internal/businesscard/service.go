package businesscard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

var (
	ErrInvalidInput       = errors.New("invalid business card input")
	ErrInvalidProject     = errors.New("project cannot be added to card")
	ErrItemLimit          = errors.New("business card item limit reached")
	ErrItemDuplicate      = errors.New("project already in card")
	ErrFeaturedNotFound   = errors.New("featured project is not in card")
	ErrNoItems            = errors.New("business card has no visible projects")
	ErrStorageUnavailable = errors.New("card storage is not configured")
	ErrInvalidFile        = errors.New("invalid featured file")
	ErrFileTooLarge       = errors.New("featured file is too large")
)

type ProjectReader interface {
	Get(ctx context.Context, id string) (*model.Project, error)
}

type Publisher interface {
	List(ctx context.Context) ([]publish.Published, error)
}

type S3 interface {
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

type Service struct {
	Repo             Repository
	Projects         ProjectReader
	Publisher        Publisher
	S3               S3
	PublicBaseURL    string
	PresignExpiry    time.Duration
	MaxFeaturedBytes int64
	Now              func() time.Time
	NewID            func() string
}

func NewService(repo Repository, projects ProjectReader, publisher Publisher, s3 S3, publicBaseURL string, presignExpiry time.Duration) *Service {
	if presignExpiry <= 0 {
		presignExpiry = 15 * time.Minute
	}
	return &Service{
		Repo: repo, Projects: projects, Publisher: publisher, S3: s3,
		PublicBaseURL: publicBaseURL, PresignExpiry: presignExpiry,
		MaxFeaturedBytes: 2 << 20, Now: time.Now, NewID: uuid.NewString,
	}
}

type Card struct {
	ID           string     `json:"id"`
	OwnerUserID  int64      `json:"ownerUserId"`
	OwnerName    string     `json:"ownerName"`
	AvatarURL    string     `json:"avatarUrl"`
	Headline     string     `json:"headline"`
	Tags         []string   `json:"tags"`
	Items        []CardItem `json:"items"`
	Featured     *Featured  `json:"featured"`
	Published    bool       `json:"published"`
	CommentCount int        `json:"commentCount"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type CardItem struct {
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverURL    string `json:"coverUrl"`
	Type        string `json:"type"`
	ShareURL    string `json:"shareUrl"`
}

type Featured struct {
	Kind      string `json:"kind"`
	ProjectID string `json:"projectId,omitempty"`
	FileType  string `json:"fileType,omitempty"`
	Name      string `json:"name,omitempty"`
	Size      int64  `json:"size,omitempty"`
	URL       string `json:"url,omitempty"`
}

func (s *Service) GetMine(ctx context.Context, userID int64, workspaceIDs []int64) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.resolve(ctx, rec, workspaceSet(workspaceIDs), true)
}

func (s *Service) GetPublic(ctx context.Context, publicID string) (*Card, error) {
	rec, err := s.Repo.GetByPublicID(ctx, publicID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if rec.Hidden {
		return nil, ErrNotFound
	}
	return s.resolve(ctx, rec, nil, false)
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, workspaceIDs []int64, headline string, tags []string) (*Card, error) {
	headline = strings.TrimSpace(headline)
	if utf8.RuneCountInString(headline) > 80 {
		return nil, ErrInvalidInput
	}
	normalized, err := normalizeTags(tags)
	if err != nil {
		return nil, err
	}
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.UpdateProfile(ctx, rec.ID, headline, normalized); err != nil {
		return nil, err
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) AddProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	items, err := s.Repo.ListItems(ctx, rec.ID)
	if err != nil {
		return nil, err
	}
	if len(items) >= 5 {
		return nil, ErrItemLimit
	}
	for _, item := range items {
		if item.ProjectID == projectID {
			return nil, ErrItemDuplicate
		}
	}
	if _, _, err := s.validateProject(ctx, projectID, workspaceIDs, true); err != nil {
		return nil, err
	}
	sortOrder := 0
	for _, item := range items {
		if item.SortOrder >= sortOrder {
			sortOrder = item.SortOrder + 1
		}
	}
	if err := s.Repo.AddItem(ctx, rec.ID, projectID, sortOrder); err != nil {
		return nil, err
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) RemoveProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err := s.Repo.RemoveItem(ctx, rec.ID, projectID); err != nil {
		return nil, err
	}
	if rec.FeaturedType == "project" && rec.FeaturedProjectID == projectID {
		if err := s.Repo.ClearFeatured(ctx, rec.ID); err != nil {
			return nil, err
		}
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) SetFeaturedProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	items, err := s.Repo.ListItems(ctx, rec.ID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, item := range items {
		if item.ProjectID == projectID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrFeaturedNotFound
	}
	oldKey := rec.FeaturedS3Key
	if err := s.Repo.SetFeaturedProject(ctx, rec.ID, projectID); err != nil {
		return nil, err
	}
	if oldKey != "" && s.S3 != nil {
		_ = s.S3.Delete(ctx, oldKey)
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) ClearFeatured(ctx context.Context, userID int64, workspaceIDs []int64) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	oldKey := rec.FeaturedS3Key
	if err := s.Repo.ClearFeatured(ctx, rec.ID); err != nil {
		return nil, err
	}
	if oldKey != "" && s.S3 != nil {
		_ = s.S3.Delete(ctx, oldKey)
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) UploadFeatured(ctx context.Context, userID int64, workspaceIDs []int64, contentType, filename string, size int64, body io.Reader) (*Card, error) {
	contentType = strings.TrimSpace(contentType)
	if size <= 0 || size > s.MaxFeaturedBytes {
		return nil, ErrFileTooLarge
	}
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != size {
		return nil, ErrInvalidFile
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, ErrInvalidFile
	}
	contentType = strings.ToLower(mediaType)
	fileType, ext, ok := detectFeatured(data, contentType)
	if !ok {
		return nil, ErrInvalidFile
	}
	if s.S3 == nil {
		return nil, ErrStorageUnavailable
	}
	oldKey := rec.FeaturedS3Key
	key := fmt.Sprintf("cards/%d/%s.%s", userID, s.nextID(), ext)
	if err := s.S3.Put(ctx, key, contentType, size, bytes.NewReader(data)); err != nil {
		return nil, err
	}
	if err := s.Repo.SetFeaturedFile(ctx, rec.ID, key, sanitizeFilename(filename), fileType, size); err != nil {
		_ = s.S3.Delete(ctx, key)
		return nil, err
	}
	if oldKey != "" && oldKey != key {
		_ = s.S3.Delete(ctx, oldKey)
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func detectFeatured(data []byte, contentType string) (string, string, bool) {
	switch contentType {
	case "application/pdf":
		if bytes.HasPrefix(data, []byte("%PDF-")) {
			return "pdf", "pdf", true
		}
	case "image/png":
		if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
			return "image", "png", true
		}
	case "image/jpeg":
		if bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) {
			return "image", "jpg", true
		}
	case "image/gif":
		if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
			return "image", "gif", true
		}
	case "image/webp":
		if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
			return "image", "webp", true
		}
	}
	return "", "", false
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "." || name == "/" {
		name = ""
	}
	runes := []rune(name)
	if len(runes) > 255 {
		name = string(runes[:255])
	}
	return name
}

func (s *Service) Publish(ctx context.Context, userID int64, workspaceIDs []int64) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil {
		return nil, err
	}
	card, err := s.resolve(ctx, rec, workspaceSet(workspaceIDs), true)
	if err != nil {
		return nil, err
	}
	if len(card.Items) == 0 {
		return nil, ErrNoItems
	}
	if err := s.Repo.Publish(ctx, rec.ID, s.now()); err != nil {
		return nil, err
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) resolve(ctx context.Context, rec *CardRecord, allowed map[int64]bool, enforce bool) (*Card, error) {
	published, err := s.publishMap(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]CardItem, 0, len(rec.Items))
	for _, item := range rec.Items {
		project, err := s.Projects.Get(ctx, item.ProjectID)
		if err != nil || project.IsHidden {
			continue
		}
		if enforce && !allowed[project.WorkspaceID] {
			continue
		}
		pub, ok := published[item.ProjectID]
		if !ok {
			continue
		}
		items = append(items, s.cardItem(ctx, project, pub))
	}
	tags := rec.Tags
	if tags == nil {
		tags = []string{}
	}
	return &Card{
		ID: rec.PublicID, OwnerUserID: rec.UserID, OwnerName: rec.OwnerName,
		AvatarURL: s.avatarURL(ctx, rec.OwnerAvatarS3Path), Headline: rec.Headline, Tags: tags,
		Items: items, Featured: s.resolveFeatured(ctx, rec, items), Published: rec.Published,
		CommentCount: rec.CommentCount, CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
	}, nil
}

func (s *Service) cardItem(ctx context.Context, project *model.Project, pub publish.Published) CardItem {
	name := pub.Title
	if name == "" {
		name = project.Name
	}
	cover := project.CoverURL
	if project.CoverS3Path != "" && s.S3 != nil {
		if signed, err := s.S3.PresignGet(ctx, project.CoverS3Path, s.resizeExpiry()); err == nil {
			cover = signed
		}
	}
	return CardItem{
		ProjectID: project.ID, Name: name, Description: pub.Description,
		CoverURL: cover, Type: contentKind(project.PreviewPath),
		ShareURL: strings.TrimRight(s.PublicBaseURL, "/") + "/p/" + project.ID + "/",
	}
}

func (s *Service) resolveFeatured(ctx context.Context, rec *CardRecord, items []CardItem) *Featured {
	switch rec.FeaturedType {
	case "project":
		for _, item := range items {
			if item.ProjectID == rec.FeaturedProjectID {
				return &Featured{Kind: "project", ProjectID: item.ProjectID}
			}
		}
	case "pdf", "image":
		if rec.FeaturedS3Key == "" || s.S3 == nil {
			return nil
		}
		url, err := s.S3.PresignGet(ctx, rec.FeaturedS3Key, s.resizeExpiry())
		if err != nil {
			return nil
		}
		return &Featured{Kind: "file", FileType: rec.FeaturedType, Name: rec.FeaturedFileName, Size: rec.FeaturedFileSize, URL: url}
	}
	return nil
}

func (s *Service) avatarURL(ctx context.Context, key string) string {
	if key == "" || s.S3 == nil {
		return ""
	}
	url, err := s.S3.PresignGet(ctx, key, s.resizeExpiry())
	if err != nil {
		return ""
	}
	return url
}

func (s *Service) publishMap(ctx context.Context) (map[string]publish.Published, error) {
	items, err := s.Publisher.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]publish.Published, len(items))
	for _, item := range items {
		out[item.ProjectID] = item
	}
	return out, nil
}

func (s *Service) validateProject(ctx context.Context, projectID string, workspaceIDs []int64, enforce bool) (*model.Project, publish.Published, error) {
	project, err := s.Projects.Get(ctx, projectID)
	if err != nil || project.IsHidden {
		return nil, publish.Published{}, ErrInvalidProject
	}
	if enforce && !workspaceSet(workspaceIDs)[project.WorkspaceID] {
		return nil, publish.Published{}, ErrInvalidProject
	}
	published, err := s.publishMap(ctx)
	if err != nil {
		return nil, publish.Published{}, err
	}
	pub, ok := published[projectID]
	if !ok {
		return nil, publish.Published{}, ErrInvalidProject
	}
	return project, pub, nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) nextID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewString()
}

func (s *Service) resizeExpiry() time.Duration {
	if s.PresignExpiry > 0 {
		return s.PresignExpiry
	}
	return 15 * time.Minute
}

func workspaceSet(ids []int64) map[int64]bool {
	out := make(map[int64]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func normalizeTags(tags []string) ([]string, error) {
	out := make([]string, 0, 5)
	seen := map[string]bool{}
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > 12 {
			return nil, ErrInvalidInput
		}
		if seen[tag] {
			continue
		}
		seen[tag] = true
		out = append(out, tag)
	}
	if len(out) > 5 {
		return nil, ErrInvalidInput
	}
	return out, nil
}

func contentKind(previewPath string) string {
	name := strings.ToLower(strings.TrimSpace(previewPath))
	if i := strings.IndexAny(name, "?#"); i >= 0 {
		name = name[:i]
	}
	switch {
	case strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".htm"):
		return "html"
	case strings.HasSuffix(name, ".pdf"):
		return "pdf"
	case strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".webm") || strings.HasSuffix(name, ".mov"):
		return "video"
	case strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".doc") || strings.HasSuffix(name, ".docx"):
		return "document"
	default:
		ext := strings.TrimPrefix(filepath.Ext(name), ".")
		switch ext {
		case "png", "jpg", "jpeg", "webp", "gif", "svg":
			return "image"
		}
	}
	return "html"
}

type DiscoverResult struct {
	Cards    []Card `json:"cards"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type AdminRow struct {
	ID           string    `json:"id"`
	OwnerUserID  int64     `json:"ownerUserId"`
	OwnerName    string    `json:"ownerName"`
	Headline     string    `json:"headline"`
	Tags         []string  `json:"tags"`
	ProjectCount int       `json:"projectCount"`
	FeaturedType string    `json:"featuredType"`
	Published    bool      `json:"published"`
	Hidden       bool      `json:"hidden"`
	Status       string    `json:"status"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AdminListResult struct {
	Rows     []AdminRow `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

type PublicCardSummary struct {
	ID        string `json:"id"`
	OwnerName string `json:"ownerName"`
	AvatarURL string `json:"avatarUrl"`
}

type PublicProject struct {
	Card PublicCardSummary `json:"card"`
	Item CardItem          `json:"item"`
}

func (s *Service) Discover(ctx context.Context, page, pageSize int) (*DiscoverResult, error) {
	page, pageSize = normalizePage(page, pageSize, 12, 50)
	records, total, err := s.Repo.Discover(ctx, page, pageSize)
	if err != nil {
		return nil, err
	}
	cards := make([]Card, 0, len(records))
	for _, rec := range records {
		card, err := s.resolve(ctx, rec, nil, false)
		if err != nil {
			return nil, err
		}
		if len(card.Items) == 0 {
			continue
		}
		cards = append(cards, *card)
	}
	return &DiscoverResult{Cards: cards, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) AdminList(ctx context.Context, keyword string, page, pageSize int) (*AdminListResult, error) {
	page, pageSize = normalizePage(page, pageSize, 10, 100)
	records, total, err := s.Repo.AdminList(ctx, keyword, page, pageSize)
	if err != nil {
		return nil, err
	}
	rows := make([]AdminRow, 0, len(records))
	for _, rec := range records {
		tags := rec.Tags
		if tags == nil {
			tags = []string{}
		}
		rows = append(rows, AdminRow{
			ID: rec.PublicID, OwnerUserID: rec.UserID, OwnerName: rec.OwnerName,
			Headline: rec.Headline, Tags: tags, ProjectCount: len(rec.Items),
			FeaturedType: featuredLabel(rec.FeaturedType),
			Published:    rec.Published, Hidden: rec.Hidden, Status: cardStatus(rec),
			UpdatedAt: rec.UpdatedAt,
		})
	}
	return &AdminListResult{Rows: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) AdminSetHidden(ctx context.Context, publicID string, hidden bool) (*AdminRow, error) {
	rec, err := s.Repo.GetByPublicID(ctx, publicID)
	if err != nil {
		return nil, ErrNotFound
	}
	if err := s.Repo.SetHidden(ctx, rec.ID, hidden); err != nil {
		return nil, err
	}
	return &AdminRow{ID: rec.PublicID, Hidden: hidden, Status: cardStatus(&CardRecord{Hidden: hidden, Published: rec.Published})}, nil
}

func (s *Service) AdminDelete(ctx context.Context, publicID string) error {
	rec, err := s.Repo.GetByPublicID(ctx, publicID)
	if err != nil {
		return ErrNotFound
	}
	key, err := s.Repo.Delete(ctx, rec.ID)
	if err != nil {
		return err
	}
	if key != "" && s.S3 != nil {
		_ = s.S3.Delete(ctx, key)
	}
	return nil
}

func (s *Service) PublicProject(ctx context.Context, projectID string) (*PublicProject, error) {
	project, pub, err := s.validateProject(ctx, projectID, nil, false)
	if err != nil {
		if errors.Is(err, ErrInvalidProject) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	ownerName := ""
	avatar := ""
	cardID := ""
	if rec, err := s.Repo.FindByProject(ctx, projectID); err == nil && rec != nil && !rec.Hidden {
		cardID = rec.PublicID
		ownerName = rec.OwnerName
		avatar = s.avatarURL(ctx, rec.OwnerAvatarS3Path)
	} else if owner, err := s.Repo.OwnerProfile(ctx, project.OwnerUserID); err == nil {
		ownerName = owner.Nickname
		if ownerName == "" {
			ownerName = owner.Username
		}
		avatar = s.avatarURL(ctx, owner.AvatarS3Path)
	}
	return &PublicProject{
		Card: PublicCardSummary{ID: cardID, OwnerName: ownerName, AvatarURL: avatar},
		Item: s.cardItem(ctx, project, pub),
	}, nil
}

func normalizePage(page, pageSize, defaultSize, maxSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultSize
	}
	if pageSize > maxSize {
		pageSize = maxSize
	}
	return page, pageSize
}

func featuredLabel(value string) string {
	switch value {
	case "project":
		return "项目"
	case "pdf":
		return "PDF"
	case "image":
		return "图片"
	default:
		return "—"
	}
}

func cardStatus(rec *CardRecord) string {
	if rec.Hidden {
		return "已隐藏"
	}
	if !rec.Published {
		return "未发布"
	}
	return "正常"
}
