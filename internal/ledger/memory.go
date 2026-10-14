package ledger

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/f1nniboy/bridgery/internal/core"
)

type nativeKey struct {
	addr   core.Address
	native string
}

type memory struct {
	retention

	mu       sync.RWMutex
	updated  map[core.CID]time.Time
	messages map[core.CID]core.Message
	bindings map[core.CID]map[core.Address][]string
	reverse  map[nativeKey]core.CID
}

func (m *memory) Open() error {
	m.updated = make(map[core.CID]time.Time)
	m.messages = make(map[core.CID]core.Message)
	m.bindings = make(map[core.CID]map[core.Address][]string)
	m.reverse = make(map[nativeKey]core.CID)
	return nil
}

func (m *memory) Example() any {
	return m
}

func (m *memory) prune(_ context.Context, before time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, t := range m.updated {
		if t.Before(before) {
			m.forget(id)
		}
	}
	return nil
}

func (m *memory) forget(id core.CID) {
	for addr, natives := range m.bindings[id] {
		for _, n := range natives {
			delete(m.reverse, nativeKey{addr, n})
		}
	}
	delete(m.bindings, id)
	delete(m.messages, id)
	delete(m.updated, id)
}

func (m *memory) Put(_ context.Context, id core.CID, msg core.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updated[id] = time.Now()
	m.messages[id] = msg.WithoutData()
	return nil
}

func (m *memory) Get(_ context.Context, id core.CID) (core.Message, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msg, ok := m.messages[id]
	return msg, ok, nil
}

func (m *memory) Delete(_ context.Context, id core.CID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.messages, id) // content only, bindings stay
	return nil
}

func (m *memory) Bind(_ context.Context, id core.CID, addr core.Address, natives []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updated[id] = time.Now()

	byAddr := m.bindings[id]
	if byAddr == nil {
		byAddr = make(map[core.Address][]string)
		m.bindings[id] = byAddr
	}
	for _, n := range byAddr[addr] {
		delete(m.reverse, nativeKey{addr, n})
	}
	byAddr[addr] = slices.Clone(natives)
	for _, n := range natives {
		m.reverse[nativeKey{addr, n}] = id
	}
	return nil
}

func (m *memory) Canonical(_ context.Context, addr core.Address, nativeID string) (core.CID, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.reverse[nativeKey{addr, nativeID}]
	return id, ok, nil
}

func (m *memory) Native(_ context.Context, id core.CID, dst core.Address) ([]string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	natives, ok := m.bindings[id][dst]
	if !ok {
		return nil, false, nil
	}
	return slices.Clone(natives), true, nil
}

func (m *memory) Close() error {
	return nil
}
