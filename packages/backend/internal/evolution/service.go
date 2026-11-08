package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vibebiz/internal/dao/model"
)

type Snapshot struct {
	Versions  []model.EvolutionVersion
	Proposals []model.EvolutionProposal
	Tasks     []model.EvolutionTask
}

type Service struct {
	Store Store
	Now   func() time.Time
	NewID func(prefix string) string
}

func NewService(store Store) *Service {
	return &Service{
		Store: store,
		Now:   time.Now,
		NewID: func(prefix string) string { return prefix + "-" + uuid.NewString() },
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) newID(prefix string) string {
	if s.NewID != nil {
		return s.NewID(prefix)
	}
	return prefix + "-" + uuid.NewString()
}

func (s *Service) ListVersions(ctx context.Context) ([]model.EvolutionVersion, string, error) {
	versions, err := s.Store.ListVersions(ctx)
	if err != nil {
		return nil, "", err
	}
	if versions == nil {
		versions = []model.EvolutionVersion{}
	}
	return versions, OriginVersion, nil
}

func (s *Service) Conversation(ctx context.Context, userID string) ([]model.EvolutionMessage, []model.EvolutionProposal, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil, ErrUnknownUser
	}
	messages, err := s.Store.ListMessages(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	proposals, err := s.Store.ListProposals(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if messages == nil {
		messages = []model.EvolutionMessage{}
	}
	if proposals == nil {
		proposals = []model.EvolutionProposal{}
	}
	return messages, proposals, nil
}

func (s *Service) Snapshot(ctx context.Context) (*Snapshot, error) {
	versions, err := s.Store.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	proposals, err := s.Store.ListAllProposals(ctx)
	if err != nil {
		return nil, err
	}
	tasks, err := s.Store.ListTasks(ctx)
	if err != nil {
		return nil, err
	}
	if versions == nil {
		versions = []model.EvolutionVersion{}
	}
	if proposals == nil {
		proposals = []model.EvolutionProposal{}
	}
	if tasks == nil {
		tasks = []model.EvolutionTask{}
	}
	return &Snapshot{Versions: versions, Proposals: proposals, Tasks: tasks}, nil
}

func (s *Service) EnsureDraft(ctx context.Context, viewer Viewer) (*model.EvolutionVersion, error) {
	viewer = normalizeViewer(viewer)
	if viewer.ID == "" {
		return nil, ErrUnknownUser
	}
	versions, err := s.Store.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range versions {
		if versions[i].UserID == viewer.ID {
			return &versions[i], nil
		}
	}
	now := s.now()
	version := &model.EvolutionVersion{
		PublicID:      s.newID("v"),
		UserID:        viewer.ID,
		UserName:      viewer.Name,
		Label:         nextVersionLabel(viewer.ID, 0),
		Status:        VersionDraft,
		Scenario:      "baseline",
		Summary:       "从官方版本创建的个人副本。",
		OriginVersion: OriginVersion,
		ProposalIDs:   proposalIDs([]string{}),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.Store.CreateVersion(ctx, version); err != nil {
		return nil, err
	}
	return version, nil
}

func (s *Service) SubmitProposal(ctx context.Context, viewer Viewer, content string) (*model.EvolutionProposal, *model.EvolutionVersion, error) {
	viewer = normalizeViewer(viewer)
	if viewer.ID == "" {
		return nil, nil, ErrUnknownUser
	}
	content, err := normalizeContent(content)
	if err != nil {
		return nil, nil, err
	}

	versions, err := s.Store.ListVersions(ctx)
	if err != nil {
		return nil, nil, err
	}
	count := 0
	for _, version := range versions {
		if version.UserID == viewer.ID {
			count++
		}
	}

	decision := Decide(content)
	now := s.now()
	proposalID := s.newID("p")
	versionID := ""
	var version *model.EvolutionVersion
	if decision.Action != ActionPushback {
		versionID = s.newID("v")
		version = &model.EvolutionVersion{
			PublicID:      versionID,
			UserID:        viewer.ID,
			UserName:      viewer.Name,
			Label:         nextVersionLabel(viewer.ID, count),
			Status:        VersionPreview,
			Scenario:      decision.Scenario,
			Summary:       decision.Summary,
			OriginVersion: OriginVersion,
			ProposalIDs:   proposalIDs([]string{proposalID}),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := s.Store.CreateVersion(ctx, version); err != nil {
			return nil, nil, err
		}
	}

	status := ProposalQueued
	if decision.Action == ActionPushback {
		status = ProposalReviewing
	}
	proposal := &model.EvolutionProposal{
		PublicID:        proposalID,
		UserID:          viewer.ID,
		UserName:        viewer.Name,
		Kind:            decision.Kind,
		Content:         content,
		Status:          status,
		AgentReply:      decision.Reply,
		VersionPublicID: versionID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.Store.CreateProposal(ctx, proposal); err != nil {
		return nil, nil, err
	}
	if err := s.appendMessages(ctx, viewer.ID, viewer.Name,
		messageDraft{Role: "user", Content: content},
		messageDraft{Role: "agent", Content: decision.Reply},
	); err != nil {
		return nil, nil, err
	}
	return proposal, version, nil
}

func (s *Service) SubmitChat(ctx context.Context, viewer Viewer, content string) ([]model.EvolutionMessage, error) {
	viewer = normalizeViewer(viewer)
	if viewer.ID == "" {
		return nil, ErrUnknownUser
	}
	content, err := normalizeContent(content)
	if err != nil {
		return nil, err
	}
	decision := Decide(content)
	reply := decision.Reply + "\n\n（对话只写入聊天记录；如需我直接生成效果，请切换到「提议」提交。）"
	now := s.now()
	messages := []model.EvolutionMessage{
		{
			PublicID:  s.newID("c"),
			UserID:    viewer.ID,
			UserName:  viewer.Name,
			Role:      "user",
			Content:   content,
			CreatedAt: now,
		},
		{
			PublicID:  s.newID("c"),
			UserID:    viewer.ID,
			UserName:  AgentName,
			Role:      "agent",
			Content:   reply,
			CreatedAt: now,
		},
	}
	for i := range messages {
		if err := s.Store.CreateMessage(ctx, &messages[i]); err != nil {
			return nil, err
		}
	}
	return messages, nil
}

func (s *Service) ConfirmVersion(ctx context.Context, versionID string) (*model.EvolutionVersion, error) {
	version, err := s.Store.GetVersion(ctx, strings.TrimSpace(versionID))
	if err != nil {
		return nil, err
	}
	if version.UserID == OfficialUserID {
		return nil, ErrOfficialReadonly
	}
	now := s.now()
	if err := s.Store.UpdateVersionStatus(ctx, version.PublicID, VersionAccepted); err != nil {
		return nil, err
	}
	if err := s.Store.AcceptProposalsByVersion(ctx, version.PublicID); err != nil {
		return nil, err
	}
	if err := s.appendMessages(ctx, version.UserID, version.UserName,
		messageDraft{Role: "agent", Content: "已收录你的意见版本，评估通过后随下一版本发布。"},
	); err != nil {
		return nil, err
	}
	version.Status = VersionAccepted
	version.UpdatedAt = now
	return version, nil
}

func (s *Service) AdminReply(ctx context.Context, versionID, content string) (*model.EvolutionMessage, error) {
	version, err := s.Store.GetVersion(ctx, strings.TrimSpace(versionID))
	if err != nil {
		return nil, err
	}
	content, err = normalizeContent(content)
	if err != nil {
		return nil, err
	}
	message := &model.EvolutionMessage{
		PublicID:  s.newID("c"),
		UserID:    version.UserID,
		UserName:  AdminName,
		Role:      "admin",
		Content:   content,
		CreatedAt: s.now(),
	}
	if err := s.Store.CreateMessage(ctx, message); err != nil {
		return nil, err
	}
	return message, nil
}

func (s *Service) SubmitTask(ctx context.Context, versionID string) (*model.EvolutionTask, error) {
	version, err := s.Store.GetVersion(ctx, strings.TrimSpace(versionID))
	if err != nil {
		return nil, err
	}
	if version.UserID == OfficialUserID {
		return nil, ErrOfficialReadonly
	}
	if _, err := s.Store.GetTaskByVersion(ctx, version.PublicID); err == nil {
		return nil, ErrTaskExists
	} else if err != ErrNotFound {
		return nil, err
	}

	messages, err := s.Store.ListMessages(ctx, version.UserID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	task := &model.EvolutionTask{
		PublicID:        s.newID("t"),
		VersionPublicID: version.PublicID,
		UserID:          version.UserID,
		UserName:        version.UserName,
		VersionLabel:    version.Label,
		OriginVersion:   version.OriginVersion,
		ChatSummary:     buildSummary(version, messages),
		Status:          TaskQueued,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.Store.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	if err := s.Store.UpdateVersionStatus(ctx, version.PublicID, VersionSubmitted); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) UpdateTaskStatus(ctx context.Context, taskID, status string) (*model.EvolutionTask, error) {
	status = strings.TrimSpace(status)
	if !ValidTaskStatus(status) {
		return nil, ErrInvalidStatus
	}
	task, err := s.Store.GetTask(ctx, strings.TrimSpace(taskID))
	if err != nil {
		return nil, err
	}
	if err := s.Store.UpdateTaskStatus(ctx, task.PublicID, status); err != nil {
		return nil, err
	}
	task.Status = status
	task.UpdatedAt = s.now()
	if status == TaskReleased {
		version, err := s.Store.GetVersion(ctx, task.VersionPublicID)
		if err == nil && version.UserID != OfficialUserID {
			if err := s.Store.UpdateVersionStatus(ctx, version.PublicID, VersionReleased); err != nil {
				return nil, err
			}
		}
	}
	return task, nil
}

type messageDraft struct {
	Role    string
	Content string
}

func (s *Service) appendMessages(ctx context.Context, userID, userName string, drafts ...messageDraft) error {
	now := s.now()
	for _, draft := range drafts {
		message := &model.EvolutionMessage{
			PublicID:  s.newID("c"),
			UserID:    userID,
			UserName:  userName,
			Role:      draft.Role,
			Content:   draft.Content,
			CreatedAt: now,
		}
		if draft.Role == "agent" || draft.Role == "admin" {
			message.UserName = AgentName
			if draft.Role == "admin" {
				message.UserName = AdminName
			}
		}
		if err := s.Store.CreateMessage(ctx, message); err != nil {
			return err
		}
	}
	return nil
}

func normalizeViewer(viewer Viewer) Viewer {
	viewer.ID = strings.TrimSpace(viewer.ID)
	viewer.Name = strings.TrimSpace(viewer.Name)
	if viewer.Name == "" {
		viewer.Name = viewer.ID
	}
	return viewer
}

func normalizeContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", ErrEmptyContent
	}
	if utf8.RuneCountInString(content) > MaxContentRunes {
		return "", ErrContentTooLong
	}
	return content, nil
}

func nextVersionLabel(userID string, existing int) string {
	return fmt.Sprintf("v%s.%d", userID, existing+1)
}

func proposalIDs(ids []string) model.JSON {
	raw, err := json.Marshal(ids)
	if err != nil {
		return model.JSON("[]")
	}
	return model.JSON(raw)
}

func buildSummary(version *model.EvolutionVersion, messages []model.EvolutionMessage) string {
	parts := make([]string, 0, 5)
	parts = append(parts, version.Summary)
	start := len(messages) - 4
	if start < 0 {
		start = 0
	}
	for _, message := range messages[start:] {
		parts = append(parts, message.Content)
	}
	return strings.TrimSpace(strings.Join(parts, " / "))
}
