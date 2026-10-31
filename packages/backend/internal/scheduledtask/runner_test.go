package scheduledtask

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"vibebiz/internal/client"
	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/notification"
)

type fakeHistory struct {
	tail client.ChatHistoryTail
	err  error
}

func (f *fakeHistory) ReadChatHistory(context.Context, string, int) (client.ChatHistoryTail, error) {
	return f.tail, f.err
}

type fakeChat struct {
	payload []byte
	body    string
	err     error
}

func (f *fakeChat) Chat(_ context.Context, _ string, payload []byte) (io.ReadCloser, error) {
	f.payload = payload
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.body)), nil
}

type fakeFlags struct {
	stopped  bool
	acquired bool
}

func (f *fakeFlags) Acquire(context.Context, int64, time.Duration) (bool, error) {
	return f.acquired, nil
}
func (f *fakeFlags) Release(context.Context, int64) {}
func (f *fakeFlags) MarkStop(context.Context, int64, time.Duration) error {
	return nil
}
func (f *fakeFlags) Stopped(context.Context, int64) bool { return f.stopped }
func (f *fakeFlags) ClearStop(context.Context, int64)    { f.stopped = false }

type fakeNotifier struct {
	created   int
	published int
}

func (f *fakeNotifier) Create(context.Context, notification.CreateInput, notification.Creator) (notification.View, error) {
	f.created++
	return notification.View{ID: 11}, nil
}
func (f *fakeNotifier) Publish(context.Context, int64) (notification.View, error) {
	f.published++
	return notification.View{ID: 11}, nil
}

func TestRunnerBuildsPromptAndNotifies(t *testing.T) {
	now := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	projectID := int64(11)
	repo := &fakeRepo{
		items: []daomodel.ScheduledTask{{
			Base:   daomodel.Base{ID: 3, ExtraP: []byte(`{"projectPublicId":"p1"}`)},
			UserID: 7, ProjectID: &projectID, Name: "检查", Prompt: "检查依赖",
			Enabled: 1, CronExpr: "0 9 * * *", Timezone: "Asia/Shanghai",
			LastStatus: StatusRunning,
		}},
		topics: []daomodel.ProjectTopic{{
			Base: daomodel.Base{ID: 9}, TopicCode: TopicCodeScheduled, PromptTemplate: "系统提示",
		}},
	}
	history := &fakeHistory{tail: client.ChatHistoryTail{Content: `{"type":"session"}` + "\n" + `{"type":"message","message":{"role":"user","content":"你好"}}` + "\n" + `{"type":"message","message":{"role":"assistant","content":"收到"}}` + "\n"}}
	chat := &fakeChat{body: "data: {\"type\":\"agent\",\"payload\":{\"type\":\"result\"}}\n\ndata: [DONE]\n\n"}
	flags := &fakeFlags{acquired: true}
	notifier := &fakeNotifier{}
	runner := NewRunner(repo, chat, history, flags, notifier, "http://vibebiz-dev.com:5173", slog.Default())
	runner.Now = func() time.Time { return now }
	out, err := runner.Execute(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != StatusSuccess {
		t.Fatalf("status = %s", out.Status)
	}
	var payload map[string]string
	if err := json.Unmarshal(chat.payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["message"] != "检查依赖" || payload["systemPrompt"] != "系统提示" || !strings.Contains(payload["context"], "用户：你好") {
		t.Fatalf("payload = %+v", payload)
	}
	if notifier.created != 1 || notifier.published != 1 {
		t.Fatalf("notifications = %+v", notifier)
	}
}
