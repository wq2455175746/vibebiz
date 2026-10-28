package handler

import (
	"context"
	"time"

	"vibebiz/internal/dao"
)

type fakeAccountRepo struct {
	profile       *dao.UserProfile
	usersByEmail  map[string]*dao.UserProfile
	memberships   []dao.WorkspaceMembership
	workspaces    map[int64]*dao.WorkspaceDetail
	members       map[int64][]dao.WorkspaceMember
	folders       map[int64][]dao.WorkspaceFolderRecord
	quota         *dao.UserQuotaRecord
	usernameTaken bool
	updatedUsed   int64
	storedKey     string
}

func newFakeAccountRepo() *fakeAccountRepo {
	return &fakeAccountRepo{
		usersByEmail: make(map[string]*dao.UserProfile),
		workspaces:   make(map[int64]*dao.WorkspaceDetail),
		members:      make(map[int64][]dao.WorkspaceMember),
		folders:      make(map[int64][]dao.WorkspaceFolderRecord),
	}
}

func (f *fakeAccountRepo) UserProfile(_ context.Context, userID int64) (*dao.UserProfile, error) {
	if f.profile == nil || f.profile.ID != userID {
		return nil, dao.ErrNotFound
	}
	return f.profile, nil
}

func (f *fakeAccountRepo) FindUserByEmail(_ context.Context, email string) (*dao.UserProfile, error) {
	profile, ok := f.usersByEmail[email]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return profile, nil
}

func (f *fakeAccountRepo) UsernameExists(_ context.Context, _ string, _ int64) (bool, error) {
	return f.usernameTaken, nil
}

func (f *fakeAccountRepo) UpdateUsername(_ context.Context, _ int64, username string) error {
	if f.profile != nil {
		f.profile.Username = username
	}
	return nil
}

func (f *fakeAccountRepo) UpdateUserAvatar(_ context.Context, _ int64, key string) error {
	if f.profile != nil {
		f.profile.AvatarS3Path = key
	}
	f.storedKey = key
	return nil
}

func (f *fakeAccountRepo) Workspaces(_ context.Context, _ int64) ([]dao.WorkspaceMembership, error) {
	return f.memberships, nil
}

func (f *fakeAccountRepo) Workspace(_ context.Context, workspaceID int64) (*dao.WorkspaceDetail, error) {
	detail, ok := f.workspaces[workspaceID]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return detail, nil
}

func (f *fakeAccountRepo) UpdateWorkspace(_ context.Context, workspaceID int64, name, description string) error {
	detail, ok := f.workspaces[workspaceID]
	if !ok {
		return dao.ErrNotFound
	}
	detail.Name = name
	detail.Description = description
	return nil
}

func (f *fakeAccountRepo) UpdateWorkspaceAvatar(_ context.Context, workspaceID int64, key string) error {
	detail, ok := f.workspaces[workspaceID]
	if !ok {
		return dao.ErrNotFound
	}
	detail.AvatarS3Path = key
	f.storedKey = key
	return nil
}

func (f *fakeAccountRepo) Members(_ context.Context, workspaceID int64) ([]dao.WorkspaceMember, error) {
	return f.members[workspaceID], nil
}

func (f *fakeAccountRepo) IsWorkspaceMember(_ context.Context, workspaceID, userID int64) (bool, error) {
	for _, member := range f.members[workspaceID] {
		if member.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAccountRepo) AddWorkspaceMember(_ context.Context, workspaceID, userID int64, role string, _ int64, joinedAt time.Time) error {
	f.members[workspaceID] = append(f.members[workspaceID], dao.WorkspaceMember{
		UserID: userID, Username: "invited", Email: "invited@example.com", Role: role, Status: 1, JoinedAt: &joinedAt,
	})
	return nil
}

func (f *fakeAccountRepo) Folders(_ context.Context, workspaceID int64) ([]dao.WorkspaceFolderRecord, error) {
	return f.folders[workspaceID], nil
}

func (f *fakeAccountRepo) Folder(_ context.Context, workspaceID, folderID int64) (*dao.WorkspaceFolderRecord, error) {
	for i := range f.folders[workspaceID] {
		if f.folders[workspaceID][i].ID == folderID {
			return &f.folders[workspaceID][i], nil
		}
	}
	return nil, dao.ErrNotFound
}

func (f *fakeAccountRepo) FolderNameExists(_ context.Context, workspaceID, parentID int64, name string) (bool, error) {
	for _, folder := range f.folders[workspaceID] {
		if folder.ParentID == parentID && folder.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeAccountRepo) CreateFolder(_ context.Context, workspaceID, parentID int64, name string, createdBy int64) (*dao.WorkspaceFolderRecord, error) {
	folder := dao.WorkspaceFolderRecord{
		ID: int64(len(f.folders[workspaceID]) + 1), WorkspaceID: workspaceID, ParentID: parentID, Name: name,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
	}
	f.folders[workspaceID] = append(f.folders[workspaceID], folder)
	return &folder, nil
}

func (f *fakeAccountRepo) UserQuota(_ context.Context, _ int64) (*dao.UserQuotaRecord, error) {
	if f.quota == nil {
		return nil, dao.ErrNotFound
	}
	return f.quota, nil
}

func (f *fakeAccountRepo) UpdateStorageUsed(_ context.Context, _ int64, used int64) error {
	f.updatedUsed = used
	return nil
}
