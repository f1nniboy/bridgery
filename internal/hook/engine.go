package hook

import (
	"context"
	"log/slog"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
	luar "layeh.com/gopher-luar"

	"github.com/f1nniboy/bridgery/internal/core"
)

const callTimeout = time.Second

const (
	phaseReceive = "receive"
	phaseDeliver = "deliver"
)

var phases = []string{phaseReceive, phaseDeliver}

type Config struct {
	Dir string `toml:"dir,commented" comment:"directory of Lua scripts, omit to disable"`
}

type Target struct {
	core.Address

	Platform string        `luar:"platform"`
	Parent   *core.Message `luar:"parent"`
	Caps     core.CapSet   `luar:"-"`
}

func (t *Target) Can(name string) bool { return t.Caps.Has(name) }

type handler struct {
	file  string
	phase string
	fn    *lua.LFunction
}

type Engine struct {
	dir string
	log *slog.Logger

	mu       sync.Mutex
	L        *lua.LState
	handlers []handler
}

func New(cfg Config, log *slog.Logger) (*Engine, error) {
	if cfg.Dir == "" {
		return nil, nil
	}
	e := &Engine{dir: cfg.Dir, log: log}
	if err := e.reload(); err != nil {
		return nil, err
	}
	e.log.Info("hooks loaded", "dir", cfg.Dir, "handlers", len(e.handlers))
	return e, nil
}

func (e *Engine) Receive(ev core.Event) (core.Event, bool) {
	kind := ev.Kind
	ev, ok := e.run(phaseReceive, ev, nil)
	ev.Kind = kind
	return ev, ok
}

func (e *Engine) Deliver(ev core.Event, t Target) (core.Event, bool) {
	return e.run(phaseDeliver, ev, &t)
}

func (e *Engine) run(phase string, ev core.Event, dst *Target) (core.Event, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, h := range e.handlers {
		if h.phase != phase {
			continue
		}
		var keep bool
		if ev, keep = e.call(h, ev, dst); !keep {
			return core.Event{}, false
		}
	}
	return ev, true
}

func (e *Engine) call(h handler, ev core.Event, dst *Target) (core.Event, bool) {
	view := ev
	view.Content = cloned(ev.Content)
	view.Files = cloned(ev.Files)

	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	e.L.SetContext(ctx)

	dstv := lua.LValue(lua.LNil)
	if dst != nil {
		dstv = luar.New(e.L, dst)
	}
	e.L.Push(h.fn)
	e.L.Push(luar.New(e.L, &view))
	e.L.Push(dstv)
	if err := e.L.PCall(2, 1, nil); err != nil {
		e.log.Warn("hook error, continuing", "file", h.file, "err", err)
		return ev, true
	}
	ret := e.L.Get(-1)
	e.L.Pop(1)
	if ret == lua.LFalse {
		return core.Event{}, false
	}
	return view, true
}

func cloned[T any](s []T) []T { return append(make([]T, 0, len(s)), s...) }
