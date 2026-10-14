package ledger

import (
	"context"
	"time"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/logging"
)

type backend interface {
	core.Ledger
	config.Backend
	maxAge() time.Duration
	prune(ctx context.Context, before time.Time) error
}

type retention struct {
	MaxAge config.Duration `toml:"max_age,commented" comment:"forget messages this long after they were last updated, 0 to keep them forever"`
}

func (r retention) maxAge() time.Duration { return time.Duration(r.MaxAge) }

var Registry = map[string]func() backend{
	"memory": func() backend { return &memory{retention: retention{MaxAge: config.Duration(24 * time.Hour)}} },
	"sqlite": func() backend { return &sqlite{retention: retention{MaxAge: config.Duration(30 * 24 * time.Hour)}} },
}

func Open(cfg config.Backends) (core.Ledger, error) {
	b, err := config.OpenBackend(cfg, Registry, "memory")
	if err != nil {
		return nil, err
	}
	ctx, stop := context.WithCancel(context.Background())
	l := &ledger{backend: b, stop: stop}
	if age := b.maxAge(); age > 0 {
		go l.sweep(ctx, age)
	}
	return l, nil
}

type ledger struct {
	backend
	stop context.CancelFunc
}

func (l *ledger) Close() error {
	l.stop()
	return l.backend.Close()
}

func (l *ledger) sweep(ctx context.Context, maxAge time.Duration) {
	log := logging.New("ledger")
	interval := min(max(maxAge/10, time.Second), time.Hour)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			pctx, cancel := context.WithTimeout(ctx, interval)
			err := l.prune(pctx, now.Add(-maxAge))
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Warn("prune", "err", err)
			}
		}
	}
}
