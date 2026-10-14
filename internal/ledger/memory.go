package ledger

import (
	"container/list"
	"slices"
	"sync"

	"github.com/f1nniboy/bridgery/internal/core"
)

type nativeKey struct {
	addr   core.Address
	native string
}

const defaultMaxSize = 10000

type memory struct {
	MaxSize int `toml:"max_size,commented" comment:"max cached messages, 0 for unbounded"`

	mu sync.RWMutex

	order *list.List // core.CID, front = oldest
	elems map[core.CID]*list.Element

	messages map[core.CID]core.Message
	bindings map[core.CID]map[core.Address][]string
	reverse  map[nativeKey]core.CID
}

func (m *memory) Open() error {
	m.order = list.New()
	m.elems = make(map[core.CID]*list.Element)
	m.messages = make(map[core.CID]core.Message)
	m.bindings = make(map[core.CID]map[core.Address][]string)
	m.reverse = make(map[nativeKey]core.CID)
	return nil
}

func (m *memory) Example() any {
	return &memory{MaxSize: defaultMaxSize}
}

func (m *memory) track(id core.CID) {
	if m.MaxSize <= 0 {
		return
	}
	if e, ok := m.elems[id]; ok {
		m.order.MoveToBack(e) // touched again, keep it from aging out by first-insert
		return
	}
	m.elems[id] = m.order.PushBack(id)
	if m.order.Len() > m.MaxSize {
		m.clean()
	}
}

func (m *memory) clean() {
	front := m.order.Front()
	id := front.Value.(core.CID)

	m.order.Remove(front)
	delete(m.elems, id)
	delete(m.messages, id)

	for addr, natives := range m.bindings[id] {
		for _, n := range natives {
			delete(m.reverse, nativeKey{addr, n})
		}
	}
	delete(m.bindings, id)
}

func (m *memory) Put(id core.CID, r core.Message) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.track(id)
	m.messages[id] = r
}

func (m *memory) Get(id core.CID) (core.Message, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.messages[id]
	return r, ok
}

func (m *memory) Delete(id core.CID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.messages, id) // content only, bindings stay for delete propagation and get cleaned up by clean
}

func (m *memory) Bind(id core.CID, addr core.Address, natives []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.track(id)

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
}

func (m *memory) Canonical(addr core.Address, nativeID string) (core.CID, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.reverse[nativeKey{addr, nativeID}]
	return id, ok
}

func (m *memory) Native(id core.CID, dst core.Address) ([]string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	natives, ok := m.bindings[id][dst]
	if !ok {
		return nil, false
	}
	return slices.Clone(natives), true
}

func (m *memory) Close() error { return nil }
