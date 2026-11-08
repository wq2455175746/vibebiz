package evolution

import (
	"context"
	"errors"

	"vibebiz/internal/dao/model"
)

const (
	ProjectID       = "vibebiz-evolve"
	OriginVersion   = "v1.2026.11"
	OfficialUserID  = "official"
	OfficialName    = "Vibebiz 官方"
	OfficialVersion = "v-official-1"
	AgentName       = "集思进化助手"
	AdminName       = "运营管理员"
	MaxContentRunes = 4000
)

const (
	VersionDraft     = "draft"
	VersionPreview   = "preview"
	VersionAccepted  = "accepted"
	VersionSubmitted = "submitted"
	VersionReleased  = "released"
)

const (
	ProposalQueued    = "queued"
	ProposalReviewing = "reviewing"
	ProposalAccepted  = "accepted"
)

const (
	TaskQueued   = "queued"
	TaskCoding   = "coding"
	TaskCI       = "ci"
	TaskReleased = "released"
	TaskFailed   = "failed"
)

var (
	ErrNotFound         = errors.New("evolution record not found")
	ErrUnknownUser      = errors.New("evolution viewer is required")
	ErrEmptyContent     = errors.New("evolution content is empty")
	ErrContentTooLong   = errors.New("evolution content is too long")
	ErrOfficialReadonly = errors.New("official version is readonly")
	ErrTaskExists       = errors.New("evolution task already exists")
	ErrInvalidStatus    = errors.New("invalid evolution status")
)

type Viewer struct {
	ID   string
	Name string
}

type Store interface {
	ListVersions(ctx context.Context) ([]model.EvolutionVersion, error)
	GetVersion(ctx context.Context, publicID string) (*model.EvolutionVersion, error)
	CreateVersion(ctx context.Context, version *model.EvolutionVersion) error
	UpdateVersionStatus(ctx context.Context, publicID, status string) error

	ListProposals(ctx context.Context, userID string) ([]model.EvolutionProposal, error)
	ListAllProposals(ctx context.Context) ([]model.EvolutionProposal, error)
	CreateProposal(ctx context.Context, proposal *model.EvolutionProposal) error
	AcceptProposalsByVersion(ctx context.Context, versionID string) error

	ListMessages(ctx context.Context, userID string) ([]model.EvolutionMessage, error)
	CreateMessage(ctx context.Context, message *model.EvolutionMessage) error

	ListTasks(ctx context.Context) ([]model.EvolutionTask, error)
	GetTask(ctx context.Context, publicID string) (*model.EvolutionTask, error)
	GetTaskByVersion(ctx context.Context, versionID string) (*model.EvolutionTask, error)
	CreateTask(ctx context.Context, task *model.EvolutionTask) error
	UpdateTaskStatus(ctx context.Context, publicID, status string) error
}

func ValidTaskStatus(status string) bool {
	switch status {
	case TaskQueued, TaskCoding, TaskCI, TaskReleased, TaskFailed:
		return true
	default:
		return false
	}
}
