package notification

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHubBroadcastToClient(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.Handle(conn)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	want := Event{Type: "notifications.changed", Action: "published", ID: 12}
	deadline := time.Now().Add(3 * time.Second)
	for {
		hub.Broadcast(want)
		_ = conn.SetReadDeadline(deadline)
		var got Event
		if err := conn.ReadJSON(&got); err == nil {
			if got != want {
				t.Fatalf("event = %+v, want %+v", got, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for broadcast")
		}
	}
}

func TestHubBroadcastDropsSlowClient(t *testing.T) {
	hub := NewHub()
	client := &hubClient{send: make(chan []byte, maxSendBacklog)}
	for i := 0; i < maxSendBacklog; i++ {
		client.send <- []byte("queued")
	}
	hub.clients[client] = struct{}{}

	hub.Broadcast(Event{Type: "notifications.changed", Action: "published", ID: 12})

	if len(hub.clients) != 0 {
		t.Fatalf("clients = %d, want slow client removed", len(hub.clients))
	}
}

func TestHubCloseDropsClient(t *testing.T) {
	hub := NewHub()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.Handle(conn)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	hub.Close()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected read error after hub close")
	}
}
