package auth

import "github.com/labstack/echo/v4"

const sessionContextKey = "auth.session"

func SessionFrom(c echo.Context) (*Session, bool) {
	session, ok := c.Get(sessionContextKey).(*Session)
	return session, ok
}

func SetSession(c echo.Context, session *Session) {
	c.Set(sessionContextKey, session)
}
