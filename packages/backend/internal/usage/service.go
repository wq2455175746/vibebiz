package usage

import (
	"context"
	"fmt"
	"time"
)

type Quota struct {
	ProjectDailyQuota     int64
	LLMTokenDailyQuota    int64
	UploadDailyCountQuota int64
	UploadDailyBytesQuota int64
}

type Usage struct {
	ProjectsCreated int64
	InputTokens     int64
	OutputTokens    int64
	ModelRequests   int64
	UploadCount     int64
	UploadBytes     int64
}

type Filter struct {
	From   time.Time
	To     time.Time
	UserID int64
}

type DailyRow struct {
	UserID          int64  `json:"userId"`
	Date            string `json:"date"`
	ProjectsCreated int64  `json:"projectsCreated"`
	InputTokens     int64  `json:"inputTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	ModelRequests   int64  `json:"modelRequests"`
	UploadCount     int64  `json:"uploadCount"`
	UploadBytes     int64  `json:"uploadBytes"`
}

type QuotaExceededError struct {
	QuotaType string
	Used      int64
	Limit     int64
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("%s quota exceeded: %d/%d", e.QuotaType, e.Used, e.Limit)
}

type Service struct {
	Repo Repository
	Now  func() time.Time
	Loc  *time.Location
}

func NewService(repo Repository, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{Repo: repo, Now: time.Now, Loc: loc}
}

func (s *Service) Today() time.Time {
	now := s.Now().In(s.Loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.Loc)
}

func (s *Service) ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, s.Loc)
}

func (s *Service) CheckProjectCreate(ctx context.Context, userID int64) error {
	quota, err := s.Repo.Quota(ctx, userID)
	if err != nil {
		return err
	}
	if quota.ProjectDailyQuota <= 0 {
		return nil
	}
	used, err := s.Repo.Usage(ctx, userID, s.Today())
	if err != nil {
		return err
	}
	if used.ProjectsCreated >= quota.ProjectDailyQuota {
		return &QuotaExceededError{QuotaType: "project", Used: used.ProjectsCreated, Limit: quota.ProjectDailyQuota}
	}
	return nil
}

func (s *Service) CheckChat(ctx context.Context, userID int64) error {
	quota, err := s.Repo.Quota(ctx, userID)
	if err != nil {
		return err
	}
	if quota.LLMTokenDailyQuota <= 0 {
		return nil
	}
	used, err := s.Repo.Usage(ctx, userID, s.Today())
	if err != nil {
		return err
	}
	total := used.InputTokens + used.OutputTokens
	if total >= quota.LLMTokenDailyQuota {
		return &QuotaExceededError{QuotaType: "token", Used: total, Limit: quota.LLMTokenDailyQuota}
	}
	return nil
}

func (s *Service) CheckUpload(ctx context.Context, userID, addCount, addBytes int64) error {
	quota, err := s.Repo.Quota(ctx, userID)
	if err != nil {
		return err
	}
	if quota.UploadDailyCountQuota <= 0 && quota.UploadDailyBytesQuota <= 0 {
		return nil
	}
	used, err := s.Repo.Usage(ctx, userID, s.Today())
	if err != nil {
		return err
	}
	if quota.UploadDailyCountQuota > 0 && used.UploadCount+addCount > quota.UploadDailyCountQuota {
		return &QuotaExceededError{QuotaType: "upload_count", Used: used.UploadCount, Limit: quota.UploadDailyCountQuota}
	}
	if quota.UploadDailyBytesQuota > 0 && used.UploadBytes+addBytes > quota.UploadDailyBytesQuota {
		return &QuotaExceededError{QuotaType: "upload_bytes", Used: used.UploadBytes, Limit: quota.UploadDailyBytesQuota}
	}
	return nil
}

func (s *Service) RecordProjectCreate(ctx context.Context, userID int64) error {
	return s.Repo.Increment(ctx, userID, s.Today(), Usage{ProjectsCreated: 1})
}

func (s *Service) RecordChatUsage(ctx context.Context, userID, inputTokens, outputTokens, modelRequests int64) error {
	if inputTokens == 0 && outputTokens == 0 && modelRequests == 0 {
		return nil
	}
	return s.Repo.Increment(ctx, userID, s.Today(), Usage{
		InputTokens: inputTokens, OutputTokens: outputTokens, ModelRequests: modelRequests,
	})
}

func (s *Service) RecordUpload(ctx context.Context, userID, count, bytes int64) error {
	if count == 0 && bytes == 0 {
		return nil
	}
	return s.Repo.Increment(ctx, userID, s.Today(), Usage{UploadCount: count, UploadBytes: bytes})
}

func (s *Service) Series(ctx context.Context, filter Filter) ([]DailyRow, error) {
	return s.Repo.Series(ctx, filter)
}
