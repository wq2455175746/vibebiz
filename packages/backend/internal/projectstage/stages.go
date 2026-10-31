package projectstage

import (
	"context"
	"errors"
	"time"

	"vibebiz/internal/model"
)

const (
	WorkspaceDocument    = "document"
	WorkspaceDevelopment = "development"
	WorkspaceTest        = "test"
	WorkspaceArtifact    = "artifact"
	WorkspacePlaceholder = "placeholder"
)

var (
	ErrStageNotFound = errors.New("project stage not found")
	ErrStageConflict = errors.New("project stage conflict")
)

type Definition struct {
	Key          string
	Seq          int
	Title        string
	RoleCode     string
	RoleName     string
	ArtifactPath string
	Workspace    string
}

var Definitions = []Definition{
	{Key: "requirements", Seq: 1, Title: "需求梳理", RoleCode: "emma", RoleName: "Emma · 产品经理", ArtifactPath: "docs/requirements.md", Workspace: WorkspaceDocument},
	{Key: "architecture", Seq: 2, Title: "概要设计", RoleCode: "bob", RoleName: "Bob · 系统架构师", ArtifactPath: "docs/architecture.md", Workspace: WorkspaceDocument},
	{Key: "detailed_design", Seq: 3, Title: "详细设计", RoleCode: "senior", RoleName: "高级工程师", ArtifactPath: "docs/detailed-design.md", Workspace: WorkspaceDocument},
	{Key: "test_plan", Seq: 4, Title: "测试用例", RoleCode: "qa", RoleName: "测试开发工程师", ArtifactPath: "docs/test-plan.md", Workspace: WorkspaceDocument},
	{Key: "development", Seq: 5, Title: "全栈开发", RoleCode: "alex", RoleName: "Alex · 全栈工程师", ArtifactPath: "docs/development.md", Workspace: WorkspaceDevelopment},
	{Key: "artifacts", Seq: 6, Title: "制品空间", Workspace: WorkspacePlaceholder},
	{Key: "delivery", Seq: 7, Title: "交付空间", Workspace: WorkspacePlaceholder},
}

var CloneDefinitions = []Definition{
	{Key: "base_design", Seq: 1, Title: "基础设计", RoleCode: "bob", RoleName: "Bob · 系统架构师（目标分析）", ArtifactPath: "docs/clone/base-design.md", Workspace: WorkspaceDocument},
	{Key: "architecture", Seq: 2, Title: "架构", RoleCode: "bob", RoleName: "Bob · 系统架构师（方案设计）", ArtifactPath: "docs/clone/architecture.md", Workspace: WorkspaceDocument},
	{Key: "test", Seq: 3, Title: "测试", RoleCode: "qa", RoleName: "QA · 测试开发工程师", ArtifactPath: "docs/clone/test-plan.md", Workspace: WorkspaceTest},
	{Key: "development", Seq: 4, Title: "全栈开发", RoleCode: "alex", RoleName: "Alex · 全栈工程师", ArtifactPath: "docs/clone/development.md", Workspace: WorkspaceDevelopment},
	{Key: "artifacts", Seq: 5, Title: "制品空间", Workspace: WorkspaceArtifact},
}

func DefinitionsForMode(mode string) []Definition {
	if mode == model.ProjectModeClone {
		return CloneDefinitions
	}
	return Definitions
}

func DefinitionFor(key string) (Definition, bool) {
	return DefinitionForMode(model.ProjectModeEnterprise, key)
}

func DefinitionForMode(mode, key string) (Definition, bool) {
	for _, definition := range DefinitionsForMode(mode) {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

func InitialStages(projectID string) []model.ProjectStage {
	return InitialStagesForMode(projectID, model.ProjectModeEnterprise)
}

func InitialStagesForMode(projectID, mode string) []model.ProjectStage {
	definitions := DefinitionsForMode(mode)
	stages := make([]model.ProjectStage, 0, len(definitions))
	for _, definition := range definitions {
		status := model.ProjectStageLocked
		switch {
		case definition.Workspace == WorkspacePlaceholder:
			status = model.ProjectStagePlaceholder
		case definition.Seq == 1:
			status = model.ProjectStageActive
		}
		stages = append(stages, model.ProjectStage{
			ProjectID: projectID, StageKey: definition.Key, Seq: definition.Seq,
			Status: status, ArtifactPath: definition.ArtifactPath,
		})
	}
	return stages
}

type Store interface {
	CreateInitial(ctx context.Context, projectID string) error
	CreateInitialForMode(ctx context.Context, projectID, mode string) error
	DeleteByProject(ctx context.Context, projectID string) error
	List(ctx context.Context, projectID string) ([]model.ProjectStage, error)
	Get(ctx context.Context, projectID, stageKey string) (*model.ProjectStage, error)
	Start(ctx context.Context, projectID, stageKey string, now time.Time) (bool, error)
	Complete(ctx context.Context, projectID, stageKey string, now time.Time) (string, error)
}
