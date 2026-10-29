package notification

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestEventBusDeliversToOtherInstance(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	hub := &fakeHub{}
	bus := NewEventBus(client, "vibebiz", hub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go bus.Run(ctx)
	select {
	case <-bus.Ready():
	case <-time.After(time.Second):
		t.Fatal("event bus did not subscribe")
	}

	publisher := NewEventBus(client, "vibebiz", nil)
	want := Event{Type: "notifications.changed", Action: "published", ID: 42}
	if err := publisher.Publish(ctx, want); err != nil {
		t.Fatalf("publish: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(hub.events) > 0 {
			if hub.last != want {
				t.Fatalf("event = %+v, want %+v", hub.last, want)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("event not delivered")
}

func TestEventBusPublishFallsBackLocally(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	hub := &fakeHub{}
	bus := NewEventBus(client, "vibebiz", hub)

	want := Event{Type: "notifications.changed", Action: "deleted", ID: 7}
	if err := bus.Publish(context.Background(), want); err == nil {
		t.Fatal("publish err = nil, want connection error")
	}
	if len(hub.events) != 1 || hub.last != want {
		t.Fatalf("local events = %+v, want %+v", hub.events, want)
	}
}
