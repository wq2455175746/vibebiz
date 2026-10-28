package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/account"
	"vibebiz/internal/auth"
	"vibebiz/internal/response"
)

type SettingsHandler struct {
	Account    *account.Service
	UserCookie auth.CookieManager
}

func NewSettingsHandler(service *account.Service, cookie auth.CookieManager) *SettingsHandler {
	return &SettingsHandler{Account: service, UserCookie: cookie}
}

type updateProfileRequest struct {
	Username string `json:"username"`
}

type avatarPresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

type avatarConfirmRequest struct {
	Key string `json:"key"`
}

type updateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type inviteMemberRequest struct {
	Email string `json:"email"`
}

type createFolderRequest struct {
	Name     string `json:"name"`
	ParentID int64  `json:"parentId"`
}

func (h *SettingsHandler) GetProfile(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	me, err := h.Account.Me(c.Request().Context(), session)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, me)
}

func (h *SettingsHandler) UpdateProfile(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req updateProfileRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	me, err := h.Account.UpdateUsername(c.Request().Context(), session, req.Username)
	if err != nil {
		return settingsError(c, err)
	}
	if err := h.UserCookie.Set(c, auth.CookieIdentity{SID: session.ID, Username: me.Username, Email: me.Email}); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50005, "set session cookie failed")
	}
	return response.OK(c, me)
}

func (h *SettingsHandler) PresignAvatar(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	req, ok := bindAvatarPresign(c)
	if !ok {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	presign, err := h.Account.PresignUserAvatar(c.Request().Context(), session.UserID, req.Filename, req.ContentType, req.Size)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, presign)
}

func (h *SettingsHandler) ConfirmAvatar(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	req, ok := bindAvatarConfirm(c)
	if !ok {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	me, err := h.Account.ConfirmUserAvatar(c.Request().Context(), session, req.Key)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, me)
}

func (h *SettingsHandler) UploadAvatar(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	contentType, size, body, err := bindAvatarUpload(c)
	if err != nil {
		return settingsError(c, err)
	}
	me, err := h.Account.UploadUserAvatar(c.Request().Context(), session, contentType, size, body)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, me)
}

func (h *SettingsHandler) GetWorkspace(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	workspace, err := h.Account.CurrentWorkspace(c.Request().Context(), session)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, workspace)
}

func (h *SettingsHandler) UpdateWorkspace(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req updateWorkspaceRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	workspace, err := h.Account.UpdateWorkspace(c.Request().Context(), session, req.Name, req.Description)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, workspace)
}

func (h *SettingsHandler) PresignWorkspaceAvatar(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	req, ok := bindAvatarPresign(c)
	if !ok {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	presign, err := h.Account.PresignWorkspaceAvatar(c.Request().Context(), session, req.Filename, req.ContentType, req.Size)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, presign)
}

func (h *SettingsHandler) ConfirmWorkspaceAvatar(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	req, ok := bindAvatarConfirm(c)
	if !ok {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	workspace, err := h.Account.ConfirmWorkspaceAvatar(c.Request().Context(), session, req.Key)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, workspace)
}

func (h *SettingsHandler) UploadWorkspaceAvatar(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	contentType, size, body, err := bindAvatarUpload(c)
	if err != nil {
		return settingsError(c, err)
	}
	workspace, err := h.Account.UploadWorkspaceAvatar(c.Request().Context(), session, contentType, size, body)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, workspace)
}

func (h *SettingsHandler) Members(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	members, err := h.Account.Members(c.Request().Context(), session)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, map[string]any{"members": members})
}

func (h *SettingsHandler) Folders(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	folders, err := h.Account.Folders(c.Request().Context(), session)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, map[string]any{"folders": folders})
}

func (h *SettingsHandler) CreateFolder(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req createFolderRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	folder, err := h.Account.CreateFolder(c.Request().Context(), session, req.Name, req.ParentID)
	if err != nil {
		return settingsError(c, err)
	}
	return response.Created(c, folder)
}

func (h *SettingsHandler) InviteMember(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req inviteMemberRequest
	if err := c.Bind(&req); err != nil || req.Email == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	member, err := h.Account.InviteMember(c.Request().Context(), session, req.Email)
	if err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, member)
}

func bindAvatarPresign(c echo.Context) (avatarPresignRequest, bool) {
	var req avatarPresignRequest
	if err := c.Bind(&req); err != nil {
		return avatarPresignRequest{}, false
	}
	if req.Filename == "" || req.ContentType == "" || req.Size <= 0 {
		return avatarPresignRequest{}, false
	}
	return req, true
}

func bindAvatarUpload(c echo.Context) (string, int64, io.Reader, error) {
	contentType := strings.TrimSpace(c.Request().Header.Get(echo.HeaderContentType))
	if contentType == "" {
		return "", 0, nil, account.ErrInvalidAvatarType
	}
	size := c.Request().ContentLength
	if size <= 0 || size > account.MaxAvatarBytes {
		return "", 0, nil, account.ErrInvalidAvatarSize
	}
	return contentType, size, io.LimitReader(c.Request().Body, size), nil
}

func bindAvatarConfirm(c echo.Context) (avatarConfirmRequest, bool) {
	var req avatarConfirmRequest
	if err := c.Bind(&req); err != nil || req.Key == "" {
		return avatarConfirmRequest{}, false
	}
	return req, true
}

func settingsError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, account.ErrInvalidUsername), errors.Is(err, account.ErrInvalidWorkspaceInput), errors.Is(err, account.ErrInvalidFolderName):
		return response.Fail(c, http.StatusBadRequest, 40002, "invalid input")
	case errors.Is(err, account.ErrInvalidAvatarType), errors.Is(err, account.ErrInvalidAvatarSize):
		return response.Fail(c, http.StatusBadRequest, 40003, "invalid avatar")
	case errors.Is(err, account.ErrCannotInviteSelf):
		return response.Fail(c, http.StatusBadRequest, 40004, "cannot invite yourself")
	case errors.Is(err, account.ErrUsernameConflict), errors.Is(err, account.ErrAlreadyMember), errors.Is(err, account.ErrFolderExists):
		return response.Fail(c, http.StatusConflict, 40901, "already exists")
	case errors.Is(err, account.ErrFolderNotFound):
		return response.Fail(c, http.StatusNotFound, 40404, "folder not found")
	case errors.Is(err, account.ErrWorkspaceForbidden):
		return response.Fail(c, http.StatusForbidden, 40304, "workspace owner required")
	case errors.Is(err, account.ErrUserNotFound):
		return response.Fail(c, http.StatusNotFound, 40402, "user not found")
	case errors.Is(err, account.ErrWorkspaceNotFound):
		return response.Fail(c, http.StatusNotFound, 40403, "workspace not found")
	case errors.Is(err, account.ErrAvatarNotFound):
		return response.Fail(c, http.StatusBadRequest, 40005, "avatar upload not found")
	case errors.Is(err, auth.ErrNoWorkspace):
		return response.Fail(c, http.StatusForbidden, 40302, "no workspace available")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50007, "settings request failed")
	}
}
