package notificationchannel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"vibebiz/internal/dao/model"
)

type Type string

const (
	TypeWebhook  Type = "webhook"
	TypeFeishu   Type = "feishu"
	TypeWeCom    Type = "wecom"
	TypeDingTalk Type = "dingtalk"
	TypeEmail    Type = "email"
)

const (
	TestStatusSuccess = "success"
	TestStatusFailed  = "failed"
)

var (
	ErrInvalid            = errors.New("invalid notification channel")
	ErrNotFound           = errors.New("notification channel not found")
	ErrNameConflict       = errors.New("notification channel name already exists")
	ErrCipherUnavailable  = errors.New("notification channel encryption unavailable")
	ErrSenderUnavailable  = errors.New("notification channel sender unavailable")
	ErrEmailNotConfigured = errors.New("smtp is not configured")
	ErrProviderRejected   = errors.New("notification provider rejected message")
)

type Config struct {
	URL    string `json:"url,omitempty"`
	Secret string `json:"secret,omitempty"`
	To     string `json:"to,omitempty"`
}

type Channel struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	Type           Type       `json:"type"`
	Config         Config     `json:"config"`
	Enabled        bool       `json:"enabled"`
	LastTestAt     *time.Time `json:"lastTestAt,omitempty"`
	LastTestStatus string     `json:"lastTestStatus"`
	LastTestError  string     `json:"lastTestError"`
}

type Input struct {
	Name    string
	Type    Type
	Config  Config
	Enabled bool
}

type Cipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(encoded string) (string, error)
}

type Repository interface {
	List(ctx context.Context, userID int64) ([]model.NotificationChannel, error)
	Get(ctx context.Context, userID, id int64) (*model.NotificationChannel, error)
	Create(ctx context.Context, item *model.NotificationChannel) error
	Update(ctx context.Context, item *model.NotificationChannel) error
	Delete(ctx context.Context, userID, id int64) error
	NameExists(ctx context.Context, userID int64, name string, excludeID int64) (bool, error)
	UpdateTest(ctx context.Context, userID, id int64, at time.Time, status, message string) error
}

type Sender interface {
	Send(ctx context.Context, channel Channel) error
}

type Service struct {
	Repo   Repository
	Cipher Cipher
	Sender Sender
	Now    func() time.Time
}

func (s *Service) List(ctx context.Context, userID int64) ([]Channel, error) {
	items, err := s.Repo.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]Channel, 0, len(items))
	for index := range items {
		channel, err := s.fromModel(&items[index])
		if err != nil {
			return nil, err
		}
		out = append(out, channel)
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, userID int64, input Input) (Channel, error) {
	if err := validateInput(input); err != nil {
		return Channel{}, err
	}
	exists, err := s.Repo.NameExists(ctx, userID, strings.TrimSpace(input.Name), 0)
	if err != nil {
		return Channel{}, err
	}
	if exists {
		return Channel{}, ErrNameConflict
	}
	cipherText, err := s.encrypt(input.Config)
	if err != nil {
		return Channel{}, err
	}
	item := &model.NotificationChannel{
		UserID: userID, Name: strings.TrimSpace(input.Name), Type: string(input.Type),
		ConfigCipher: cipherText, Enabled: input.Enabled,
	}
	if err := s.Repo.Create(ctx, item); err != nil {
		return Channel{}, err
	}
	return s.fromModel(item)
}

func (s *Service) Update(ctx context.Context, userID, id int64, input Input) (Channel, error) {
	if err := validateInput(input); err != nil {
		return Channel{}, err
	}
	item, err := s.Repo.Get(ctx, userID, id)
	if err != nil {
		return Channel{}, err
	}
	exists, err := s.Repo.NameExists(ctx, userID, strings.TrimSpace(input.Name), id)
	if err != nil {
		return Channel{}, err
	}
	if exists {
		return Channel{}, ErrNameConflict
	}
	cipherText, err := s.encrypt(input.Config)
	if err != nil {
		return Channel{}, err
	}
	item.Name = strings.TrimSpace(input.Name)
	item.Type = string(input.Type)
	item.ConfigCipher = cipherText
	item.Enabled = input.Enabled
	if err := s.Repo.Update(ctx, item); err != nil {
		return Channel{}, err
	}
	return s.fromModel(item)
}

func (s *Service) Delete(ctx context.Context, userID, id int64) error {
	return s.Repo.Delete(ctx, userID, id)
}

func (s *Service) Test(ctx context.Context, userID, id int64) (Channel, error) {
	item, err := s.Repo.Get(ctx, userID, id)
	if err != nil {
		return Channel{}, err
	}
	channel, err := s.fromModel(item)
	if err != nil {
		return Channel{}, err
	}
	if s.Sender == nil {
		return channel, ErrSenderUnavailable
	}
	now := s.now()
	sendErr := s.Sender.Send(ctx, channel)
	status := TestStatusSuccess
	message := ""
	if sendErr != nil {
		status = TestStatusFailed
		message = truncateError(sendErr.Error())
	}
	if err := s.Repo.UpdateTest(ctx, userID, id, now, status, message); err != nil {
		return channel, err
	}
	channel.LastTestAt = &now
	channel.LastTestStatus = status
	channel.LastTestError = message
	if sendErr != nil {
		return channel, sendErr
	}
	return channel, nil
}

func (s *Service) encrypt(config Config) (string, error) {
	if s.Cipher == nil {
		return "", ErrCipherUnavailable
	}
	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return s.Cipher.Encrypt(string(data))
}

func (s *Service) fromModel(item *model.NotificationChannel) (Channel, error) {
	if s.Cipher == nil {
		return Channel{}, ErrCipherUnavailable
	}
	plain, err := s.Cipher.Decrypt(item.ConfigCipher)
	if err != nil {
		return Channel{}, fmt.Errorf("decrypt notification channel: %w", err)
	}
	var config Config
	if err := json.Unmarshal([]byte(plain), &config); err != nil {
		return Channel{}, fmt.Errorf("decode notification channel: %w", err)
	}
	return Channel{
		ID: item.ID, Name: item.Name, Type: Type(item.Type), Config: config, Enabled: item.Enabled,
		LastTestAt: item.LastTestAt, LastTestStatus: item.LastTestStatus, LastTestError: item.LastTestError,
	}, nil
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func validateInput(input Input) error {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 64 {
		return ErrInvalid
	}
	if len([]rune(input.Config.Secret)) > 512 {
		return ErrInvalid
	}
	switch input.Type {
	case TypeWebhook:
		return validateWebhookURL(input.Config.URL, "")
	case TypeFeishu:
		return validateWebhookURL(input.Config.URL, "open.feishu.cn")
	case TypeWeCom:
		return validateWebhookURL(input.Config.URL, "qyapi.weixin.qq.com")
	case TypeDingTalk:
		return validateWebhookURL(input.Config.URL, "oapi.dingtalk.com")
	case TypeEmail:
		address := strings.TrimSpace(input.Config.To)
		if address == "" {
			return ErrInvalid
		}
		parsed, err := mail.ParseAddress(address)
		if err != nil || parsed.Address != address {
			return ErrInvalid
		}
		return nil
	default:
		return ErrInvalid
	}
}

func validateWebhookURL(raw, expectedHost string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ErrInvalid
	}
	if expectedHost != "" && !strings.EqualFold(parsed.Hostname(), expectedHost) {
		return ErrInvalid
	}
	return nil
}

func truncateError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 500 {
		return value[:500]
	}
	return value
}
