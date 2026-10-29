package notification

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type localBroadcaster interface {
	Broadcast(event Event)
}

type EventBus struct {
	client  *redis.Client
	channel string
	local   localBroadcaster
	ready   chan struct{}
}

func NewEventBus(client *redis.Client, prefix string, local localBroadcaster) *EventBus {
	return &EventBus{
		client:  client,
		channel: prefix + ":notification:events",
		local:   local,
		ready:   make(chan struct{}),
	}
}

func (b *EventBus) Ready() <-chan struct{} {
	return b.ready
}

func (b *EventBus) Publish(ctx context.Context, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal notification event: %w", err)
	}
	if err := b.client.Publish(ctx, b.channel, data).Err(); err != nil {
		if b.local != nil {
			b.local.Broadcast(event)
		}
		return fmt.Errorf("publish notification event: %w", err)
	}
	return nil
}

func (b *EventBus) Run(ctx context.Context) error {
	sub := b.client.Subscribe(ctx, b.channel)
	defer func() {
		_ = sub.Close()
	}()
	if _, err := sub.Receive(ctx); err != nil {
		return fmt.Errorf("subscribe notification events: %w", err)
	}
	close(b.ready)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-sub.Channel():
			if !ok {
				return nil
			}
			var event Event
			if err := json.Unmarshal([]byte(message.Payload), &event); err != nil {
				continue
			}
			if b.local != nil {
				b.local.Broadcast(event)
			}
		}
	}
}
