package evolution

import (
	"context"
	"embed"
	"fmt"
	"sync"
	"time"

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/model"
)

//go:embed base/index.html
var baseFiles embed.FS

const ProjectPrefix = "evolve-"

type ProjectStore interface {
	Get(ctx context.Context, id string) (*model.Project, error)
	Create(ctx context.Context, project *model.Project) error
	Delete(ctx context.Context, id string) error
}

type ProjectInitializer interface {
	InitProject(ctx context.Context, id string) (int64, error)
}

type FileWriter interface {
	WriteFile(ctx context.Context, id, path, content, baseVersion string) (*client.WriteResult, error)
}

type MetaStore interface {
	Set(ctx context.Context, projectID string, meta auth.ProjectMeta) error
	Delete(ctx context.Context, projectID string) error
}

type WorkspaceProvisioner struct {
	Projects    ProjectStore
	Initializer ProjectInitializer
	Files       FileWriter
	Metas       MetaStore
	Now         func() time.Time

	mu sync.Mutex
}

func EvolutionProjectID(userID string) string {
	return ProjectPrefix + userID
}

func (p *WorkspaceProvisioner) Ensure(ctx context.Context, viewer Viewer) (string, error) {
	if viewer.ID == "" {
		return "", ErrUnknownUser
	}
	if viewer.WorkspaceID == 0 {
		return "", ErrNoWorkspace
	}
	projectID := EvolutionProjectID(viewer.ID)

	p.mu.Lock()
	defer p.mu.Unlock()

	if project, err := p.Projects.Get(ctx, projectID); err == nil && project != nil {
		return projectID, nil
	}

	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	project := &model.Project{
		ID:          projectID,
		Name:        "集思进化计划",
		Prompt:      "集思进化计划个人工作区",
		Mode:        model.ProjectModeStandard,
		State:       model.ProjectStateReady,
		WorkspaceID: viewer.WorkspaceID,
		OwnerUserID: viewer.InternalID,
		IsHidden:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	sandboxID, err := p.Initializer.InitProject(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("init evolution workspace: %w", err)
	}
	project.SandboxID = &sandboxID

	if err := p.Projects.Create(ctx, project); err != nil {
		if existing, getErr := p.Projects.Get(ctx, projectID); getErr == nil && existing != nil {
			return projectID, nil
		}
		return "", fmt.Errorf("create evolution project: %w", err)
	}
	if err := p.Metas.Set(ctx, projectID, auth.ProjectMeta{
		WorkspaceID: viewer.WorkspaceID,
		OwnerUserID: viewer.InternalID,
	}); err != nil {
		_ = p.Projects.Delete(ctx, projectID)
		return "", fmt.Errorf("set evolution project meta: %w", err)
	}
	if err := p.writeBase(ctx, projectID); err != nil {
		_ = p.Metas.Delete(ctx, projectID)
		_ = p.Projects.Delete(ctx, projectID)
		return "", err
	}
	return projectID, nil
}

func (p *WorkspaceProvisioner) writeBase(ctx context.Context, projectID string) error {
	content, err := baseFiles.ReadFile("base/index.html")
	if err != nil {
		return fmt.Errorf("read evolution base: %w", err)
	}
	if _, err := p.Files.WriteFile(ctx, projectID, "index.html", string(content), ""); err != nil {
		return fmt.Errorf("write evolution base: %w", err)
	}
	return nil
}
