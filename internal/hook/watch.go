package hook

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	lua "github.com/yuin/gopher-lua"
)

func (e *Engine) reload() error {
	entries, err := os.ReadDir(e.dir)
	if err != nil {
		return err
	}
	var names []string
	for _, en := range entries {
		if !en.IsDir() && strings.HasSuffix(en.Name(), ".lua") {
			names = append(names, en.Name())
		}
	}
	slices.Sort(names)

	L := lua.NewState()
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	L.SetContext(ctx)
	e.registerUtils(L)
	var (
		handlers []handler
		current  string
	)
	L.SetGlobal("on", L.NewFunction(func(L *lua.LState) int {
		handlers = append(handlers, handler{current, L.CheckString(1), L.CheckFunction(2)})
		return 0
	}))
	for _, name := range names {
		current = name
		if err := L.DoFile(filepath.Join(e.dir, name)); err != nil {
			L.Close()
			return errors.New(strings.TrimSpace(err.Error()))
		}
	}
	for _, h := range handlers {
		if !slices.Contains(phases, h.phase) {
			L.Close()
			return fmt.Errorf("%s: unknown phase %q", h.file, h.phase)
		}
	}

	e.mu.Lock()
	old := e.L
	e.L, e.handlers = L, handlers
	e.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

func (e *Engine) Start(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := w.Add(e.dir); err != nil {
		w.Close()
		return err
	}
	e.log.Info("watching hooks", "dir", e.dir)
	go e.watch(ctx, w)
	return nil
}

func (e *Engine) watch(ctx context.Context, w *fsnotify.Watcher) {
	defer w.Close()
	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-w.Events:
			if strings.HasSuffix(ev.Name, ".lua") {
				debounce = time.After(150 * time.Millisecond)
			}
		case err := <-w.Errors:
			e.log.Warn("hook watch error", "err", err)
		case <-debounce:
			debounce = nil
			if err := e.reload(); err != nil {
				e.log.Warn("hook reload failed, keeping previous", "err", err)
				continue
			}
			e.mu.Lock()
			n := len(e.handlers)
			e.mu.Unlock()
			e.log.Info("hooks reloaded", "handlers", n)
		}
	}
}
