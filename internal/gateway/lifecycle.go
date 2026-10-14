package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
)

const (
	minBackoff = 10 * time.Second
	maxBackoff = 15 * time.Minute
)

func buildChannels(defs []config.Gateway) map[string][]core.ChannelSpec {
	channels := make(map[string][]core.ChannelSpec)
	seen := make(map[string]map[string]bool) // account -> channel set
	for _, gw := range defs {
		for _, c := range gw.Channels {
			if seen[c.Account] == nil {
				seen[c.Account] = make(map[string]bool)
			}
			if seen[c.Account][c.Channel] {
				continue
			}
			seen[c.Account][c.Channel] = true
			channels[c.Account] = append(channels[c.Account], core.ChannelSpec{Name: c.Channel, Key: c.Key})
		}
	}
	return channels
}

func (g *Gateway) manage(ctx context.Context, account string, ch <-chan core.Status) {
	b := g.accounts[account].Bridge
	var delay time.Duration

	for {
		if delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
		if ctx.Err() != nil {
			return
		}

		if err := b.Open(ctx); err != nil {
			if errors.Is(err, core.ErrPermanent) {
				g.log.Error("connect failed, giving up", "account", account, "err", err)
				return
			}
			delay = nextBackoff(delay)
			g.log.Warn("connect failed, retrying", "account", account, "err", err, "in", delay)
			continue
		}
		g.log.Info("connected", "account", account)

		reconnect := g.supervise(ctx, account, ch)
		if err := b.Close(); err != nil {
			g.log.Warn("close bridge", "account", account, "err", err)
		}
		if !reconnect {
			return
		}
		delay = minBackoff
		g.log.Warn("disconnected, reconnecting", "account", account, "in", delay)
	}
}

func nextBackoff(d time.Duration) time.Duration {
	if d < minBackoff {
		return minBackoff
	}
	return min(2*d, maxBackoff)
}

type chanRetry struct {
	spec  core.ChannelSpec
	delay time.Duration
	timer *time.Timer
}

func (s *chanRetry) stop() {
	if s.timer != nil {
		s.timer.Stop()
	}
}

func (g *Gateway) supervise(ctx context.Context, account string, ch <-chan core.Status) bool {
	b := g.accounts[account].Bridge
	states := map[string]*chanRetry{}
	retry := make(chan string, len(g.channels[account])+1)
	defer func() {
		for _, s := range states {
			s.stop()
		}
	}()

	down := func(name string, err error) {
		s := states[name]
		if s == nil { // not desired, or already given up
			return
		}
		s.stop()
		if errors.Is(err, core.ErrPermanent) {
			delete(states, name)
			g.log.Error("channel down, giving up", "account", account, "channel", name, "err", err)
			return
		}
		s.delay = nextBackoff(s.delay)
		g.log.Warn("channel down, retrying", "account", account, "channel", name, "err", err, "in", s.delay)
		s.timer = time.AfterFunc(s.delay, func() {
			select {
			case retry <- name:
			default:
			}
		})
	}
	join := func(s *chanRetry) {
		if err := b.Join(s.spec); err != nil {
			down(s.spec.Name, err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return false
		case name := <-retry:
			if s := states[name]; s != nil {
				join(s)
			}
		case m := <-ch:
			switch m.Event {
			case core.Ready:
				for _, s := range states {
					s.stop()
				}
				for _, spec := range g.channels[account] {
					s := &chanRetry{spec: spec}
					states[spec.Name] = s
					join(s)
				}
			case core.ChannelUp:
				if s := states[m.Channel]; s != nil {
					s.stop()
					s.delay = 0 // successful, so reset backoff
				}
			case core.ChannelDown:
				down(m.Channel, m.Err)
			case core.Disconnected:
				return true
			}
		}
	}
}
