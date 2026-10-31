package resource

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
)

const (
	MaxFileBytes         int64 = 20 << 20
	MaxProjectBytes      int64 = 200 << 20
	MaxCloneFileBytes    int64 = 100 << 20
	MaxCloneProjectBytes int64 = 500 << 20
	MaxUploadBytes       int64 = MaxCloneFileBytes

	PurposeDefault       = ""
	PurposeCloneImage    = "clone_image"
	PurposeCloneUnitTest = "clone_unit_test"
	PurposeCloneDocs     = "clone_docs"
)

var (
	ErrNotFound      = errors.New("resource not found")
	ErrInvalidFile   = errors.New("invalid resource file")
	ErrTooLarge      = errors.New("resource is too large")
	ErrProjectFull   = errors.New("project resource quota exceeded")
	ErrUnavailable   = errors.New("resource storage unavailable")
	ErrNotUploadable = errors.New("resource is not uploadable")
)

type S3 interface {
	PresignPut(ctx context.Context, key, contentType string, size int64, expiry time.Duration) (string, map[string]string, error)
	Head(ctx context.Context, key string) (int64, string, error)
	Copy(ctx context.Context, srcKey, dstKey, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, int64, error)
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	Delete(ctx context.Context, key string) error
}

type Sandbox interface {
	WriteResource(ctx context.Context, projectID string, meta ResourceMeta, body io.Reader, size int64) error
	DeleteResource(ctx context.Context, projectID, resourceID string) error
}

type Repository interface {
	Create(ctx context.Context, item *model.ProjectResource) error
	GetByResourceID(ctx context.Context, resourceID string) (*model.ProjectResource, error)
	ListByProject(ctx context.Context, projectID string) ([]model.ProjectResource, error)
	ListReadyByProject(ctx context.Context, projectID string) ([]model.ProjectResource, error)
	SumReadyByProject(ctx context.Context, projectID string) (int64, error)
	Update(ctx context.Context, item *model.ProjectResource) error
	SoftDelete(ctx context.Context, resourceID string) error
}

type Service struct {
	Repo    Repository
	S3      S3
	Sandbox Sandbox
	Expiry  time.Duration
	Now     func() time.Time
	NewID   func() string
}

type PresignInput struct {
	Filename    string
	ContentType string
	Size        int64
	Purpose     string
}

type PresignResult struct {
	ResourceID string            `json:"resourceId"`
	UploadURL  string            `json:"uploadUrl"`
	Key        string            `json:"key"`
	Headers    map[string]string `json:"headers"`
}

type ResourceMeta struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	StorageName string `json:"storageName"`
	ContentType string `json:"contentType"`
	S3Key       string `json:"s3Key"`
	Size        int64  `json:"size"`
	Kind        string `json:"kind"`
	Purpose     string `json:"purpose"`
}

type Item struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	StorageName string    `json:"storageName"`
	Path        string    `json:"path"`
	ContentType string    `json:"contentType"`
	Size        int64     `json:"size"`
	Kind        string    `json:"kind"`
	S3Key       string    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ListResult struct {
	Resources  []Item `json:"resources"`
	TotalBytes int64  `json:"totalBytes"`
}

func NewService(repo Repository, s3 S3, sandbox Sandbox) *Service {
	return &Service{Repo: repo, S3: s3, Sandbox: sandbox, Expiry: 15 * time.Minute, Now: time.Now, NewID: uuid.NewString}
}

func (s *Service) Presign(ctx context.Context, workspaceID, userID int64, input PresignInput) (PresignResult, error) {
	if s.S3 == nil {
		return PresignResult{}, ErrUnavailable
	}
	if workspaceID == 0 || userID == 0 {
		return PresignResult{}, ErrInvalidFile
	}
	original := filepath.Base(strings.TrimSpace(input.Filename))
	if original == "" || original == "." {
		return PresignResult{}, ErrInvalidFile
	}
	ext := strings.ToLower(filepath.Ext(original))
	purpose, ok := normalizePurpose(input.Purpose)
	if !ok {
		return PresignResult{}, ErrInvalidFile
	}
	maxFile, _ := purposeLimits(purpose)
	contentType, kind, err := validateForPurpose(original, ext, input.ContentType, purpose)
	if err != nil {
		return PresignResult{}, err
	}
	if input.Size <= 0 || input.Size > maxFile {
		return PresignResult{}, ErrTooLarge
	}
	resourceID := s.NewID()
	storageName := resourceID + ext
	key := fmt.Sprintf("staging/%d/%d/%s/%s", workspaceID, userID, resourceID, storageName)
	uploadURL, headers, err := s.S3.PresignPut(ctx, key, contentType, input.Size, s.Expiry)
	if err != nil {
		return PresignResult{}, err
	}
	item := &model.ProjectResource{
		ResourceID:   resourceID,
		WorkspaceID:  workspaceID,
		UserID:       userID,
		OriginalName: original,
		DisplayName:  original,
		StorageName:  storageName,
		ContentType:  contentType,
		SizeBytes:    input.Size,
		Kind:         kind,
		Purpose:      purpose,
		S3StagingKey: key,
		Status:       "staging",
	}
	if err := s.Repo.Create(ctx, item); err != nil {
		_ = s.S3.Delete(ctx, key)
		return PresignResult{}, err
	}
	return PresignResult{ResourceID: resourceID, UploadURL: uploadURL, Key: key, Headers: headers}, nil
}

func (s *Service) UploadStaging(ctx context.Context, workspaceID, userID int64, resourceID string, body io.Reader, size int64, contentType string) error {
	if s.S3 == nil {
		return ErrUnavailable
	}
	item, err := s.Repo.GetByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if item.WorkspaceID != workspaceID || item.UserID != userID || item.ProjectPublicID != nil || item.Status != "staging" {
		return ErrNotFound
	}
	maxFile, _ := purposeLimits(item.Purpose)
	normalized, _, err := validateForPurpose(item.OriginalName, filepath.Ext(item.OriginalName), contentType, item.Purpose)
	if err != nil {
		return err
	}
	if normalized != item.ContentType {
		return ErrInvalidFile
	}
	if size <= 0 || size > maxFile || size != item.SizeBytes {
		return ErrTooLarge
	}
	return s.S3.Put(ctx, item.S3StagingKey, item.ContentType, size, body)
}

func (s *Service) Adopt(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) (Item, error) {
	if s.S3 == nil {
		return Item{}, ErrUnavailable
	}
	item, err := s.Repo.GetByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return Item{}, ErrNotFound
		}
		return Item{}, err
	}
	if item.WorkspaceID != workspaceID || item.UserID != userID {
		return Item{}, ErrNotFound
	}
	if item.Status == "ready" {
		return toItem(*item), nil
	}
	if item.Status == "deleted" || item.Status == "staging" && item.ProjectPublicID != nil {
		return Item{}, ErrNotFound
	}
	size, contentType, err := s.S3.Head(ctx, item.S3StagingKey)
	if err != nil {
		return Item{}, err
	}
	maxFile, maxProject := purposeLimits(item.Purpose)
	if size <= 0 || size > maxFile || size != item.SizeBytes {
		s.fail(ctx, item)
		_ = s.S3.Delete(ctx, item.S3StagingKey)
		return Item{}, ErrTooLarge
	}
	normalized, kind, err := validateForPurpose(item.OriginalName, filepath.Ext(item.OriginalName), contentType, item.Purpose)
	if err != nil {
		s.fail(ctx, item)
		_ = s.S3.Delete(ctx, item.S3StagingKey)
		return Item{}, err
	}
	existing, err := s.Repo.ListByProject(ctx, projectID)
	if err != nil {
		return Item{}, err
	}
	item.ProjectPublicID = &projectID
	item.ContentType = normalized
	item.Kind = kind
	item.DisplayName = uniqueName(item.OriginalName, existing, resourceID)
	item.Status = "uploaded"
	if err := s.Repo.Update(ctx, item); err != nil {
		return Item{}, err
	}
	total, err := s.Repo.SumReadyByProject(ctx, projectID)
	if err != nil {
		return Item{}, err
	}
	if total+size > maxProject {
		s.fail(ctx, item)
		_ = s.S3.Delete(ctx, item.S3StagingKey)
		return Item{}, ErrProjectFull
	}
	finalKey := fmt.Sprintf("resources/%s/%s/%s", projectID, resourceID, item.StorageName)
	if err := s.S3.Copy(ctx, item.S3StagingKey, finalKey, item.ContentType); err != nil {
		s.fail(ctx, item)
		return Item{}, err
	}
	body, actualSize, err := s.S3.Get(ctx, finalKey)
	if err != nil {
		s.fail(ctx, item)
		return Item{}, err
	}
	defer body.Close()
	if actualSize != size {
		s.fail(ctx, item)
		return Item{}, ErrInvalidFile
	}
	item.S3Key = finalKey
	meta := ResourceMeta{
		ID: item.ResourceID, DisplayName: item.DisplayName, StorageName: item.StorageName,
		ContentType: item.ContentType, S3Key: item.S3Key, Size: item.SizeBytes, Kind: item.Kind, Purpose: item.Purpose,
	}
	if err := s.Sandbox.WriteResource(ctx, projectID, meta, body, size); err != nil {
		s.fail(ctx, item)
		return Item{}, err
	}
	item.Status = "ready"
	if err := s.Repo.Update(ctx, item); err != nil {
		return Item{}, err
	}
	_ = s.S3.Delete(ctx, item.S3StagingKey)
	return toItem(*item), nil
}

func (s *Service) List(ctx context.Context, projectID string) (ListResult, error) {
	items, err := s.Repo.ListReadyByProject(ctx, projectID)
	if err != nil {
		return ListResult{}, err
	}
	total, err := s.Repo.SumReadyByProject(ctx, projectID)
	if err != nil {
		return ListResult{}, err
	}
	out := make([]Item, 0, len(items))
	for _, item := range items {
		out = append(out, toItem(item))
	}
	return ListResult{Resources: out, TotalBytes: total}, nil
}

func (s *Service) Delete(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) error {
	item, err := s.Repo.GetByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if item.WorkspaceID != workspaceID || item.UserID != userID || item.ProjectPublicID == nil || *item.ProjectPublicID != projectID {
		return ErrNotFound
	}
	if err := s.Sandbox.DeleteResource(ctx, projectID, resourceID); err != nil {
		return err
	}
	if item.S3Key != "" {
		_ = s.S3.Delete(ctx, item.S3Key)
	}
	if item.S3StagingKey != "" {
		_ = s.S3.Delete(ctx, item.S3StagingKey)
	}
	return s.Repo.SoftDelete(ctx, resourceID)
}

func (s *Service) DeleteStaging(ctx context.Context, workspaceID, userID int64, resourceID string) error {
	item, err := s.Repo.GetByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	if item.WorkspaceID != workspaceID || item.UserID != userID || item.ProjectPublicID != nil {
		return ErrNotFound
	}
	_ = s.S3.Delete(ctx, item.S3StagingKey)
	return s.Repo.SoftDelete(ctx, resourceID)
}

func (s *Service) fail(ctx context.Context, item *model.ProjectResource) {
	item.Status = "failed"
	_ = s.Repo.Update(ctx, item)
}

func toItem(item model.ProjectResource) Item {
	return Item{
		ID: item.ResourceID, Name: item.OriginalName, DisplayName: item.DisplayName,
		StorageName: item.StorageName, Path: ".resources/" + item.StorageName,
		ContentType: item.ContentType, Size: item.SizeBytes, Kind: item.Kind, S3Key: item.S3Key, CreatedAt: item.CreatedAt,
	}
}

func uniqueName(original string, existing []model.ProjectResource, selfID string) string {
	used := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		if item.ResourceID != selfID {
			used[item.DisplayName] = struct{}{}
		}
	}
	if _, ok := used[original]; !ok {
		return original
	}
	ext := filepath.Ext(original)
	stem := strings.TrimSuffix(original, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, ok := used[candidate]; !ok {
			return candidate
		}
	}
}

func normalizePurpose(purpose string) (string, bool) {
	switch purpose {
	case PurposeDefault, PurposeCloneImage, PurposeCloneUnitTest, PurposeCloneDocs:
		return purpose, true
	default:
		return "", false
	}
}

func purposeLimits(purpose string) (int64, int64) {
	switch purpose {
	case PurposeCloneImage:
		return 10 << 20, MaxCloneProjectBytes
	case PurposeCloneUnitTest, PurposeCloneDocs:
		return MaxCloneFileBytes, MaxCloneProjectBytes
	default:
		return MaxFileBytes, MaxProjectBytes
	}
}

func validateForPurpose(name, ext, rawType, purpose string) (string, string, error) {
	switch purpose {
	case PurposeCloneImage:
		contentType, kind, err := validateFile(name, ext, rawType)
		if err != nil || kind != "image" {
			return "", "", ErrInvalidFile
		}
		return contentType, kind, nil
	case PurposeCloneUnitTest:
		if ext != ".zip" {
			return "", "", ErrInvalidFile
		}
		contentType := normalizeContentType(rawType)
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = "application/zip"
		}
		if !zipMIME[contentType] {
			return "", "", ErrInvalidFile
		}
		return contentType, "document", nil
	case PurposeCloneDocs:
		if strings.TrimSpace(name) == "" {
			return "", "", ErrInvalidFile
		}
		contentType := normalizeContentType(rawType)
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = mimeByExt[ext]
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		return contentType, "document", nil
	default:
		return validateFile(name, ext, rawType)
	}
}

func normalizeContentType(rawType string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(rawType, ";")[0]))
}

var zipMIME = map[string]bool{
	"application/zip":              true,
	"application/x-zip-compressed": true,
	"application/x-zip":            true,
	"multipart/x-zip":              true,
}

func validateFile(name, ext, rawType string) (string, string, error) {
	ext = strings.ToLower(ext)
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(rawType, ";")[0]))
	if contentType == "" {
		contentType = mimeByExt[ext]
	}
	if !allowedExt[ext] {
		return "", "", ErrInvalidFile
	}
	if contentType == "" || contentType == "application/octet-stream" || !allowedMIME[contentType] {
		contentType = mimeByExt[ext]
	}
	if contentType == "" || !allowedMIME[contentType] {
		return "", "", ErrInvalidFile
	}
	return contentType, kindByExt(ext), nil
}

var allowedExt = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
	".txt": true, ".md": true, ".markdown": true, ".html": true, ".htm": true, ".css": true,
	".js": true, ".mjs": true, ".ts": true, ".tsx": true, ".jsx": true, ".json": true,
	".yaml": true, ".yml": true, ".xml": true, ".csv": true,
	".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true,
}

var mimeByExt = map[string]string{
	".pdf": "application/pdf", ".doc": "application/msword",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xls":  "application/vnd.ms-excel",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".ppt":  "application/vnd.ms-powerpoint",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
	".txt":  "text/plain", ".md": "text/markdown", ".markdown": "text/markdown",
	".html": "text/html", ".htm": "text/html", ".css": "text/css",
	".js": "text/javascript", ".mjs": "text/javascript", ".ts": "text/plain", ".tsx": "text/plain", ".jsx": "text/plain",
	".json": "application/json", ".yaml": "text/yaml", ".yml": "text/yaml", ".xml": "application/xml", ".csv": "text/csv",
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif",
}

var allowedMIME = map[string]bool{
	"application/pdf": true, "application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.ms-excel": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         true,
	"application/vnd.ms-powerpoint":                                             true,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": true,
	"text/plain": true, "text/markdown": true, "text/html": true, "text/css": true,
	"text/javascript": true, "application/javascript": true, "application/json": true,
	"text/yaml": true, "application/xml": true, "text/xml": true, "text/csv": true,
	"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true,
}

func kindByExt(ext string) string {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return "image"
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return "document"
	default:
		return "text"
	}
}
