package gateway

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/f1nniboy/bridgery/internal/core"
)

// channel membership of one account for one session
type membership struct {
	ctx    context.Context
	bridge core.Bridge
	log    *slog.Logger

	states map[string]*chanState
	retry  chan string
	failed chan joinFailure
	joins  sync.WaitGroup
}

type chanState struct {
	spec     core.ChannelSpec
	delay    time.Duration
	failedAt time.Time
	timer    *time.Timer
}

type joinFailure struct {
	name string
	err  error
}

func newMembership(ctx context.Context, bridge core.Bridge, log *slog.Logger) *membership {
	return &membership{
		ctx:    ctx,
		bridge: bridge,
		log:    log,
		states: make(map[string]*chanState),
		retry:  make(chan string),
		failed: make(chan joinFailure),
	}
}

func (m *membership) add(spec core.ChannelSpec) {
	m.states[spec.Name] = &chanState{spec: spec}
	m.join(spec.Name)
}

// call after the session ended, waits for joins in flight
func (m *membership) stop() {
	for _, s := range m.states {
		s.stopTimer()
	}
	m.joins.Wait()
}

func (m *membership) join(name string) {
	s := m.states[name]
	if s == nil {
		return
	}
	spec := s.spec
	m.joins.Go(func() {
		err := m.bridge.Join(m.ctx, spec)
		if err == nil {
			m.log.Debug("joined", "channel", spec.Name)
			return
		}
		select {
		case m.failed <- joinFailure{spec.Name, err}:
		case <-m.ctx.Done():
		}
	})
}

func (m *membership) down(name string, err error) {
	s := m.states[name]
	if s == nil { // not desired, or already given up
		return
	}
	s.stopTimer()
	if errors.Is(err, core.ErrPermanent) {
		delete(m.states, name)
		m.log.Error("channel down, giving up", "channel", name, "err", err)
		return
	}
	if time.Since(s.failedAt) > 2*maxBackoff {
		s.delay = 0
	}
	s.failedAt = time.Now()
	s.delay = nextBackoff(s.delay)
	m.log.Warn("channel down, retrying", "channel", name, "err", err, "in", s.delay)
	s.timer = time.AfterFunc(s.delay, func() {
		select {
		case m.retry <- name:
		case <-m.ctx.Done():
		}
	})
}

func (s *chanState) stopTimer() {
	if s.timer != nil {
		s.timer.Stop()
	}
}
