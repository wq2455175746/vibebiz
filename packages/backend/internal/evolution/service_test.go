package evolution

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"vibebiz/internal/dao/model"
)

func testService() (*Service, *MemoryStore) {
	store := NewMemoryStore()
	service := NewService(store)
	fixed := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	service.Now = func() time.Time { return fixed }
	seq := 0
	service.NewID = func(prefix string) string {
		seq++
		return fmt.Sprintf("%s-%d", prefix, seq)
	}
	return service, store
}

func TestSubmitBugCreatesPreviewVersion(t *testing.T) {
	service, store := testService()
	ctx := context.Background()

	proposal, version, err := service.SubmitProposal(ctx, Viewer{ID: "7", Name: "tester"}, "删除按钮点击没有反应")
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Kind != KindBug || proposal.Status != ProposalQueued {
		t.Fatalf("proposal = %+v", proposal)
	}
	if version == nil || version.Status != VersionPreview || version.Scenario != "bugfix" {
		t.Fatalf("version = %+v", version)
	}
	if proposal.VersionPublicID != version.PublicID {
		t.Fatalf("proposal version = %q, version id = %q", proposal.VersionPublicID, version.PublicID)
	}
	messages, err := store.ListMessages(ctx, "7")
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].Role != "agent" {
		t.Fatalf("messages = %+v", messages)
	}
}

func TestSubmitFeaturePushesBack(t *testing.T) {
	service, _ := testService()
	proposal, version, err := service.SubmitProposal(context.Background(), Viewer{ID: "7", Name: "tester"}, "希望支持多人同时编辑")
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Kind != KindFeature || proposal.Status != ProposalReviewing {
		t.Fatalf("proposal = %+v", proposal)
	}
	if version != nil {
		t.Fatalf("version = %+v, want nil", version)
	}
	if proposal.AgentReply == "" {
		t.Fatal("expected pushback reply")
	}
}

func TestConfirmVersionAcceptsProposals(t *testing.T) {
	service, store := testService()
	ctx := context.Background()
	_, version, err := service.SubmitProposal(ctx, Viewer{ID: "7", Name: "tester"}, "首页卡片布局再紧凑一点")
	if err != nil {
		t.Fatal(err)
	}
	if version == nil {
		t.Fatal("expected preview version")
	}

	confirmed, err := service.ConfirmVersion(ctx, version.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != VersionAccepted {
		t.Fatalf("status = %q", confirmed.Status)
	}
	proposals, err := store.ListProposals(ctx, "7")
	if err != nil {
		t.Fatal(err)
	}
	for _, proposal := range proposals {
		if proposal.Status != ProposalAccepted {
			t.Fatalf("proposal status = %q", proposal.Status)
		}
	}
	messages, err := store.ListMessages(ctx, "7")
	if err != nil {
		t.Fatal(err)
	}
	last := messages[len(messages)-1]
	if !strings.Contains(last.Content, "已收录你的意见版本") {
		t.Fatalf("last message = %q", last.Content)
	}
}

func TestSubmitTaskDedupesAndMarksVersion(t *testing.T) {
	service, store := testService()
	ctx := context.Background()
	_, version, err := service.SubmitProposal(ctx, Viewer{ID: "7", Name: "tester"}, "首页卡片布局再紧凑一点")
	if err != nil || version == nil {
		t.Fatalf("submit proposal: %v", err)
	}

	task, err := service.SubmitTask(ctx, version.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != TaskQueued || task.VersionPublicID != version.PublicID {
		t.Fatalf("task = %+v", task)
	}
	if _, err := service.SubmitTask(ctx, version.PublicID); !errors.Is(err, ErrTaskExists) {
		t.Fatalf("second submit error = %v", err)
	}
	stored, err := store.GetVersion(ctx, version.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != VersionSubmitted {
		t.Fatalf("version status = %q", stored.Status)
	}
}

func TestUpdateTaskStatusReleasesVersion(t *testing.T) {
	service, store := testService()
	ctx := context.Background()
	_, version, err := service.SubmitProposal(ctx, Viewer{ID: "7", Name: "tester"}, "首页卡片布局再紧凑一点")
	if err != nil || version == nil {
		t.Fatalf("submit proposal: %v", err)
	}
	task, err := service.SubmitTask(ctx, version.PublicID)
	if err != nil {
		t.Fatal(err)
	}

	updated, err := service.UpdateTaskStatus(ctx, task.PublicID, TaskCoding)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != TaskCoding {
		t.Fatalf("status = %q", updated.Status)
	}
	if _, err := service.UpdateTaskStatus(ctx, task.PublicID, "unknown"); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("invalid status error = %v", err)
	}
	if _, err := service.UpdateTaskStatus(ctx, task.PublicID, TaskReleased); err != nil {
		t.Fatal(err)
	}
	stored, err := store.GetVersion(ctx, version.PublicID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != VersionReleased {
		t.Fatalf("version status = %q", stored.Status)
	}
}

func TestOfficialVersionIsReadonly(t *testing.T) {
	service, store := testService()
	ctx := context.Background()
	official := &model.EvolutionVersion{PublicID: OfficialVersion, UserID: OfficialUserID, UserName: OfficialName}
	if err := store.CreateVersion(ctx, official); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmVersion(ctx, OfficialVersion); !errors.Is(err, ErrOfficialReadonly) {
		t.Fatalf("confirm error = %v", err)
	}
	if _, err := service.SubmitTask(ctx, OfficialVersion); !errors.Is(err, ErrOfficialReadonly) {
		t.Fatalf("submit task error = %v", err)
	}
}

func TestAdminReplyAndEmptyContent(t *testing.T) {
	service, store := testService()
	ctx := context.Background()
	_, version, err := service.SubmitProposal(ctx, Viewer{ID: "7", Name: "tester"}, "首页卡片布局再紧凑一点")
	if err != nil || version == nil {
		t.Fatalf("submit proposal: %v", err)
	}

	message, err := service.AdminReply(ctx, version.PublicID, "这个方向可以，补充一下验收标准。")
	if err != nil {
		t.Fatal(err)
	}
	if message.Role != "admin" || message.UserName != AdminName {
		t.Fatalf("message = %+v", message)
	}
	messages, err := store.ListMessages(ctx, "7")
	if err != nil {
		t.Fatal(err)
	}
	if messages[len(messages)-1].Role != "admin" {
		t.Fatalf("messages = %+v", messages)
	}
	if _, _, err := service.SubmitProposal(ctx, Viewer{ID: "7", Name: "tester"}, "   "); !errors.Is(err, ErrEmptyContent) {
		t.Fatalf("empty content error = %v", err)
	}
}

func TestEnsureDraftIsIdempotent(t *testing.T) {
	service, store := testService()
	ctx := context.Background()

	first, err := service.EnsureDraft(ctx, Viewer{ID: "7", Name: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != VersionDraft || first.Label != "v7.1" {
		t.Fatalf("draft = %+v", first)
	}
	second, err := service.EnsureDraft(ctx, Viewer{ID: "7", Name: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if second.PublicID != first.PublicID {
		t.Fatalf("second draft = %q, first = %q", second.PublicID, first.PublicID)
	}
	versions, err := store.ListVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("versions = %d", len(versions))
	}
}
