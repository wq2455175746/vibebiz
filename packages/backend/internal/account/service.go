package account

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vibebiz/internal/auth"
	"vibebiz/internal/dao"
)

const MaxAvatarBytes = 5 << 20

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUsernameConflict      = errors.New("username already exists")
	ErrInvalidUsername       = errors.New("invalid username")
	ErrInvalidAvatarType     = errors.New("invalid avatar type")
	ErrInvalidAvatarSize     = errors.New("invalid avatar size")
	ErrAvatarNotFound        = errors.New("avatar upload not found")
	ErrWorkspaceNotFound     = errors.New("workspace not found")
	ErrWorkspaceForbidden    = errors.New("workspace owner required")
	ErrAlreadyMember         = errors.New("already a workspace member")
	ErrCannotInviteSelf      = errors.New("cannot invite yourself")
	ErrInvalidWorkspaceInput = errors.New("invalid workspace input")
	ErrInvalidFolderName     = errors.New("invalid folder name")
	ErrFolderExists          = errors.New("folder already exists")
	ErrFolderNotFound        = errors.New("folder not found")
)

type Presigner interface {
	PresignPut(ctx context.Context, key, contentType string, size int64, expiry time.Duration) (string, map[string]string, error)
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	Head(ctx context.Context, key string) (int64, string, error)
}

type Repository interface {
	UserProfile(ctx context.Context, userID int64) (*dao.UserProfile, error)
	FindUserByEmail(ctx context.Context, email string) (*dao.UserProfile, error)
	UsernameExists(ctx context.Context, username string, excludeUserID int64) (bool, error)
	UpdateUsername(ctx context.Context, userID int64, username string) error
	UpdateUserAvatar(ctx context.Context, userID int64, key string) error
	Workspaces(ctx context.Context, userID int64) ([]dao.WorkspaceMembership, error)
	Workspace(ctx context.Context, workspaceID int64) (*dao.WorkspaceDetail, error)
	UpdateWorkspace(ctx context.Context, workspaceID int64, name, description string) error
	UpdateWorkspaceAvatar(ctx context.Context, workspaceID int64, key string) error
	Members(ctx context.Context, workspaceID int64) ([]dao.WorkspaceMember, error)
	IsWorkspaceMember(ctx context.Context, workspaceID, userID int64) (bool, error)
	AddWorkspaceMember(ctx context.Context, workspaceID, userID int64, role string, invitedBy int64, joinedAt time.Time) error
	Folders(ctx context.Context, workspaceID int64) ([]dao.WorkspaceFolderRecord, error)
	Folder(ctx context.Context, workspaceID, folderID int64) (*dao.WorkspaceFolderRecord, error)
	FolderNameExists(ctx context.Context, workspaceID, parentID int64, name string) (bool, error)
	CreateFolder(ctx context.Context, workspaceID, parentID int64, name string, createdBy int64) (*dao.WorkspaceFolderRecord, error)
	UserQuota(ctx context.Context, userID int64) (*dao.UserQuotaRecord, error)
	UpdateStorageUsed(ctx context.Context, userID int64, used int64) error
}

type Service struct {
	Repo     Repository
	Sessions auth.Store
	S3       Presigner
	Expiry   time.Duration
	Now      func() time.Time
	NewID    func() string
}

func NewService(repo Repository, sessions auth.Store, presigner Presigner, expiry time.Duration) *Service {
	return &Service{
		Repo:     repo,
		Sessions: sessions,
		S3:       presigner,
		Expiry:   expiry,
		Now:      time.Now,
		NewID:    uuid.NewString,
	}
}

type WorkspaceSummary struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	IsOwner bool   `json:"isOwner"`
	Role    string `json:"role"`
}

type Me struct {
	ID                 int64              `json:"id"`
	Username           string             `json:"username"`
	Email              string             `json:"email"`
	AvatarURL          string             `json:"avatarUrl"`
	CurrentWorkspaceID int64              `json:"currentWorkspaceId"`
	Workspaces         []WorkspaceSummary `json:"workspaces"`
}

type Workspace struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatarUrl"`
	IsOwner     bool   `json:"isOwner"`
	Role        string `json:"role"`
	MemberCount int64  `json:"memberCount"`
}

type Member struct {
	UserID    int64      `json:"userId"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	AvatarURL string     `json:"avatarUrl"`
	Role      string     `json:"role"`
	Status    int8       `json:"status"`
	JoinedAt  *time.Time `json:"joinedAt"`
	IsOwner   bool       `json:"isOwner"`
}

type Presign struct {
	UploadURL string            `json:"uploadUrl"`
	Key       string            `json:"key"`
	Headers   map[string]string `json:"headers"`
}

type Quota struct {
	StorageQuotaBytes int64 `json:"storageQuotaBytes"`
	StorageUsedBytes  int64 `json:"storageUsedBytes"`
}

type Folder struct {
	ID        int64     `json:"id"`
	ParentID  int64     `json:"parentId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Service) Me(ctx context.Context, session *auth.Session) (Me, error) {
	profile, err := s.Repo.UserProfile(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return Me{}, ErrUserNotFound
		}
		return Me{}, err
	}
	memberships, currentID, err := s.resolveWorkspaces(ctx, session)
	if err != nil {
		return Me{}, err
	}
	me := Me{
		ID:                 profile.ID,
		Username:           profile.Username,
		Email:              profile.Email,
		AvatarURL:          s.avatarURL(ctx, profile.AvatarS3Path),
		CurrentWorkspaceID: currentID,
		Workspaces:         make([]WorkspaceSummary, 0, len(memberships)),
	}
	for _, membership := range memberships {
		isOwner := membership.OwnerUserID == session.UserID
		role := membership.Role
		if isOwner && role == "" {
			role = "owner"
		}
		me.Workspaces = append(me.Workspaces, WorkspaceSummary{
			ID:      membership.ID,
			Name:    membership.Name,
			IsOwner: isOwner,
			Role:    role,
		})
	}
	session.Username = me.Username
	session.Email = me.Email
	return me, nil
}

func (s *Service) UpdateUsername(ctx context.Context, session *auth.Session, username string) (Me, error) {
	username = strings.TrimSpace(username)
	if count := utf8.RuneCountInString(username); count < 2 || count > 64 {
		return Me{}, ErrInvalidUsername
	}
	exists, err := s.Repo.UsernameExists(ctx, username, session.UserID)
	if err != nil {
		return Me{}, err
	}
	if exists {
		return Me{}, ErrUsernameConflict
	}
	if err := s.Repo.UpdateUsername(ctx, session.UserID, username); err != nil {
		return Me{}, err
	}
	session.Username = username
	if err := s.updateSession(ctx, session); err != nil {
		return Me{}, err
	}
	return s.Me(ctx, session)
}

func (s *Service) PresignUserAvatar(ctx context.Context, userID int64, filename, contentType string, size int64) (Presign, error) {
	ext, err := avatarExtension(contentType)
	if err != nil {
		return Presign{}, err
	}
	if size <= 0 || size > MaxAvatarBytes {
		return Presign{}, ErrInvalidAvatarSize
	}
	if s.S3 == nil {
		return Presign{}, fmt.Errorf("avatar storage is not configured")
	}
	key := fmt.Sprintf("avatar/%d/%s.%s", userID, s.newID(), ext)
	uploadURL, headers, err := s.S3.PresignPut(ctx, key, contentType, size, s.expiry())
	if err != nil {
		return Presign{}, err
	}
	return Presign{UploadURL: uploadURL, Key: key, Headers: headers}, nil
}

func (s *Service) ConfirmUserAvatar(ctx context.Context, session *auth.Session, key string) (Me, error) {
	prefix := fmt.Sprintf("avatar/%d/", session.UserID)
	if !validObjectKey(key, prefix) {
		return Me{}, ErrAvatarNotFound
	}
	if err := s.confirmAvatar(ctx, key); err != nil {
		return Me{}, err
	}
	if err := s.Repo.UpdateUserAvatar(ctx, session.UserID, key); err != nil {
		return Me{}, err
	}
	return s.Me(ctx, session)
}

func (s *Service) UploadUserAvatar(ctx context.Context, session *auth.Session, contentType string, size int64, body io.Reader) (Me, error) {
	ext, err := avatarExtension(contentType)
	if err != nil {
		return Me{}, err
	}
	if size <= 0 || size > MaxAvatarBytes {
		return Me{}, ErrInvalidAvatarSize
	}
	if s.S3 == nil {
		return Me{}, fmt.Errorf("avatar storage is not configured")
	}
	key := fmt.Sprintf("avatar/%d/%s.%s", session.UserID, s.newID(), ext)
	if err := s.S3.Put(ctx, key, contentType, size, body); err != nil {
		return Me{}, err
	}
	if err := s.Repo.UpdateUserAvatar(ctx, session.UserID, key); err != nil {
		return Me{}, err
	}
	return s.Me(ctx, session)
}

func (s *Service) CurrentWorkspace(ctx context.Context, session *auth.Session) (Workspace, error) {
	memberships, currentID, err := s.resolveWorkspaces(ctx, session)
	if err != nil {
		return Workspace{}, err
	}
	detail, err := s.workspaceDetail(ctx, currentID)
	if err != nil {
		return Workspace{}, err
	}
	return s.workspaceFromDetail(ctx, detail, membershipByID(memberships, currentID), session.UserID), nil
}

func (s *Service) UpdateWorkspace(ctx context.Context, session *auth.Session, name, description string) (Workspace, error) {
	detail, err := s.ownedWorkspace(ctx, session)
	if err != nil {
		return Workspace{}, err
	}
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if count := utf8.RuneCountInString(name); count < 1 || count > 100 {
		return Workspace{}, ErrInvalidWorkspaceInput
	}
	if utf8.RuneCountInString(description) > 500 {
		return Workspace{}, ErrInvalidWorkspaceInput
	}
	if err := s.Repo.UpdateWorkspace(ctx, detail.ID, name, description); err != nil {
		return Workspace{}, err
	}
	detail.Name = name
	detail.Description = description
	return s.CurrentWorkspace(ctx, session)
}

func (s *Service) PresignWorkspaceAvatar(ctx context.Context, session *auth.Session, filename, contentType string, size int64) (Presign, error) {
	detail, err := s.ownedWorkspace(ctx, session)
	if err != nil {
		return Presign{}, err
	}
	ext, err := avatarExtension(contentType)
	if err != nil {
		return Presign{}, err
	}
	if size <= 0 || size > MaxAvatarBytes {
		return Presign{}, ErrInvalidAvatarSize
	}
	if s.S3 == nil {
		return Presign{}, fmt.Errorf("avatar storage is not configured")
	}
	key := fmt.Sprintf("workspace-avatar/%d/%s.%s", detail.ID, s.newID(), ext)
	uploadURL, headers, err := s.S3.PresignPut(ctx, key, contentType, size, s.expiry())
	if err != nil {
		return Presign{}, err
	}
	return Presign{UploadURL: uploadURL, Key: key, Headers: headers}, nil
}

func (s *Service) ConfirmWorkspaceAvatar(ctx context.Context, session *auth.Session, key string) (Workspace, error) {
	detail, err := s.ownedWorkspace(ctx, session)
	if err != nil {
		return Workspace{}, err
	}
	prefix := fmt.Sprintf("workspace-avatar/%d/", detail.ID)
	if !validObjectKey(key, prefix) {
		return Workspace{}, ErrAvatarNotFound
	}
	if err := s.confirmAvatar(ctx, key); err != nil {
		return Workspace{}, err
	}
	if err := s.Repo.UpdateWorkspaceAvatar(ctx, detail.ID, key); err != nil {
		return Workspace{}, err
	}
	return s.CurrentWorkspace(ctx, session)
}

func (s *Service) UploadWorkspaceAvatar(ctx context.Context, session *auth.Session, contentType string, size int64, body io.Reader) (Workspace, error) {
	detail, err := s.ownedWorkspace(ctx, session)
	if err != nil {
		return Workspace{}, err
	}
	ext, err := avatarExtension(contentType)
	if err != nil {
		return Workspace{}, err
	}
	if size <= 0 || size > MaxAvatarBytes {
		return Workspace{}, ErrInvalidAvatarSize
	}
	if s.S3 == nil {
		return Workspace{}, fmt.Errorf("avatar storage is not configured")
	}
	key := fmt.Sprintf("workspace-avatar/%d/%s.%s", detail.ID, s.newID(), ext)
	if err := s.S3.Put(ctx, key, contentType, size, body); err != nil {
		return Workspace{}, err
	}
	if err := s.Repo.UpdateWorkspaceAvatar(ctx, detail.ID, key); err != nil {
		return Workspace{}, err
	}
	return s.CurrentWorkspace(ctx, session)
}

func (s *Service) Members(ctx context.Context, session *auth.Session) ([]Member, error) {
	_, currentID, err := s.resolveWorkspaces(ctx, session)
	if err != nil {
		return nil, err
	}
	member, err := s.Repo.IsWorkspaceMember(ctx, currentID, session.UserID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrWorkspaceForbidden
	}
	records, err := s.Repo.Members(ctx, currentID)
	if err != nil {
		return nil, err
	}
	detail, err := s.workspaceDetail(ctx, currentID)
	if err != nil {
		return nil, err
	}
	members := make([]Member, 0, len(records))
	for _, record := range records {
		members = append(members, Member{
			UserID:    record.UserID,
			Username:  record.Username,
			Email:     record.Email,
			AvatarURL: s.avatarURL(ctx, record.AvatarS3Path),
			Role:      record.Role,
			Status:    record.Status,
			JoinedAt:  record.JoinedAt,
			IsOwner:   record.UserID == detail.OwnerUserID,
		})
	}
	return members, nil
}

func (s *Service) InviteMember(ctx context.Context, session *auth.Session, email string) (Member, error) {
	detail, err := s.ownedWorkspace(ctx, session)
	if err != nil {
		return Member{}, err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return Member{}, ErrUserNotFound
	}
	profile, err := s.Repo.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return Member{}, ErrUserNotFound
		}
		return Member{}, err
	}
	if profile.ID == session.UserID {
		return Member{}, ErrCannotInviteSelf
	}
	member, err := s.Repo.IsWorkspaceMember(ctx, detail.ID, profile.ID)
	if err != nil {
		return Member{}, err
	}
	if member {
		return Member{}, ErrAlreadyMember
	}
	now := s.now()
	if err := s.Repo.AddWorkspaceMember(ctx, detail.ID, profile.ID, "editor", session.UserID, now); err != nil {
		return Member{}, err
	}
	return Member{
		UserID:    profile.ID,
		Username:  profile.Username,
		Email:     profile.Email,
		AvatarURL: s.avatarURL(ctx, profile.AvatarS3Path),
		Role:      "editor",
		Status:    1,
		JoinedAt:  &now,
		IsOwner:   profile.ID == detail.OwnerUserID,
	}, nil
}

func (s *Service) Folders(ctx context.Context, session *auth.Session) ([]Folder, error) {
	_, workspaceID, err := s.resolveWorkspaces(ctx, session)
	if err != nil {
		return nil, err
	}
	records, err := s.Repo.Folders(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	folders := make([]Folder, 0, len(records))
	for _, record := range records {
		folders = append(folders, Folder{
			ID:        record.ID,
			ParentID:  record.ParentID,
			Name:      record.Name,
			CreatedAt: record.CreatedAt,
		})
	}
	return folders, nil
}

func (s *Service) CreateFolder(ctx context.Context, session *auth.Session, name string, parentID int64) (Folder, error) {
	_, workspaceID, err := s.resolveWorkspaces(ctx, session)
	if err != nil {
		return Folder{}, err
	}
	name, err = normalizeFolderName(name)
	if err != nil {
		return Folder{}, err
	}
	if parentID < 0 {
		return Folder{}, ErrInvalidFolderName
	}
	if parentID > 0 {
		if _, err := s.Repo.Folder(ctx, workspaceID, parentID); err != nil {
			if errors.Is(err, dao.ErrNotFound) {
				return Folder{}, ErrFolderNotFound
			}
			return Folder{}, err
		}
	}
	exists, err := s.Repo.FolderNameExists(ctx, workspaceID, parentID, name)
	if err != nil {
		return Folder{}, err
	}
	if exists {
		return Folder{}, ErrFolderExists
	}
	record, err := s.Repo.CreateFolder(ctx, workspaceID, parentID, name, session.UserID)
	if err != nil {
		return Folder{}, err
	}
	return Folder{ID: record.ID, ParentID: record.ParentID, Name: record.Name, CreatedAt: record.CreatedAt}, nil
}

func (s *Service) Quota(ctx context.Context, userID int64) (Quota, error) {
	record, err := s.Repo.UserQuota(ctx, userID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return Quota{}, nil
		}
		return Quota{}, err
	}
	return Quota{StorageQuotaBytes: record.StorageQuotaBytes, StorageUsedBytes: record.StorageUsedBytes}, nil
}

func (s *Service) SyncStorageUsed(ctx context.Context, userID, used int64) error {
	return s.Repo.UpdateStorageUsed(ctx, userID, used)
}

func normalizeFolderName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if count := utf8.RuneCountInString(name); count < 1 || count > 64 {
		return "", ErrInvalidFolderName
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return "", ErrInvalidFolderName
	}
	for _, r := range name {
		if r < 0x20 {
			return "", ErrInvalidFolderName
		}
	}
	return name, nil
}

func (s *Service) resolveWorkspaces(ctx context.Context, session *auth.Session) ([]dao.WorkspaceMembership, int64, error) {
	memberships, err := s.Repo.Workspaces(ctx, session.UserID)
	if err != nil {
		return nil, 0, err
	}
	if len(memberships) == 0 {
		return nil, 0, auth.ErrNoWorkspace
	}
	currentID := session.CurrentWorkspaceID
	if !hasMembership(memberships, currentID) {
		currentID = defaultWorkspaceID(memberships, session.UserID)
	}
	ids := make([]int64, 0, len(memberships))
	for _, membership := range memberships {
		ids = append(ids, membership.ID)
	}
	changed := session.CurrentWorkspaceID != currentID || !equalIDs(session.WorkspaceIDs, ids)
	session.CurrentWorkspaceID = currentID
	session.WorkspaceIDs = ids
	if changed {
		if err := s.updateSession(ctx, session); err != nil {
			return nil, 0, err
		}
	}
	return memberships, currentID, nil
}

func (s *Service) ownedWorkspace(ctx context.Context, session *auth.Session) (*dao.WorkspaceDetail, error) {
	_, currentID, err := s.resolveWorkspaces(ctx, session)
	if err != nil {
		return nil, err
	}
	detail, err := s.workspaceDetail(ctx, currentID)
	if err != nil {
		return nil, err
	}
	if detail.OwnerUserID != session.UserID {
		return nil, ErrWorkspaceForbidden
	}
	return detail, nil
}

func (s *Service) workspaceDetail(ctx context.Context, workspaceID int64) (*dao.WorkspaceDetail, error) {
	detail, err := s.Repo.Workspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return nil, ErrWorkspaceNotFound
		}
		return nil, err
	}
	return detail, nil
}

func (s *Service) workspaceFromDetail(ctx context.Context, detail *dao.WorkspaceDetail, membership dao.WorkspaceMembership, userID int64) Workspace {
	isOwner := detail.OwnerUserID == userID
	role := membership.Role
	if isOwner && role == "" {
		role = "owner"
	}
	return Workspace{
		ID:          detail.ID,
		Name:        detail.Name,
		Description: detail.Description,
		AvatarURL:   s.avatarURL(ctx, detail.AvatarS3Path),
		IsOwner:     isOwner,
		Role:        role,
		MemberCount: detail.MemberCount,
	}
}

func (s *Service) confirmAvatar(ctx context.Context, key string) error {
	if s.S3 == nil {
		return fmt.Errorf("avatar storage is not configured")
	}
	size, contentType, err := s.S3.Head(ctx, key)
	if err != nil {
		return ErrAvatarNotFound
	}
	if _, err := avatarExtension(contentType); err != nil || size <= 0 || size > MaxAvatarBytes {
		return ErrAvatarNotFound
	}
	return nil
}

func (s *Service) avatarURL(ctx context.Context, key string) string {
	if key == "" || s.S3 == nil {
		return ""
	}
	url, err := s.S3.PresignGet(ctx, key, s.expiry())
	if err != nil {
		return ""
	}
	return url
}

func (s *Service) updateSession(ctx context.Context, session *auth.Session) error {
	if s.Sessions == nil {
		return nil
	}
	return s.Sessions.Update(ctx, session)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) newID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewString()
}

func (s *Service) expiry() time.Duration {
	if s.Expiry > 0 {
		return s.Expiry
	}
	return 15 * time.Minute
}

func avatarExtension(contentType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return "jpg", nil
	case "image/png":
		return "png", nil
	case "image/webp":
		return "webp", nil
	default:
		return "", ErrInvalidAvatarType
	}
}

func validObjectKey(key, prefix string) bool {
	if !strings.HasPrefix(key, prefix) {
		return false
	}
	rest := strings.TrimPrefix(key, prefix)
	return rest != "" && !strings.Contains(rest, "/") && !strings.Contains(rest, "..")
}

func defaultWorkspaceID(memberships []dao.WorkspaceMembership, userID int64) int64 {
	for _, membership := range memberships {
		if membership.OwnerUserID == userID {
			return membership.ID
		}
	}
	return memberships[0].ID
}

func hasMembership(memberships []dao.WorkspaceMembership, id int64) bool {
	for _, membership := range memberships {
		if membership.ID == id {
			return true
		}
	}
	return false
}

func membershipByID(memberships []dao.WorkspaceMembership, id int64) dao.WorkspaceMembership {
	for _, membership := range memberships {
		if membership.ID == id {
			return membership
		}
	}
	return dao.WorkspaceMembership{}
}

func equalIDs(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
