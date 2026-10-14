package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/hook"
	"github.com/f1nniboy/bridgery/internal/logging"
)

const drainTimeout = 5 * time.Second

type Account struct {
	Bridge   core.Bridge
	Platform string
}

type Router struct {
	accounts map[string]Account
	routes   map[core.Address][]core.Address
	channels map[string][]core.ChannelSpec // account -> channels to join
	queues   map[core.Address]chan core.Event

	ledger core.Ledger
	hooks  *hook.Engine // nil when no hooks are configured
	media  core.Media
	log    *slog.Logger

	timeout time.Duration
}

func New(cfg config.Config, ledger core.Ledger, accounts map[string]Account, media core.Media, log *slog.Logger) (*Router, error) {
	for _, gw := range cfg.Gateways {
		for _, c := range gw.Channels {
			if _, ok := accounts[c.Account]; !ok {
				return nil, fmt.Errorf("channel %s references unknown account %q", c.Address(), c.Account)
			}
		}
	}

	hooks, err := hook.New(cfg.Hooks, logging.New("hook"))
	if err != nil {
		return nil, fmt.Errorf("load hooks: %w", err)
	}

	routes := buildRoutes(cfg.Gateways)
	return &Router{
		accounts: accounts,
		routes:   routes,
		channels: buildChannels(cfg.Gateways),
		queues:   buildQueues(routes, cfg.Runtime.QueueSize),
		ledger:   ledger,
		hooks:    hooks,
		media:    media,
		log:      log,
		timeout:  time.Duration(cfg.Runtime.Timeout),
	}, nil
}

func (r *Router) Run(ctx context.Context) error {
	if r.hooks != nil {
		if err := r.hooks.Start(ctx); err != nil {
			return fmt.Errorf("watch hooks: %w", err)
		}
	}

	delivery, cancelDelivery := context.WithCancel(context.Background())
	defer cancelDelivery()
	drained := make(chan struct{})

	var workers, ingests, managers sync.WaitGroup
	for addr, queue := range r.queues {
		bridge := r.accounts[addr.Account].Bridge
		workers.Go(func() { r.worker(delivery, addr, bridge, queue) })
	}
	for name, acc := range r.accounts {
		if _, used := r.channels[name]; !used {
			r.log.Warn("account not used by any gateway, not connecting", "account", name)
			continue
		}
		ingests.Go(func() { r.ingest(ctx, name, acc.Bridge.Inbound()) })
		managers.Go(func() { r.manage(ctx, name, acc.Bridge.Status(), drained) })
	}

	<-ctx.Done()
	r.log.Info("shutting down, draining queues")
	ingests.Wait()
	for _, q := range r.queues {
		close(q)
	}
	deadline := time.AfterFunc(drainTimeout, cancelDelivery)
	workers.Wait()
	deadline.Stop()

	close(drained)
	managers.Wait()
	r.log.Info("shutdown complete")
	return nil
}
