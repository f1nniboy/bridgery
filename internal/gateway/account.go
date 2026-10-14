package gateway

import (
	"context"
	"sync"

	"github.com/f1nniboy/bridgery/internal/core"
)

type Account struct {
	Bridge   core.Bridge
	Platform string

	mu sync.RWMutex // deliveries hold it for reading, close for writing
	up bool
}

func (a *Account) open(ctx context.Context) error {
	if err := a.Bridge.Open(ctx); err != nil {
		return err
	}
	a.mu.Lock()
	a.up = true
	a.mu.Unlock()
	return nil
}

func (a *Account) acquire() bool {
	a.mu.RLock()
	if !a.up {
		a.mu.RUnlock()
		return false
	}
	return true
}

func (a *Account) release() { a.mu.RUnlock() }

func (a *Account) close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.up = false
	return a.Bridge.Close()
}
