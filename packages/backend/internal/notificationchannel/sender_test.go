package notificationchannel

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type fakeMailer struct {
	to      string
	subject string
	body    string
	err     error
}

func (f *fakeMailer) Send(_ context.Context, to, subject, body string) error {
	f.to, f.subject, f.body = to, subject, body
	return f.err
}

func testSender(server *httptest.Server, mailer Mailer) *TestSender {
	return &TestSender{
		HTTP:     server.Client(),
		Mailer:   mailer,
		Now:      func() time.Time { return time.UnixMilli(1700000000000).UTC() },
		LookupIP: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("8.8.8.8")}, nil },
	}
}

func TestWebhookSenderPayload(t *testing.T) {
	var body string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		_, _ = w.Write([]byte(`{"code":1}`))
	}))
	defer server.Close()
	sender := testSender(server, nil)
	err := sender.Send(context.Background(), Channel{Type: TypeWebhook, Config: Config{URL: server.URL + "/hook"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "vibebiz.channel.test") || !strings.Contains(body, "通知渠道测试") {
		t.Fatalf("body = %s", body)
	}
}

func TestFeishuSenderPayloadAndSignature(t *testing.T) {
	var rawQuery, body string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		_, _ = w.Write([]byte(`{"code":0}`))
	}))
	defer server.Close()
	sender := testSender(server, nil)
	err := sender.Send(context.Background(), Channel{Type: TypeFeishu, Config: Config{URL: server.URL + "/open-apis/bot/v2/hook/x", Secret: "sec"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"msg_type":"text"`) || !strings.Contains(body, "通知渠道测试") {
		t.Fatalf("body = %s", body)
	}
	values, _ := url.ParseQuery(rawQuery)
	if values.Get("timestamp") == "" || values.Get("sign") == "" {
		t.Fatalf("query = %s", rawQuery)
	}
}

func TestWeComAndDingTalkPayloads(t *testing.T) {
	var lastBody string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		lastBody = string(data)
		_, _ = w.Write([]byte(`{"errcode":0}`))
	}))
	defer server.Close()
	sender := testSender(server, nil)
	for _, typ := range []Type{TypeWeCom, TypeDingTalk} {
		if err := sender.Send(context.Background(), Channel{Type: typ, Config: Config{URL: server.URL + "/hook/" + string(typ)}}); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if !strings.Contains(lastBody, `"msgtype":"text"`) {
			t.Fatalf("%s body = %s", typ, lastBody)
		}
	}
}

func TestSenderRejectsProviderErrorAndPrivateURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":40001}`))
	}))
	defer server.Close()
	sender := testSender(server, nil)
	if err := sender.Send(context.Background(), Channel{Type: TypeWeCom, Config: Config{URL: server.URL + "/hook"}}); !errors.Is(err, ErrProviderRejected) {
		t.Fatalf("err = %v", err)
	}

	private := &TestSender{
		HTTP:     server.Client(),
		LookupIP: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("127.0.0.1")}, nil },
	}
	if err := private.Send(context.Background(), Channel{Type: TypeWebhook, Config: Config{URL: server.URL + "/hook"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("private err = %v", err)
	}
}

func TestSenderDoesNotLeakWebhookURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	raw := server.URL + "/hook/secret-token"
	server.Close()
	sender := testSender(server, nil)
	err := sender.Send(context.Background(), Channel{Type: TypeWebhook, Config: Config{URL: raw}})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked URL: %v", err)
	}
}

func TestEmailSenderUsesMailer(t *testing.T) {
	mailer := &fakeMailer{}
	sender := &TestSender{Mailer: mailer}
	channel := Channel{Type: TypeEmail, Config: Config{To: "a@example.com"}}
	if err := sender.Send(context.Background(), channel); err != nil {
		t.Fatal(err)
	}
	if mailer.to != "a@example.com" || !strings.Contains(mailer.subject, "Vibebiz") {
		t.Fatalf("mailer = %+v", mailer)
	}

	if err := (&TestSender{}).Send(context.Background(), channel); !errors.Is(err, ErrEmailNotConfigured) {
		t.Fatalf("err = %v", err)
	}
}
