package scheduledtask

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	"vibebiz/internal/client"
	daomodel "vibebiz/internal/dao/model"
	notifymodel "vibebiz/internal/dao/model"
	"vibebiz/internal/notification"
)

type HistoryClient interface {
	ReadChatHistory(ctx context.Context, id string, limit int) (client.ChatHistoryTail, error)
}

type ChatClient interface {
	Chat(ctx context.Context, id string, payload []byte) (io.ReadCloser, error)
}

type Notifier interface {
	Create(ctx context.Context, input notification.CreateInput, creator notification.Creator) (notification.View, error)
	Publish(ctx context.Context, id int64) (notification.View, error)
}

type Runner struct {
	Repo     Repository
	Chat     ChatClient
	History  HistoryClient
	Flags    Flags
	Notifier Notifier
	WebBase  string
	Logger   *slog.Logger
	Now      func() time.Time
}

func NewRunner(repo Repository, chat ChatClient, history HistoryClient, flags Flags, notifier Notifier, webBase string, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{
		Repo: repo, Chat: chat, History: history, Flags: flags, Notifier: notifier,
		WebBase: strings.TrimRight(webBase, "/"), Logger: logger, Now: time.Now,
	}
}

type ExecutionResult struct {
	Status string
	Error  string
}

func (r *Runner) HandleTask(ctx context.Context, task *asynq.Task) error {
	var payload struct {
		TaskID int64 `json:"taskId"`
	}
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	_, err := r.Execute(ctx, payload.TaskID)
	return err
}

func (r *Runner) Execute(ctx context.Context, taskID int64) (ExecutionResult, error) {
	item, err := r.Repo.Get(ctx, taskID)
	if err != nil {
		return ExecutionResult{}, err
	}
	if item.Enabled != 1 {
		return ExecutionResult{Status: StatusStopped}, nil
	}
	acquired, err := r.Flags.Acquire(ctx, taskID, 15*time.Minute)
	if err != nil {
		return ExecutionResult{}, err
	}
	if !acquired {
		return ExecutionResult{Status: StatusRunning}, nil
	}
	defer r.Flags.Release(ctx, taskID)

	_ = r.Repo.Update(ctx, taskID, map[string]any{"last_status": StatusRunning})
	topic, err := r.Repo.TopicByCode(ctx, TopicCodeScheduled)
	if err != nil {
		return r.fail(ctx, item, err)
	}
	projectID := projectIDOf(item)
	if projectID == "" {
		return r.fail(ctx, item, errors.New("fixed project is missing"))
	}
	history, err := r.History.ReadChatHistory(ctx, projectID, 5)
	if err != nil {
		return r.fail(ctx, item, err)
	}
	payload, _ := json.Marshal(map[string]string{
		"message":      item.Prompt,
		"systemPrompt": topic.PromptTemplate,
		"context":      formatHistory(history.Content),
	})
	stream, err := r.Chat.Chat(ctx, projectID, payload)
	if err != nil {
		return r.fail(ctx, item, err)
	}
	defer stream.Close()
	if err := consumeSSE(stream); err != nil {
		return r.fail(ctx, item, err)
	}

	status := StatusSuccess
	if r.Flags.Stopped(ctx, taskID) {
		status = StatusStopped
		r.Flags.ClearStop(ctx, taskID)
	}
	_ = r.Repo.Update(ctx, taskID, map[string]any{"last_status": status, "last_error": ""})
	if status == StatusSuccess {
		r.notify(ctx, item, status, "")
	}
	return ExecutionResult{Status: status}, nil
}

func (r *Runner) fail(ctx context.Context, item *daomodel.ScheduledTask, cause error) (ExecutionResult, error) {
	status := StatusFailed
	if r.Flags.Stopped(ctx, item.ID) {
		status = StatusStopped
		r.Flags.ClearStop(ctx, item.ID)
	}
	fields := map[string]any{"last_status": status, "last_error": cause.Error()}
	var apiErr *client.APIError
	if errors.As(cause, &apiErr) && apiErr.Status == http.StatusNotFound {
		fields["enabled"] = 0
		fields["next_run_at"] = nil
	}
	_ = r.Repo.Update(ctx, item.ID, fields)
	if status == StatusFailed && r.shouldNotifyFailure(ctx) {
		r.notify(ctx, item, status, cause.Error())
	}
	return ExecutionResult{Status: status, Error: cause.Error()}, cause
}

func (r *Runner) shouldNotifyFailure(ctx context.Context) bool {
	retry, ok := asynq.GetRetryCount(ctx)
	if !ok {
		return true
	}
	maxRetry, ok := asynq.GetMaxRetry(ctx)
	if !ok {
		return true
	}
	return retry >= maxRetry
}

func (r *Runner) notify(ctx context.Context, item *daomodel.ScheduledTask, status, detail string) {
	if r.Notifier == nil {
		return
	}
	title := "定时任务执行成功"
	if status == StatusFailed {
		title = "定时任务执行失败"
	}
	content := "任务「" + item.Name + "」" + status
	if detail != "" {
		content += "：" + detail
	}
	link := ""
	if projectID := projectIDOf(item); projectID != "" && r.WebBase != "" {
		link = r.WebBase + "/editor/" + url.PathEscape(projectID)
	}
	view, err := r.Notifier.Create(ctx, notification.CreateInput{
		Title: title, Content: content, Type: notifymodel.NotificationInbox,
		Link: link, RecipientUserID: item.UserID, ExpireAt: r.Now().Add(7 * 24 * time.Hour),
	}, notification.Creator{ID: 0, Name: "system"})
	if err != nil {
		return
	}
	_, _ = r.Notifier.Publish(ctx, view.ID)
}

func formatHistory(content string) string {
	lines := strings.Split(content, "\n")
	parts := make([]string, 0, 5)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil || entry.Type != "message" {
			continue
		}
		role := "用户"
		if entry.Message.Role == "assistant" {
			role = "助手"
		}
		text := strings.TrimSpace(entry.Message.Content)
		if len([]rune(text)) > 1500 {
			runes := []rune(text)
			text = string(runes[:1500]) + "..."
		}
		parts = append(parts, role+"："+text)
	}
	if len(parts) > 5 {
		parts = parts[len(parts)-5:]
	}
	out := strings.Join(parts, "\n")
	if len([]rune(out)) > 6000 {
		runes := []rune(out)
		out = string(runes[:6000]) + "..."
	}
	return out
}

func consumeSSE(stream io.Reader) error {
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if payload == "[DONE]" {
				return nil
			}
			var event struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if json.Unmarshal([]byte(payload), &event) == nil && event.Type == "error" {
				if event.Message == "" {
					event.Message = "agent failed"
				}
				return errors.New(event.Message)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}
