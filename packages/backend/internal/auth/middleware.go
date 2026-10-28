package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/response"
)

type Authenticator struct {
	Service    *Service
	UserCookie CookieManager
	OpsCookie  CookieManager
}

func NewAuthenticator(service *Service, userCookie, opsCookie CookieManager) *Authenticator {
	return &Authenticator{Service: service, UserCookie: userCookie, OpsCookie: opsCookie}
}

func (a *Authenticator) RequireUser(next echo.HandlerFunc) echo.HandlerFunc {
	return a.require(next, RealmUser, a.UserCookie)
}

func (a *Authenticator) RequireOps(next echo.HandlerFunc) echo.HandlerFunc {
	return a.require(next, RealmOps, a.OpsCookie)
}

func (a *Authenticator) require(next echo.HandlerFunc, realm Realm, cookies CookieManager) echo.HandlerFunc {
	return func(c echo.Context) error {
		identity, err := cookies.Read(c)
		if err != nil {
			return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
		}
		session, err := a.Service.Current(c.Request().Context(), realm, identity.SID)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
			}
			return response.Fail(c, http.StatusServiceUnavailable, 50301, "session store unavailable")
		}
		if session.Realm != realm {
			return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
		}
		c.Set(sessionContextKey, session)
		return next(c)
	}
}
