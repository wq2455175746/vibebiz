package account

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"vibebiz/internal/auth"
	"vibebiz/internal/dao"
)

type fakePresigner struct {
	putKey       string
	putType      string
	putSize      int64
	uploadKey    string
	uploadType   string
	uploadSize   int64
	uploadBody   string
	uploadErr    error
	headSize     int64
	headType     string
	headErr      error
	presignGetOK bool
}

func (f *fakePresigner) PresignPut(_ context.Context, key, contentType string, size int64, _ time.Duration) (string, map[string]string, error) {
	f.putKey = key
	f.putType = contentType
	f.putSize = size
	return "https://s3.example/put/" + key, map[string]string{"Content-Type": contentType}, nil
}

func (f *fakePresigner) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	f.presignGetOK = true
	return "https://s3.example/get/" + key, nil
}

func (f *fakePresigner) Put(_ context.Context, key, contentType string, size int64, body io.Reader) error {
	if f.uploadErr != nil {
		return f.uploadErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.uploadKey = key
	f.uploadType = contentType
	f.uploadSize = size
	f.uploadBody = string(data)
	return nil
}

func (f *fakePresigner) Head(_ context.Context, _ string) (int64, string, error) {
	return f.headSize, f.headType, f.headErr
}

type fakeRepo struct {
	profile      *dao.UserProfile
	usersByEmail map[string]*dao.UserProfile
	memberships  []dao.WorkspaceMembership
	workspaces   map[int64]*dao.WorkspaceDetail
	members      map[int64][]dao.WorkspaceMember
	folders      map[int64][]dao.WorkspaceFolderRecord
	quota        *dao.UserQuotaRecord
	usernameUsed bool
	updatedUsed  int64
	storedKey    string
	added        int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		usersByEmail: make(map[string]*dao.UserProfile),
		workspaces:   make(map[int64]*dao.WorkspaceDetail),
		members:      make(map[int64][]dao.WorkspaceMember),
		folders:      make(map[int64][]dao.WorkspaceFolderRecord),
	}
}

func (f *fakeRepo) UserProfile(_ context.Context, _ int64) (*dao.UserProfile, error) {
	if f.profile == nil {
		return nil, dao.ErrNotFound
	}
	return f.profile, nil
}

func (f *fakeRepo) FindUserByEmail(_ context.Context, email string) (*dao.UserProfile, error) {
	profile, ok := f.usersByEmail[email]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return profile, nil
}

func (f *fakeRepo) UsernameExists(_ context.Context, _ string, _ int64) (bool, error) {
	return f.usernameUsed, nil
}

func (f *fakeRepo) UpdateUsername(_ context.Context, _ int64, username string) error {
	f.profile.Username = username
	return nil
}

func (f *fakeRepo) UpdateUserAvatar(_ context.Context, _ int64, key string) error {
	f.profile.AvatarS3Path = key
	f.storedKey = key
	return nil
}

func (f *fakeRepo) Workspaces(_ context.Context, _ int64) ([]dao.WorkspaceMembership, error) {
	return f.memberships, nil
}

func (f *fakeRepo) Workspace(_ context.Context, id int64) (*dao.WorkspaceDetail, error) {
	detail, ok := f.workspaces[id]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return detail, nil
}

func (f *fakeRepo) UpdateWorkspace(_ context.Context, id int64, name, description string) error {
	detail := f.workspaces[id]
	if detail == nil {
		return dao.ErrNotFound
	}
	detail.Name = name
	detail.Description = description
	return nil
}

func (f *fakeRepo) UpdateWorkspaceAvatar(_ context.Context, id int64, key string) error {
	detail := f.workspaces[id]
	if detail == nil {
		return dao.ErrNotFound
	}
	detail.AvatarS3Path = key
	f.storedKey = key
	return nil
}

func (f *fakeRepo) Members(_ context.Context, id int64) ([]dao.WorkspaceMember, error) {
	return f.members[id], nil
}

func (f *fakeRepo) IsWorkspaceMember(_ context.Context, workspaceID, userID int64) (bool, error) {
	for _, member := range f.members[workspaceID] {
		if member.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRepo) AddWorkspaceMember(_ context.Context, workspaceID, userID int64, role string, _ int64, joinedAt time.Time) error {
	f.added++
	f.members[workspaceID] = append(f.members[workspaceID], dao.WorkspaceMember{
		UserID: userID, Username: f.profile.Username, Email: f.profile.Email, Role: role, Status: 1, JoinedAt: &joinedAt,
	})
	return nil
}

func (f *fakeRepo) Folders(_ context.Context, workspaceID int64) ([]dao.WorkspaceFolderRecord, error) {
	return f.folders[workspaceID], nil
}

func (f *fakeRepo) Folder(_ context.Context, workspaceID, folderID int64) (*dao.WorkspaceFolderRecord, error) {
	for i := range f.folders[workspaceID] {
		if f.folders[workspaceID][i].ID == folderID {
			return &f.folders[workspaceID][i], nil
		}
	}
	return nil, dao.ErrNotFound
}

func (f *fakeRepo) FolderNameExists(_ context.Context, workspaceID, parentID int64, name string) (bool, error) {
	for _, folder := range f.folders[workspaceID] {
		if folder.ParentID == parentID && folder.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeRepo) CreateFolder(_ context.Context, workspaceID, parentID int64, name string, createdBy int64) (*dao.WorkspaceFolderRecord, error) {
	folder := dao.WorkspaceFolderRecord{
		ID: int64(len(f.folders[workspaceID]) + 1), WorkspaceID: workspaceID, ParentID: parentID, Name: name,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
	}
	f.folders[workspaceID] = append(f.folders[workspaceID], folder)
	return &folder, nil
}

func (f *fakeRepo) UserQuota(_ context.Context, _ int64) (*dao.UserQuotaRecord, error) {
	if f.quota == nil {
		return nil, dao.ErrNotFound
	}
	return f.quota, nil
}

func (f *fakeRepo) UpdateStorageUsed(_ context.Context, _ int64, used int64) error {
	f.updatedUsed = used
	return nil
}

func testAccount(t *testing.T) (*Service, *fakeRepo, *fakePresigner) {
	t.Helper()
	repo := newFakeRepo()
	repo.profile = &dao.UserProfile{ID: 7, Username: "wanghg11", Email: "wang@example.com", AvatarS3Path: "avatar/7/old.png"}
	repo.memberships = []dao.WorkspaceMembership{
		{ID: 1, Name: "team", OwnerUserID: 9, Role: "editor"},
		{ID: 5, Name: "personal", OwnerUserID: 7, Role: "owner"},
	}
	repo.workspaces[1] = &dao.WorkspaceDetail{ID: 1, Name: "team", OwnerUserID: 9, MemberCount: 2}
	repo.workspaces[5] = &dao.WorkspaceDetail{ID: 5, Name: "personal", OwnerUserID: 7, MemberCount: 1}
	repo.members[5] = []dao.WorkspaceMember{{UserID: 7, Username: "wanghg11", Email: "wang@example.com", Role: "owner", Status: 1}}
	presigner := &fakePresigner{headSize: 128, headType: "image/png"}
	svc := NewService(repo, nil, presigner, 15*time.Minute)
	svc.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	svc.NewID = func() string { return "fixed" }
	return svc, repo, presigner
}

func userSession(current int64) *auth.Session {
	return &auth.Session{ID: "sid", Realm: auth.RealmUser, UserID: 7, Username: "wanghg11", Email: "wang@example.com", WorkspaceIDs: []int64{1, 5}, CurrentWorkspaceID: current}
}

func TestMeRepairsMissingWorkspaceAndPresignsAvatar(t *testing.T) {
	svc, _, presigner := testAccount(t)
	me, err := svc.Me(context.Background(), userSession(0))
	if err != nil {
		t.Fatalf("me: %v", err)
	}
	if me.CurrentWorkspaceID != 5 {
		t.Fatalf("current = %d, want owned workspace 5", me.CurrentWorkspaceID)
	}
	if me.AvatarURL != "https://s3.example/get/avatar/7/old.png" || !presigner.presignGetOK {
		t.Fatalf("avatar url = %q", me.AvatarURL)
	}
	if len(me.Workspaces) != 2 || me.Workspaces[1].Role != "owner" || !me.Workspaces[1].IsOwner {
		t.Fatalf("workspaces = %+v", me.Workspaces)
	}
}

func TestUpdateUsername(t *testing.T) {
	svc, repo, _ := testAccount(t)
	session := userSession(5)
	me, err := svc.UpdateUsername(context.Background(), session, "  newname ")
	if err != nil {
		t.Fatalf("update username: %v", err)
	}
	if me.Username != "newname" || session.Username != "newname" || repo.profile.Username != "newname" {
		t.Fatalf("username = %+v / %+v", me, session)
	}

	for _, bad := range []string{"a", strings.Repeat("x", 65)} {
		if _, err := svc.UpdateUsername(context.Background(), session, bad); !errors.Is(err, ErrInvalidUsername) {
			t.Fatalf("bad %q err = %v", bad, err)
		}
	}
	repo.usernameUsed = true
	if _, err := svc.UpdateUsername(context.Background(), session, "taken"); !errors.Is(err, ErrUsernameConflict) {
		t.Fatalf("conflict err = %v", err)
	}
}

func TestUserAvatarPresignAndConfirm(t *testing.T) {
	svc, repo, presigner := testAccount(t)
	presign, err := svc.PresignUserAvatar(context.Background(), 7, "a.png", "image/png", 128)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if presign.Key != "avatar/7/fixed.png" || presigner.putType != "image/png" || presign.Headers["Content-Type"] != "image/png" {
		t.Fatalf("presign = %+v headers=%+v", presign, presign.Headers)
	}
	for _, tc := range []struct {
		contentType string
		size        int64
	}{
		{"image/gif", 100},
		{"image/png", MaxAvatarBytes + 1},
	} {
		if _, err := svc.PresignUserAvatar(context.Background(), 7, "a", tc.contentType, tc.size); err == nil {
			t.Fatalf("presign %s/%d should fail", tc.contentType, tc.size)
		}
	}

	session := userSession(5)
	if _, err := svc.ConfirmUserAvatar(context.Background(), session, "avatar/8/fixed.png"); !errors.Is(err, ErrAvatarNotFound) {
		t.Fatalf("foreign key err = %v", err)
	}
	presigner.headErr = errors.New("missing")
	if _, err := svc.ConfirmUserAvatar(context.Background(), session, "avatar/7/fixed.png"); !errors.Is(err, ErrAvatarNotFound) {
		t.Fatalf("missing upload err = %v", err)
	}
	presigner.headErr = nil
	if _, err := svc.ConfirmUserAvatar(context.Background(), session, "avatar/7/fixed.png"); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if repo.profile.AvatarS3Path != "avatar/7/fixed.png" {
		t.Fatalf("stored key = %q", repo.profile.AvatarS3Path)
	}
}

func TestUploadUserAvatar(t *testing.T) {
	svc, repo, presigner := testAccount(t)
	me, err := svc.UploadUserAvatar(context.Background(), userSession(5), "image/png", 5, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if presigner.uploadKey != "avatar/7/fixed.png" || presigner.uploadType != "image/png" || presigner.uploadSize != 5 || presigner.uploadBody != "hello" {
		t.Fatalf("upload = %q/%q/%d/%q", presigner.uploadKey, presigner.uploadType, presigner.uploadSize, presigner.uploadBody)
	}
	if repo.profile.AvatarS3Path != "avatar/7/fixed.png" || me.ID != 7 {
		t.Fatalf("stored = %q, me = %+v", repo.profile.AvatarS3Path, me)
	}

	for _, tc := range []struct {
		contentType string
		size        int64
	}{
		{"image/gif", 5},
		{"image/png", 0},
		{"image/png", MaxAvatarBytes + 1},
	} {
		if _, err := svc.UploadUserAvatar(context.Background(), userSession(5), tc.contentType, tc.size, strings.NewReader("hello")); err == nil {
			t.Fatalf("upload %s/%d should fail", tc.contentType, tc.size)
		}
	}

	presigner.uploadErr = errors.New("s3 down")
	if _, err := svc.UploadUserAvatar(context.Background(), userSession(5), "image/png", 5, strings.NewReader("hello")); err == nil {
		t.Fatal("upload s3 error = nil")
	}
}

func TestUploadWorkspaceAvatarOwnerOnly(t *testing.T) {
	svc, repo, presigner := testAccount(t)
	if _, err := svc.UploadWorkspaceAvatar(context.Background(), userSession(1), "image/png", 5, strings.NewReader("hello")); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("non-owner err = %v", err)
	}

	for _, tc := range []struct {
		contentType string
		size        int64
	}{
		{"image/gif", 5},
		{"image/png", 0},
		{"image/png", MaxAvatarBytes + 1},
	} {
		if _, err := svc.UploadWorkspaceAvatar(context.Background(), userSession(5), tc.contentType, tc.size, strings.NewReader("hello")); err == nil {
			t.Fatalf("upload %s/%d should fail", tc.contentType, tc.size)
		}
	}

	workspace, err := svc.UploadWorkspaceAvatar(context.Background(), userSession(5), "image/png", 5, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if presigner.uploadKey != "workspace-avatar/5/fixed.png" || presigner.uploadBody != "hello" {
		t.Fatalf("upload = %q/%q", presigner.uploadKey, presigner.uploadBody)
	}
	if repo.workspaces[5].AvatarS3Path != "workspace-avatar/5/fixed.png" || workspace.ID != 5 {
		t.Fatalf("stored = %q, workspace = %+v", repo.workspaces[5].AvatarS3Path, workspace)
	}
}

func TestWorkspaceReadUpdateAndOwnerGuard(t *testing.T) {
	svc, repo, _ := testAccount(t)
	workspace, err := svc.CurrentWorkspace(context.Background(), userSession(5))
	if err != nil {
		t.Fatalf("current workspace: %v", err)
	}
	if !workspace.IsOwner || workspace.Role != "owner" || workspace.MemberCount != 1 {
		t.Fatalf("workspace = %+v", workspace)
	}

	if _, err := svc.UpdateWorkspace(context.Background(), userSession(1), "new", "desc"); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("non-owner err = %v", err)
	}
	updated, err := svc.UpdateWorkspace(context.Background(), userSession(5), "  renamed ", " desc ")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "renamed" || updated.Description != "desc" || repo.workspaces[5].Name != "renamed" {
		t.Fatalf("updated = %+v", updated)
	}
	if _, err := svc.UpdateWorkspace(context.Background(), userSession(5), "", "desc"); !errors.Is(err, ErrInvalidWorkspaceInput) {
		t.Fatalf("empty name err = %v", err)
	}
}

func TestWorkspaceAvatarOwnerOnly(t *testing.T) {
	svc, repo, _ := testAccount(t)
	if _, err := svc.PresignWorkspaceAvatar(context.Background(), userSession(1), "a.png", "image/png", 100); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("non-owner presign err = %v", err)
	}
	presign, err := svc.PresignWorkspaceAvatar(context.Background(), userSession(5), "a.png", "image/png", 100)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if presign.Key != "workspace-avatar/5/fixed.png" {
		t.Fatalf("key = %q", presign.Key)
	}
	if _, err := svc.ConfirmWorkspaceAvatar(context.Background(), userSession(5), "workspace-avatar/1/fixed.png"); !errors.Is(err, ErrAvatarNotFound) {
		t.Fatalf("foreign key err = %v", err)
	}
	if _, err := svc.ConfirmWorkspaceAvatar(context.Background(), userSession(5), presign.Key); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if repo.workspaces[5].AvatarS3Path != presign.Key {
		t.Fatalf("stored key = %q", repo.workspaces[5].AvatarS3Path)
	}
}

func TestMembersAndInvite(t *testing.T) {
	svc, repo, _ := testAccount(t)
	repo.profile = &dao.UserProfile{ID: 7, Username: "wanghg11", Email: "wang@example.com"}
	repo.usersByEmail["wang@example.com"] = repo.profile
	repo.usersByEmail["vistor@example.com"] = &dao.UserProfile{ID: 8, Username: "vistor", Email: "vistor@example.com"}
	repo.members[5] = []dao.WorkspaceMember{{UserID: 7, Username: "wanghg11", Email: "wang@example.com", Role: "owner", Status: 1}}

	members, err := svc.Members(context.Background(), userSession(5))
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 1 || !members[0].IsOwner {
		t.Fatalf("members = %+v", members)
	}

	if _, err := svc.InviteMember(context.Background(), userSession(1), "vistor@example.com"); !errors.Is(err, ErrWorkspaceForbidden) {
		t.Fatalf("non-owner invite err = %v", err)
	}
	if _, err := svc.InviteMember(context.Background(), userSession(5), "missing@example.com"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing invite err = %v", err)
	}
	if _, err := svc.InviteMember(context.Background(), userSession(5), "wang@example.com"); !errors.Is(err, ErrCannotInviteSelf) {
		t.Fatalf("self invite err = %v", err)
	}
	invited, err := svc.InviteMember(context.Background(), userSession(5), "vistor@example.com")
	if err != nil {
		t.Fatalf("invite: %v", err)
	}
	if invited.UserID != 8 || invited.Role != "editor" || invited.Status != 1 || repo.added != 1 {
		t.Fatalf("invited = %+v", invited)
	}
	repo.members[5] = append(repo.members[5], dao.WorkspaceMember{UserID: 8, Role: "editor", Status: 1})
	if _, err := svc.InviteMember(context.Background(), userSession(5), "vistor@example.com"); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("duplicate invite err = %v", err)
	}
}

func TestQuotaAndSync(t *testing.T) {
	svc, repo, _ := testAccount(t)
	repo.quota = &dao.UserQuotaRecord{StorageQuotaBytes: 10 << 30, StorageUsedBytes: 10}
	quota, err := svc.Quota(context.Background(), 7)
	if err != nil {
		t.Fatalf("quota: %v", err)
	}
	if quota.StorageQuotaBytes != 10<<30 || quota.StorageUsedBytes != 10 {
		t.Fatalf("quota = %+v", quota)
	}
	if err := svc.SyncStorageUsed(context.Background(), 7, 42); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if repo.updatedUsed != 42 {
		t.Fatalf("updated used = %d", repo.updatedUsed)
	}
}

func TestWorkspaceFolders(t *testing.T) {
	svc, _, _ := testAccount(t)
	session := userSession(5)
	folder, err := svc.CreateFolder(context.Background(), session, " 设计稿 ", 0)
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if folder.Name != "设计稿" || folder.ParentID != 0 || folder.ID == 0 {
		t.Fatalf("folder = %+v", folder)
	}
	list, err := svc.Folders(context.Background(), session)
	if err != nil || len(list) != 1 || list[0].Name != "设计稿" {
		t.Fatalf("folders = %+v err = %v", list, err)
	}
	if _, err := svc.CreateFolder(context.Background(), session, "设计稿", 0); !errors.Is(err, ErrFolderExists) {
		t.Fatalf("duplicate err = %v", err)
	}
	if _, err := svc.CreateFolder(context.Background(), session, "a/b", 0); !errors.Is(err, ErrInvalidFolderName) {
		t.Fatalf("invalid err = %v", err)
	}
	if _, err := svc.CreateFolder(context.Background(), session, "nested", 99); !errors.Is(err, ErrFolderNotFound) {
		t.Fatalf("parent err = %v", err)
	}
}
