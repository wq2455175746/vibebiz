package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

var ErrNoCookie = errors.New("auth cookie not found")

type CookieIdentity struct {
	SID      string `json:"sid"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type CookieManager struct {
	Name   string
	Secure bool
	TTL    time.Duration
	Now    func() time.Time
}

func (m CookieManager) Set(c echo.Context, id CookieIdentity) error {
	encoded, err := encodeIdentity(id)
	if err != nil {
		return err
	}
	http.SetCookie(c.Response(), m.cookie(encoded, int(m.TTL.Seconds()), nowFunc(m.Now)().Add(m.TTL)))
	return nil
}

func (m CookieManager) Read(c echo.Context) (CookieIdentity, error) {
	cookie, err := c.Cookie(m.Name)
	if err != nil {
		return CookieIdentity{}, ErrNoCookie
	}
	payload, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return CookieIdentity{}, fmt.Errorf("decode auth cookie: %w", err)
	}
	var id CookieIdentity
	if err := json.Unmarshal(payload, &id); err != nil {
		return CookieIdentity{}, fmt.Errorf("unmarshal auth cookie: %w", err)
	}
	if id.SID == "" {
		return CookieIdentity{}, ErrNoCookie
	}
	return id, nil
}

func (m CookieManager) Clear(c echo.Context) {
	http.SetCookie(c.Response(), m.cookie("", -1, time.Unix(1, 0)))
}

func (m CookieManager) cookie(value string, maxAge int, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     m.Name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  expires,
	}
}

func nowFunc(now func() time.Time) func() time.Time {
	if now != nil {
		return now
	}
	return time.Now
}

func encodeIdentity(id CookieIdentity) (string, error) {
	payload, err := json.Marshal(id)
	if err != nil {
		return "", fmt.Errorf("marshal auth cookie: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}
