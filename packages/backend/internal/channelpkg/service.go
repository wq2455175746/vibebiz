package channelpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/dao"
	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/model"
)

var (
	ErrInvalidChannel    = errors.New("invalid channel")
	ErrInvalidSourceType = errors.New("invalid source type")
	ErrUnsupported       = errors.New("channel is not supported")
	ErrPackageTooLarge   = errors.New("channel package is too large")
	ErrBusy              = errors.New("channel package builder is busy")
)

const (
	maxHTML5SourceBytes int64 = 200 << 20
	maxHTML5SourceFiles       = 20000
)

type Repository interface {
	Create(ctx context.Context, item *daomodel.ProjectChannelPackage) error
	Update(ctx context.Context, item *daomodel.ProjectChannelPackage) error
	GetByPublicID(ctx context.Context, publicID string) (*daomodel.ProjectChannelPackage, error)
	ListByProject(ctx context.Context, projectPublicID string) ([]daomodel.ProjectChannelPackage, error)
	Delete(ctx context.Context, publicID string) error
}

type ServiceConfig struct {
	Repo          Repository
	WorkspacesDir string
	PackagesDir   string
	OpenResource  ResourceOpener
	Adapters      map[string]core.Adapter
	MaxBytes      int64
	Timeout       time.Duration
	Concurrency   int
	Now           func() time.Time
	NewID         func() string
}

type GenerateInput struct {
	Channel    string
	SourceType string
	ResourceID string
}

type Service struct {
	cfg ServiceConfig
	sem chan struct{}
}

func NewService(cfg ServiceConfig) *Service {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 1 << 30
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 90 * time.Second
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 2
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.NewID == nil {
		cfg.NewID = func() string { return uuid.NewString() }
	}
	return &Service{cfg: cfg, sem: make(chan struct{}, cfg.Concurrency)}
}

func (s *Service) Generate(ctx context.Context, project *model.Project, input GenerateInput) (*daomodel.ProjectChannelPackage, error) {
	adapter, ok := s.cfg.Adapters[input.Channel]
	if !ok {
		return nil, ErrInvalidChannel
	}
	if input.SourceType != core.SourceHTML5 && input.SourceType != core.SourceEngineArtifact {
		return nil, ErrInvalidSourceType
	}
	if project == nil {
		return nil, ErrInvalidSource
	}
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	default:
		return nil, ErrBusy
	}

	now := s.now()
	item := &daomodel.ProjectChannelPackage{
		PublicID:        s.cfg.NewID(),
		ProjectPublicID: project.ID,
		WorkspaceID:     project.WorkspaceID,
		SourceType:      input.SourceType,
		Channel:         input.Channel,
		Status:          daomodel.ChannelPackageStatusBuilding,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if input.SourceType == core.SourceEngineArtifact {
		resourceID := input.ResourceID
		item.SourceResourceID = &resourceID
	}
	if err := s.cfg.Repo.Create(ctx, item); err != nil {
		return nil, err
	}

	buildCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	if err := os.MkdirAll(s.cfg.PackagesDir, 0o755); err != nil {
		return nil, s.fail(ctx, item, err)
	}

	var (
		info core.SourceInfo
		err  error
	)
	switch input.SourceType {
	case core.SourceHTML5:
		var source *WorkspaceSource
		source, err = NewWorkspaceSource(s.cfg.WorkspacesDir, project.ID)
		if err == nil {
			info, err = source.Materialize(buildCtx)
		}
	case core.SourceEngineArtifact:
		source := NewResourceSource(project.ID, input.ResourceID, s.cfg.OpenResource, s.cfg.PackagesDir)
		info, err = source.Materialize(buildCtx)
		if err == nil && info.Dir != "" {
			defer func() { _ = os.RemoveAll(info.Dir) }()
		}
	}
	if err != nil {
		return nil, s.fail(ctx, item, err)
	}

	if input.SourceType == core.SourceHTML5 && (info.TotalBytes > maxHTML5SourceBytes || info.FileCount > maxHTML5SourceFiles) {
		return nil, s.fail(ctx, item, fmt.Errorf("%w: HTML5 source exceeds limits", ErrPackageTooLarge))
	}

	if info.Report.Level == core.LevelC {
		return nil, s.fail(ctx, item, ErrUnsupported)
	}
	report := adapter.Validate(info)
	if report.Level == core.LevelC || len(report.Blockers) > 0 {
		return nil, s.fail(ctx, item, ErrUnsupported)
	}
	info.Report = report

	tmp, err := os.CreateTemp(s.cfg.PackagesDir, ".tmp-"+item.PublicID+"-*.zip")
	if err != nil {
		return nil, s.fail(ctx, item, err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	manifest, err := adapter.Build(buildCtx, info, tmp)
	closeErr := tmp.Close()
	if err != nil {
		return nil, s.fail(ctx, item, err)
	}
	if closeErr != nil {
		return nil, s.fail(ctx, item, closeErr)
	}
	if err := buildCtx.Err(); err != nil {
		return nil, s.fail(ctx, item, err)
	}
	stat, err := os.Stat(tmpName)
	if err != nil {
		return nil, s.fail(ctx, item, err)
	}
	if stat.Size() > s.cfg.MaxBytes {
		return nil, s.fail(ctx, item, ErrPackageTooLarge)
	}

	targetDir := filepath.Join(s.cfg.PackagesDir, project.ID, input.Channel)
	target := filepath.Join(targetDir, item.PublicID+".zip")
	relative, err := filepath.Rel(s.cfg.PackagesDir, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return nil, s.fail(ctx, item, ErrInvalidSource)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, s.fail(ctx, item, err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return nil, s.fail(ctx, item, err)
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		_ = os.Remove(target)
		return nil, s.fail(ctx, item, err)
	}
	item.Status = daomodel.ChannelPackageStatusReady
	item.Version = manifest.SourceVersion
	if item.Version == "" {
		item.Version = info.Version
	}
	item.FilePath = filepath.ToSlash(relative)
	item.FileSizeBytes = stat.Size()
	item.Manifest = data
	item.ErrorMessage = ""
	item.UpdatedAt = s.now()
	if err := s.cfg.Repo.Update(context.WithoutCancel(ctx), item); err != nil {
		_ = os.Remove(target)
		return nil, s.fail(ctx, item, err)
	}
	return item, nil
}

func (s *Service) fail(ctx context.Context, item *daomodel.ProjectChannelPackage, cause error) error {
	item.Status = daomodel.ChannelPackageStatusFailed
	item.ErrorMessage = truncateMessage(cause.Error())
	item.FilePath = ""
	item.FileSizeBytes = 0
	item.Manifest = nil
	item.UpdatedAt = s.now()
	_ = s.cfg.Repo.Update(context.WithoutCancel(ctx), item)
	return cause
}

func (s *Service) PackagesDir() string { return s.cfg.PackagesDir }

func (s *Service) Get(ctx context.Context, packageID string) (*daomodel.ProjectChannelPackage, error) {
	return s.cfg.Repo.GetByPublicID(ctx, packageID)
}

func (s *Service) List(ctx context.Context, projectID string) ([]daomodel.ProjectChannelPackage, error) {
	return s.cfg.Repo.ListByProject(ctx, projectID)
}

func (s *Service) Delete(ctx context.Context, projectID, packageID string) error {
	item, err := s.cfg.Repo.GetByPublicID(ctx, packageID)
	if err != nil {
		return err
	}
	if item.ProjectPublicID != projectID {
		return dao.ErrNotFound
	}
	if item.FilePath != "" {
		if !projectIDPattern.MatchString(projectID) {
			return ErrInvalidSource
		}
		relativePath := filepath.FromSlash(item.FilePath)
		if filepath.IsAbs(relativePath) || filepath.VolumeName(relativePath) != "" {
			return ErrInvalidSource
		}
		projectRoot := filepath.Join(s.cfg.PackagesDir, projectID)
		full := filepath.Join(s.cfg.PackagesDir, relativePath)
		inside, relErr := filepath.Rel(projectRoot, full)
		if relErr != nil || inside == "." || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) || filepath.IsAbs(inside) {
			return ErrInvalidSource
		}
		if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("delete channel package file: %w", err)
		}
	}
	return s.cfg.Repo.Delete(ctx, packageID)
}

func (s *Service) now() time.Time {
	return s.cfg.Now().UTC()
}

func truncateMessage(message string) string {
	const max = 1024
	runes := []rune(message)
	if len(runes) <= max {
		return message
	}
	return string(runes[:max])
}
