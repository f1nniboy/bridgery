package await

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAwait(t *testing.T) {
	ctx := context.Background()

	t.Run("reply during send is not lost", func(t *testing.T) {
		var w Map[string, int]
		got, err := w.Await(ctx, "a", time.Second, func() error {
			if !w.Resolve("a", 1) {
				t.Error("waiter not registered before send")
			}
			return nil
		})
		if err != nil || got != 1 {
			t.Errorf("got %d, %v", got, err)
		}
	})

	t.Run("send error is returned", func(t *testing.T) {
		var w Map[string, int]
		boom := errors.New("boom")
		if _, err := w.Await(ctx, "a", time.Second, func() error { return boom }); !errors.Is(err, boom) {
			t.Errorf("got %v", err)
		}
		if w.Resolve("a", 1) {
			t.Error("waiter left behind")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		var w Map[string, int]
		if _, err := w.Await(ctx, "a", 10*time.Millisecond, func() error { return nil }); !errors.Is(err, ErrTimeout) {
			t.Errorf("got %v", err)
		}
	})

	t.Run("cancel is not a timeout", func(t *testing.T) {
		var w Map[string, int]
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := w.Await(ctx, "a", time.Second, func() error { return nil }); !errors.Is(err, context.Canceled) {
			t.Errorf("got %v", err)
		}
	})

	t.Run("resolve without waiter", func(t *testing.T) {
		var w Map[string, int]
		if w.Resolve("a", 1) {
			t.Error("reported a waiter")
		}
	})
}
