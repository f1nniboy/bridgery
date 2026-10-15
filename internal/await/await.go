package await

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrTimeout = errors.New("timed out waiting for reply")

// replies to requests, matched by key
type Map[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]chan V
}

// registers key, calls send, then waits up to timeout for its reply
func (w *Map[K, V]) Await(ctx context.Context, key K, timeout time.Duration, send func() error) (V, error) {
	var zero V
	ch := make(chan V, 1)
	w.mu.Lock()
	if w.m == nil {
		w.m = make(map[K]chan V)
	}
	w.m[key] = ch
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		if w.m[key] == ch {
			delete(w.m, key)
		}
		w.mu.Unlock()
	}()

	if err := send(); err != nil {
		return zero, err
	}
	t := time.NewTimer(timeout)
	defer t.Stop()
	select {
	case v := <-ch:
		return v, nil
	case <-t.C:
		return zero, ErrTimeout
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

// delivers v to the waiter for key, reports whether there was one
func (w *Map[K, V]) Resolve(key K, v V) bool {
	w.mu.Lock()
	ch, ok := w.m[key]
	w.mu.Unlock()
	if ok {
		select {
		case ch <- v:
		default:
		}
	}
	return ok
}
