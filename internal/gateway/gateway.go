package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/hook"
	"github.com/f1nniboy/bridgery/internal/logging"
)

type Account struct {
	Bridge   core.Bridge
	Platform string
}

type Gateway struct {
	accounts map[string]Account
	routes   map[core.Address][]core.Address
	channels map[string][]core.ChannelSpec // account -> channels to join

	ledger core.Ledger
	hooks  *hook.Engine // nil when no hooks are configured
	media  core.Media
	log    *slog.Logger

	timeout   time.Duration
	queueSize int
	queues    map[core.Address]chan core.Event // per destination, built in Run
}

func New(cfg config.Config, ledger core.Ledger, accounts map[string]Account, media core.Media, log *slog.Logger) (*Gateway, error) {
	for _, gw := range cfg.Gateways {
		for _, c := range gw.Channels {
			if _, ok := accounts[c.Account]; !ok {
				addr := core.Address{Account: c.Account, Channel: c.Channel}
				return nil, fmt.Errorf("channel %s references unknown account %q", addr, c.Account)
			}
		}
	}

	hooks, err := hook.New(cfg.Hooks, logging.New("hook"))
	if err != nil {
		return nil, fmt.Errorf("load hooks: %w", err)
	}

	return &Gateway{
		ledger:    ledger,
		accounts:  accounts,
		routes:    buildRoutes(cfg.Gateways),
		channels:  buildChannels(cfg.Gateways),
		hooks:     hooks,
		media:     media,
		log:       log,
		timeout:   time.Duration(cfg.Runtime.Timeout),
		queueSize: cfg.Runtime.QueueSize,
	}, nil
}

func buildRoutes(defs []config.Gateway) map[core.Address][]core.Address {
	routes := make(map[core.Address][]core.Address)
	seen := make(map[core.Address]map[core.Address]bool) // from -> dest set, dedups across gateways
	for _, gw := range defs {
		addrs := make([]core.Address, len(gw.Channels))
		for i, c := range gw.Channels {
			addrs[i] = core.Address{Account: c.Account, Channel: c.Channel}
		}
		for i := range gw.Channels {
			if !gw.Channels[i].Sources() {
				continue
			}
			from := addrs[i]
			for j, dst := range gw.Channels {
				// skip self (by index and by address, in case a channel is listed twice)
				// and destinations that don't receive
				if j == i || addrs[j] == from || !dst.Receives() {
					continue
				}
				if seen[from] == nil {
					seen[from] = make(map[core.Address]bool)
				}
				if seen[from][addrs[j]] {
					continue
				}
				seen[from][addrs[j]] = true
				routes[from] = append(routes[from], addrs[j])
			}
		}
	}
	return routes
}

func (g *Gateway) Run(ctx context.Context) error {
	g.queues = g.buildQueues()

	if g.hooks != nil {
		if err := g.hooks.Start(ctx); err != nil {
			return fmt.Errorf("watch hooks: %w", err)
		}
	}

	var workers sync.WaitGroup
	for addr, queue := range g.queues {
		bridge := g.accounts[addr.Account].Bridge
		workers.Go(func() { g.worker(addr, bridge, queue) })
	}
	var ingests, managers sync.WaitGroup
	for name, acc := range g.accounts {
		if _, used := g.channels[name]; !used {
			g.log.Warn("account not used by any gateway, not connecting", "account", name)
			continue
		}
		ingests.Go(func() { g.ingest(ctx, name, acc.Bridge.Inbound()) })
		managers.Go(func() { g.manage(ctx, name, acc.Bridge.Status()) })
	}

	<-ctx.Done()
	g.shutdown(&ingests, &workers, &managers)
	return nil
}

func (g *Gateway) buildQueues() map[core.Address]chan core.Event {
	queues := make(map[core.Address]chan core.Event)
	for _, dests := range g.routes {
		for _, dst := range dests {
			if _, ok := queues[dst]; ok {
				continue
			}
			queues[dst] = make(chan core.Event, g.queueSize)
		}
	}
	return queues
}

func (g *Gateway) shutdown(ingests, workers, managers *sync.WaitGroup) {
	g.log.Info("shutting down")
	managers.Wait()
	g.log.Info("bridges closed, draining inbound")
	ingests.Wait()

	for _, q := range g.queues {
		close(q)
	}
	g.log.Info("inbound drained, draining delivery queues")
	workers.Wait()
	g.log.Info("shutdown complete")
}

func (g *Gateway) ingest(ctx context.Context, account string, inbound <-chan core.Inbound) {
	for {
		select {
		case <-ctx.Done():
			// drain what's already buffered so shutdown doesn't discard it
			for {
				select {
				case in := <-inbound:
					g.handle(in, account)
				default:
					return
				}
			}
		case in := <-inbound:
			g.handle(in, account)
		}
	}
}

func (g *Gateway) handle(in core.Inbound, account string) {
	in.Origin.Account = account
	ev, ok := g.lift(in)
	if !ok {
		return
	}
	if g.hooks != nil {
		if ev, ok = g.hooks.Receive(ev); !ok {
			return // cancelled by hook
		}
	}
	if ev.Kind == core.KindPost || ev.Kind == core.KindEdit {
		g.ledger.Put(ev.ID, stripData(ev.Message))
	}
	for _, dst := range g.routes[ev.Origin] {
		q, ok := g.queues[dst]
		if !ok {
			continue
		}
		select {
		case q <- ev:
		default:
			g.log.Warn("queue full, dropping event", "dst", dst, "kind", ev.Kind)
		}
	}
}

func stripData(m core.Message) core.Message {
	m.Author.Avatar.Data = nil
	if len(m.Files) > 0 {
		files := slices.Clone(m.Files)
		for i := range files {
			files[i].Data = nil
		}
		m.Files = files
	}
	return m
}

func (g *Gateway) worker(dst core.Address, b core.Bridge, q <-chan core.Event) {
	for ev := range q {
		caps := b.Caps() // re-run every time as caps may change (e.g. IRC)
		ctx, cancel := context.WithTimeout(context.Background(), g.timeout)

		ev = g.resolveFiles(ctx, ev, dst, caps)
		d, ok := g.lower(ev, dst, caps)
		if !ok {
			cancel()
			continue
		}

		r, err := b.Deliver(ctx, d)
		cancel()
		if err != nil {
			g.log.Warn("deliver", "dst", dst, "err", err)
			continue
		}

		if len(r.IDs) > 0 {
			if d.Fallback {
				existing, _ := g.ledger.Native(ev.ID, dst)
				g.ledger.Bind(ev.ID, dst, append(existing, r.IDs...))
			} else {
				g.ledger.Bind(ev.ID, dst, r.IDs)
			}
		}
	}
}

func (g *Gateway) resolveFiles(ctx context.Context, ev core.Event, dst core.Address, caps core.CapSet) core.Event {
	// platforms with CapMedia use .Bytes() so they won't ever need
	// the media proxy
	if caps.Has(core.CapMedia) || len(ev.Files) == 0 {
		return ev
	}

	out := make([]core.File, 0, len(ev.Files))
	for _, f := range ev.Files {
		url, err := g.media.URL(ctx, f)
		if err != nil {
			g.log.Warn("media resolve", "dst", dst, "file", f.Name, "err", err)
			continue
		}
		if url == "" {
			g.log.Warn("media resolve: empty url, dropping file", "dst", dst, "file", f.Name)
			continue
		}
		f.URL, f.Public, f.Data, f.Key = url, true, nil, ""
		out = append(out, f)
	}

	ev.Files = out
	return ev
}
