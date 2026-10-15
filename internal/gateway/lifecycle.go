package gateway

import (
	"context"
	"errors"
	"time"

	"github.com/f1nniboy/bridgery/internal/core"
)

const (
	minBackoff = 10 * time.Second
	maxBackoff = 15 * time.Minute
)

func (r *Router) manage(ctx context.Context, account string, ch <-chan core.Status, drained <-chan struct{}) {
	acc := r.accounts[account]
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

		session, end := context.WithCancel(ctx)
		if err := acc.open(session); err != nil {
			end()
			if errors.Is(err, core.ErrPermanent) {
				r.log.Error("connect failed, giving up", "account", account, "err", err)
				return
			}
			delay = nextBackoff(delay)
			r.log.Warn("connect failed, retrying", "account", account, "err", err, "in", delay)
			continue
		}
		r.log.Info("connected", "account", account)

		m := newMembership(session, acc.Bridge, r.log.With("account", account))
		up := r.supervise(session, m, account, ch)
		end()
		m.stop()
		if !up {
			<-drained
			r.close(account)
			return
		}
		r.close(account)
		for len(ch) > 0 {
			<-ch
		}
		delay = minBackoff
		r.log.Warn("disconnected, reconnecting", "account", account, "in", delay)
	}
}

func (r *Router) supervise(ctx context.Context, m *membership, account string, ch <-chan core.Status) bool {
	for _, spec := range r.channels[account] {
		m.add(spec)
	}
	for {
		select {
		case <-ctx.Done():
			return false
		case name := <-m.retry:
			m.join(name)
		case f := <-m.failed:
			m.down(f.name, f.err)
		case st := <-ch:
			switch st.Event {
			case core.ChannelLost:
				m.down(st.Channel, st.Err)
			case core.Disconnected:
				return true
			}
		}
	}
}

func (r *Router) close(account string) {
	if err := r.accounts[account].close(); err != nil {
		r.log.Warn("close bridge", "account", account, "err", err)
	}
}

func nextBackoff(d time.Duration) time.Duration {
	if d < minBackoff {
		return minBackoff
	}
	return min(2*d, maxBackoff)
}
