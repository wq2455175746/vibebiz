package dao

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSettingsUserProfile(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	mock.ExpectQuery("SELECT id, username, email, avatar_s3_path FROM `user` WHERE id = \\?").
		WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "avatar_s3_path"}).
			AddRow(7, "wanghg11", "wang@example.com", "avatar/7/a.png"))

	profile, err := repo.UserProfile(context.Background(), 7)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if profile.ID != 7 || profile.Username != "wanghg11" || profile.AvatarS3Path != "avatar/7/a.png" {
		t.Fatalf("profile = %+v", profile)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsUserProfileNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	mock.ExpectQuery("SELECT .* FROM `user` WHERE id = \\?").
		WithArgs(int64(9), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.UserProfile(context.Background(), 9); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSettingsFindUserByEmail(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	mock.ExpectQuery("SELECT .* FROM `user` WHERE email = \\?").
		WithArgs("vistor@example.com", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email"}).
			AddRow(8, "vistor", "vistor@example.com"))

	profile, err := repo.FindUserByEmail(context.Background(), "vistor@example.com")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if profile.ID != 8 || profile.Username != "vistor" {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestSettingsUsernameExists(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user` WHERE \\(username = \\? AND id <> \\?\\) AND `user`\\.`deleted_at` IS NULL").
		WithArgs("taken", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := repo.UsernameExists(context.Background(), "taken", 7)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}
	if !exists {
		t.Fatal("exists = false, want true")
	}
}

func TestSettingsUpdateUsernameAndAvatar(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET `username`=\\?,`updated_at`=\\? WHERE id = \\?").
		WithArgs("newname", sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateUsername(context.Background(), 7, "newname"); err != nil {
		t.Fatalf("update username: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET `avatar_s3_path`=\\?,`updated_at`=\\? WHERE id = \\?").
		WithArgs("avatar/7/a.png", sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateUserAvatar(context.Background(), 7, "avatar/7/a.png"); err != nil {
		t.Fatalf("update avatar: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsWorkspaces(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	query := regexp.QuoteMeta("SELECT workspace.id, workspace.name, workspace.owner_user_id, workspace_user.role FROM `workspace_user` JOIN workspace ON workspace.id = workspace_user.workspace_id WHERE (workspace_user.user_id = ? AND workspace_user.status = 1 AND workspace_user.deleted_at IS NULL) AND (workspace.status = 1 AND workspace.deleted_at IS NULL) ORDER BY workspace.id ASC")
	mock.ExpectQuery(query).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "owner_user_id", "role"}).
			AddRow(1, "team", 9, "editor").
			AddRow(5, "personal", 7, "owner"))

	memberships, err := repo.Workspaces(context.Background(), 7)
	if err != nil {
		t.Fatalf("workspaces: %v", err)
	}
	if len(memberships) != 2 || memberships[1].ID != 5 || memberships[1].OwnerUserID != 7 || memberships[1].Role != "owner" {
		t.Fatalf("memberships = %+v", memberships)
	}
}

func TestSettingsWorkspaceAndMembers(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	mock.ExpectQuery("SELECT id, owner_user_id, name, description, avatar_url, avatar_s3_path FROM `workspace` WHERE id = \\?").
		WithArgs(int64(5), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "owner_user_id", "name", "description", "avatar_url", "avatar_s3_path"}).
			AddRow(5, 7, "personal", "mine", "", "workspace-avatar/5/a.png"))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `workspace_user` WHERE \\(workspace_id = \\? AND status = 1\\) AND `workspace_user`.`deleted_at` IS NULL").
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	detail, err := repo.Workspace(context.Background(), 5)
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	if detail.OwnerUserID != 7 || detail.MemberCount != 2 || detail.AvatarS3Path == "" {
		t.Fatalf("detail = %+v", detail)
	}

	membersQuery := regexp.QuoteMeta("SELECT user.id AS user_id, user.username, user.email, user.avatar_s3_path, workspace_user.role, workspace_user.status, workspace_user.joined_at FROM `workspace_user` JOIN user ON user.id = workspace_user.user_id WHERE (workspace_user.workspace_id = ? AND workspace_user.deleted_at IS NULL) AND user.deleted_at IS NULL ORDER BY workspace_user.role = 'owner' DESC, workspace_user.joined_at ASC")
	mock.ExpectQuery(membersQuery).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "username", "email", "avatar_s3_path", "role", "status", "joined_at"}).
			AddRow(7, "wanghg11", "wang@example.com", "avatar/7/a.png", "owner", 1, time.Unix(1700000000, 0).UTC()))

	members, err := repo.Members(context.Background(), 5)
	if err != nil {
		t.Fatalf("members: %v", err)
	}
	if len(members) != 1 || members[0].UserID != 7 || members[0].Role != "owner" {
		t.Fatalf("members = %+v", members)
	}
}

func TestSettingsQuotaAndMemberInsert(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewSettingsRepository(db)

	mock.ExpectQuery("SELECT storage_quota_bytes, storage_used_bytes FROM `user_quota` WHERE user_id = \\?").
		WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"storage_quota_bytes", "storage_used_bytes"}).
			AddRow(int64(10<<30), int64(100)))

	quota, err := repo.UserQuota(context.Background(), 7)
	if err != nil {
		t.Fatalf("quota: %v", err)
	}
	if quota.StorageQuotaBytes != 10<<30 || quota.StorageUsedBytes != 100 {
		t.Fatalf("quota = %+v", quota)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `workspace_user`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	now := time.Unix(1700000000, 0).UTC()
	if err := repo.AddWorkspaceMember(context.Background(), 5, 8, "editor", 7, now); err != nil {
		t.Fatalf("add member: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_quota` SET `storage_used_bytes`=\\?,`updated_at`=\\? WHERE user_id = \\?").
		WithArgs(int64(200), sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateStorageUsed(context.Background(), 7, 200); err != nil {
		t.Fatalf("update storage used: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
