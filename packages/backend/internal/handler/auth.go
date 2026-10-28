package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/account"
	"vibebiz/internal/auth"
	"vibebiz/internal/response"
)

type AuthHandler struct {
	Service    *auth.Service
	Account    *account.Service
	UserCookie auth.CookieManager
	OpsCookie  auth.CookieManager
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type switchWorkspaceRequest struct {
	WorkspaceID int64 `json:"workspaceId"`
}

type authOpsUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	RoleCode string `json:"roleCode"`
}

func (h *AuthHandler) LoginUser(c echo.Context) error {
	req, ok := bindLogin(c)
	if !ok {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	session, err := h.Service.LoginUser(c.Request().Context(), req.Identifier, req.Password, c.RealIP())
	if err != nil {
		return loginError(c, err)
	}
	if err := h.UserCookie.Set(c, auth.CookieIdentity{SID: session.ID, Username: session.Username, Email: session.Email}); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50005, "set session cookie failed")
	}
	return h.meResult(c, session)
}

func (h *AuthHandler) MeUser(c echo.Context) error {
	session, err := h.current(c, auth.RealmUser, h.UserCookie)
	if err != nil {
		return currentError(c, err)
	}
	return h.meResult(c, session)
}

func (h *AuthHandler) SwitchWorkspace(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req switchWorkspaceRequest
	if err := c.Bind(&req); err != nil || req.WorkspaceID <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	updated, err := h.Service.SwitchWorkspace(c.Request().Context(), session.ID, req.WorkspaceID)
	if err != nil {
		if errors.Is(err, auth.ErrWorkspaceNotAccessible) {
			return response.Fail(c, http.StatusForbidden, 40303, "workspace not accessible")
		}
		return currentError(c, err)
	}
	return h.meResult(c, updated)
}

func (h *AuthHandler) LogoutUser(c echo.Context) error {
	if err := h.logout(c, auth.RealmUser, h.UserCookie); err != nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "session store unavailable")
	}
	return response.OK(c, map[string]any{"loggedOut": true})
}

func (h *AuthHandler) LoginOps(c echo.Context) error {
	req, ok := bindLogin(c)
	if !ok {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	session, err := h.Service.LoginOps(c.Request().Context(), req.Identifier, req.Password, c.RealIP())
	if err != nil {
		return loginError(c, err)
	}
	if err := h.OpsCookie.Set(c, auth.CookieIdentity{SID: session.ID, Username: session.Username, Email: session.Email}); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50005, "set session cookie failed")
	}
	return response.OK(c, authOpsUser{ID: session.UserID, Username: session.Username, Email: session.Email, RoleCode: session.Role})
}

func (h *AuthHandler) MeOps(c echo.Context) error {
	session, err := h.current(c, auth.RealmOps, h.OpsCookie)
	if err != nil {
		return currentError(c, err)
	}
	return response.OK(c, authOpsUser{ID: session.UserID, Username: session.Username, Email: session.Email, RoleCode: session.Role})
}

func (h *AuthHandler) LogoutOps(c echo.Context) error {
	if err := h.logout(c, auth.RealmOps, h.OpsCookie); err != nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "session store unavailable")
	}
	return response.OK(c, map[string]any{"loggedOut": true})
}

func (h *AuthHandler) current(c echo.Context, realm auth.Realm, cookies auth.CookieManager) (*auth.Session, error) {
	identity, err := cookies.Read(c)
	if err != nil {
		return nil, auth.ErrSessionNotFound
	}
	return h.Service.Current(c.Request().Context(), realm, identity.SID)
}

func (h *AuthHandler) logout(c echo.Context, realm auth.Realm, cookies auth.CookieManager) error {
	identity, err := cookies.Read(c)
	if err == nil {
		if err := h.Service.Logout(c.Request().Context(), realm, identity.SID); err != nil {
			cookies.Clear(c)
			return err
		}
	}
	cookies.Clear(c)
	return nil
}

func bindLogin(c echo.Context) (loginRequest, bool) {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return loginRequest{}, false
	}
	req.Identifier = strings.TrimSpace(req.Identifier)
	if req.Identifier == "" || req.Password == "" {
		return loginRequest{}, false
	}
	return req, true
}

func loginError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return response.Fail(c, http.StatusUnauthorized, 40102, "invalid credentials")
	case errors.Is(err, auth.ErrAccountDisabled):
		return response.Fail(c, http.StatusForbidden, 40301, "account disabled")
	case errors.Is(err, auth.ErrNoWorkspace):
		return response.Fail(c, http.StatusForbidden, 40302, "no workspace available")
	default:
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "session store unavailable")
	}
}

func currentError(c echo.Context, err error) error {
	if errors.Is(err, auth.ErrSessionNotFound) {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	return response.Fail(c, http.StatusServiceUnavailable, 50301, "session store unavailable")
}

func (h *AuthHandler) meResult(c echo.Context, session *auth.Session) error {
	if h.Account != nil {
		me, err := h.Account.Me(c.Request().Context(), session)
		if err != nil {
			return response.Fail(c, http.StatusInternalServerError, 50006, "load account failed")
		}
		return response.OK(c, me)
	}
	workspaces := make([]account.WorkspaceSummary, 0, len(session.WorkspaceIDs))
	for _, id := range session.WorkspaceIDs {
		workspaces = append(workspaces, account.WorkspaceSummary{ID: id})
	}
	return response.OK(c, account.Me{
		ID:                 session.UserID,
		Username:           session.Username,
		Email:              session.Email,
		CurrentWorkspaceID: session.CurrentWorkspaceID,
		Workspaces:         workspaces,
	})
}
