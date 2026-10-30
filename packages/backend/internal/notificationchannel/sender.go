package notificationchannel

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const testMessage = "Vibebiz 通知渠道测试"

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type TestSender struct {
	HTTP     *http.Client
	Mailer   Mailer
	Now      func() time.Time
	LookupIP func(host string) ([]net.IP, error)
}

func (s *TestSender) Send(ctx context.Context, channel Channel) error {
	if channel.Type == TypeEmail {
		if s.Mailer == nil {
			return ErrEmailNotConfigured
		}
		return s.Mailer.Send(ctx, channel.Config.To, "Vibebiz 通知渠道测试", testMessage)
	}
	if _, err := s.safeURL(channel.Config.URL); err != nil {
		return err
	}
	body, target, err := s.request(channel)
	if err != nil {
		return err
	}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(data))
	if err != nil {
		return ErrInvalid
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client().Do(req)
	if err != nil {
		return fmt.Errorf("%w: request failed", ErrProviderRejected)
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status %d", ErrProviderRejected, resp.StatusCode)
	}
	if channel.Type != TypeWebhook {
		if err := providerError(responseBody); err != nil {
			return err
		}
	}
	return nil
}

func (s *TestSender) request(channel Channel) (map[string]any, string, error) {
	timestamp := strconv.FormatInt(s.now().UnixMilli(), 10)
	switch channel.Type {
	case TypeWebhook:
		return map[string]any{"event": "vibebiz.channel.test", "message": testMessage, "timestamp": timestamp}, channel.Config.URL, nil
	case TypeFeishu:
		target, err := signedFeishuURL(channel.Config.URL, channel.Config.Secret, timestamp)
		if err != nil {
			return nil, "", err
		}
		return map[string]any{"msg_type": "text", "content": map[string]string{"text": testMessage}}, target, nil
	case TypeWeCom:
		return map[string]any{"msgtype": "text", "text": map[string]string{"content": testMessage}}, channel.Config.URL, nil
	case TypeDingTalk:
		target, err := signedDingTalkURL(channel.Config.URL, channel.Config.Secret, timestamp)
		if err != nil {
			return nil, "", err
		}
		return map[string]any{"msgtype": "text", "text": map[string]string{"content": testMessage}}, target, nil
	default:
		return nil, "", ErrInvalid
	}
}

func signedFeishuURL(raw, secret, timestamp string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalid
	}
	if secret == "" {
		return parsed.String(), nil
	}
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	_, _ = mac.Write(nil)
	query := parsed.Query()
	query.Set("timestamp", timestamp)
	query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func signedDingTalkURL(raw, secret, timestamp string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ErrInvalid
	}
	if secret == "" {
		return parsed.String(), nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "\n" + secret))
	query := parsed.Query()
	query.Set("timestamp", timestamp)
	query.Set("sign", base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (s *TestSender) safeURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
		return nil, ErrInvalid
	}
	lookup := s.LookupIP
	if lookup == nil {
		lookup = func(host string) ([]net.IP, error) {
			return net.LookupIP(host)
		}
	}
	ips, err := lookup(parsed.Hostname())
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("%w: cannot resolve host", ErrInvalid)
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, fmt.Errorf("%w: private address", ErrInvalid)
		}
	}
	return parsed, nil
}

func (s *TestSender) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (s *TestSender) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func providerError(data []byte) error {
	var payload struct {
		Code       *int `json:"code"`
		StatusCode *int `json:"StatusCode"`
		ErrCode    *int `json:"errcode"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	for _, code := range []*int{payload.Code, payload.StatusCode, payload.ErrCode} {
		if code != nil && *code != 0 {
			return fmt.Errorf("%w: code %d", ErrProviderRejected, *code)
		}
	}
	return nil
}
