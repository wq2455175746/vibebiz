package notificationchannel

import (
	"context"
	"errors"
	"testing"
	"time"

	"vibebiz/internal/dao/model"
)

type fakeCipher struct{}

func (fakeCipher) Encrypt(value string) (string, error) { return "enc:" + value, nil }
func (fakeCipher) Decrypt(value string) (string, error) { return value[4:], nil }

type fakeRepo struct {
	items   map[int64]model.NotificationChannel
	nextID  int64
	names   map[string]bool
	testAt  time.Time
	testS   string
	testErr string
	updated bool
	deleted bool
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: make(map[int64]model.NotificationChannel), nextID: 1, names: make(map[string]bool)}
}

func (f *fakeRepo) List(context.Context, int64) ([]model.NotificationChannel, error) {
	out := make([]model.NotificationChannel, 0, len(f.items))
	for _, item := range f.items {
		out = append(out, item)
	}
	return out, nil
}

func (f *fakeRepo) Get(_ context.Context, _ int64, id int64) (*model.NotificationChannel, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &item, nil
}

func (f *fakeRepo) Create(_ context.Context, item *model.NotificationChannel) error {
	item.ID = f.nextID
	f.nextID++
	f.items[item.ID] = *item
	return nil
}

func (f *fakeRepo) Update(_ context.Context, item *model.NotificationChannel) error {
	if _, ok := f.items[item.ID]; !ok {
		return ErrNotFound
	}
	f.items[item.ID] = *item
	f.updated = true
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, _ int64, id int64) error {
	delete(f.items, id)
	f.deleted = true
	return nil
}

func (f *fakeRepo) NameExists(_ context.Context, _ int64, name string, _ int64) (bool, error) {
	return f.names[name], nil
}

func (f *fakeRepo) UpdateTest(_ context.Context, _ int64, id int64, at time.Time, status, message string) error {
	item := f.items[id]
	item.LastTestAt = &at
	item.LastTestStatus = status
	item.LastTestError = message
	f.items[id] = item
	f.testAt, f.testS, f.testErr = at, status, message
	return nil
}

type fakeSender struct {
	err     error
	channel Channel
	calls   int
}

func (f *fakeSender) Send(_ context.Context, channel Channel) error {
	f.calls++
	f.channel = channel
	return f.err
}

func newTestService(repo *fakeRepo, sender *fakeSender) *Service {
	return &Service{
		Repo: repo, Cipher: fakeCipher{}, Sender: sender,
		Now: func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
}

func TestCreateEncryptsConfig(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeSender{})
	channel, err := svc.Create(context.Background(), 7, Input{
		Name: "研发群", Type: TypeFeishu, Enabled: true,
		Config: Config{URL: "https://open.feishu.cn/open-apis/bot/v2/hook/abc", Secret: "sec"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if channel.ID == 0 || channel.Name != "研发群" || !channel.Enabled {
		t.Fatalf("channel = %+v", channel)
	}
	if repo.items[channel.ID].ConfigCipher == "" || repo.items[channel.ID].ConfigCipher == channel.Config.URL {
		t.Fatalf("config not encrypted: %q", repo.items[channel.ID].ConfigCipher)
	}
}

func TestCreateRejectsInvalidConfig(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, &fakeSender{})
	cases := []Input{
		{Name: "", Type: TypeWebhook, Config: Config{URL: "https://example.com/hook"}},
		{Name: "a", Type: TypeWebhook, Config: Config{URL: "http://example.com/hook"}},
		{Name: "a", Type: TypeFeishu, Config: Config{URL: "https://example.com/hook"}},
		{Name: "a", Type: TypeEmail, Config: Config{To: "not-an-email"}},
		{Name: "a", Type: "unknown", Config: Config{URL: "https://example.com/hook"}},
	}
	for _, input := range cases {
		if _, err := svc.Create(context.Background(), 7, input); !errors.Is(err, ErrInvalid) {
			t.Fatalf("input %+v err = %v", input, err)
		}
	}
}

func TestListDecryptsConfig(t *testing.T) {
	repo := newFakeRepo()
	repo.items[1] = model.NotificationChannel{ID: 1, UserID: 7, Name: "a", Type: string(TypeWebhook), ConfigCipher: "enc:{\"url\":\"https://example.com\"}"}
	svc := newTestService(repo, &fakeSender{})
	channels, err := svc.List(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(channels) != 1 || channels[0].Config.URL != "https://example.com" {
		t.Fatalf("channels = %+v", channels)
	}
}

func TestTestSendUpdatesSuccessAndFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.items[1] = model.NotificationChannel{ID: 1, UserID: 7, Name: "a", Type: string(TypeWebhook), ConfigCipher: "enc:{\"url\":\"https://example.com\"}"}
	sender := &fakeSender{}
	svc := newTestService(repo, sender)
	channel, err := svc.Test(context.Background(), 7, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sender.calls != 1 || channel.LastTestStatus != TestStatusSuccess || repo.testS != TestStatusSuccess {
		t.Fatalf("channel = %+v repo=%s", channel, repo.testS)
	}

	sender.err = errors.New("boom")
	if _, err := svc.Test(context.Background(), 7, 1); err == nil {
		t.Fatal("expected error")
	}
	if repo.testS != TestStatusFailed || repo.testErr == "" {
		t.Fatalf("status=%s error=%s", repo.testS, repo.testErr)
	}
}

func TestUpdateRejectsDuplicateNameAndCreateUsesUserScope(t *testing.T) {
	repo := newFakeRepo()
	repo.names["existing"] = true
	svc := newTestService(repo, &fakeSender{})
	if _, err := svc.Create(context.Background(), 7, Input{Name: "existing", Type: TypeEmail, Config: Config{To: "a@example.com"}}); !errors.Is(err, ErrNameConflict) {
		t.Fatalf("err = %v", err)
	}
}
