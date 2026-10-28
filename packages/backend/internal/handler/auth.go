package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/response"
)

type AuthHandler struct {
	Service    *auth.Service
	UserCookie auth.CookieManager
	OpsCookie  auth.CookieManager
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type authUser struct {
	ID           int64   `json:"id"`
	Username     string  `json:"username"`
	Email        string  `json:"email"`
	Nickname     string  `json:"nickname"`
	WorkspaceIDs []int64 `json:"workspaceIds"`
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
	return response.OK(c, userResult(session))
}

func (h *AuthHandler) MeUser(c echo.Context) error {
	session, err := h.current(c, auth.RealmUser, h.UserCookie)
	if err != nil {
		return currentError(c, err)
	}
	return response.OK(c, userResult(session))
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

func userResult(session *auth.Session) authUser {
	return authUser{
		ID:           session.UserID,
		Username:     session.Username,
		Email:        session.Email,
		Nickname:     "",
		WorkspaceIDs: session.WorkspaceIDs,
	}
}
