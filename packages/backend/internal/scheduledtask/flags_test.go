package scheduledtask

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisFlags(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	flags := NewRedisFlags(client, "vibebiz")
	ctx := context.Background()

	ok, err := flags.Acquire(ctx, 9, time.Minute)
	if err != nil || !ok {
		t.Fatalf("acquire ok=%v err=%v", ok, err)
	}
	if second, _ := flags.Acquire(ctx, 9, time.Minute); second {
		t.Fatal("second acquire should fail")
	}
	flags.Release(ctx, 9)
	if ok, _ := flags.Acquire(ctx, 9, time.Minute); !ok {
		t.Fatal("acquire after release should succeed")
	}

	if err := flags.MarkStop(ctx, 9, time.Minute); err != nil {
		t.Fatal(err)
	}
	if !flags.Stopped(ctx, 9) {
		t.Fatal("stop flag should be set")
	}
	flags.ClearStop(ctx, 9)
	if flags.Stopped(ctx, 9) {
		t.Fatal("stop flag should be cleared")
	}
}
