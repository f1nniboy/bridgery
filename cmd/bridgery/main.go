package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/gateway"
	"github.com/f1nniboy/bridgery/internal/ledger"
	"github.com/f1nniboy/bridgery/internal/logging"
	"github.com/f1nniboy/bridgery/internal/media"
	"github.com/f1nniboy/bridgery/internal/meta"
	"github.com/f1nniboy/bridgery/internal/platforms"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bridgery:", err)
		os.Exit(1)
	}
}

func run() error {
	path := flag.String("config", "config.toml", "path to config file")
	version := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *version {
		fmt.Println(meta.Version)
		return nil
	}

	cfg, err := config.Load(*path)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logging.Init(cfg.Log)
	log := logging.New("gateway")

	store, err := ledger.Open(cfg.Ledger)
	if err != nil {
		return fmt.Errorf("open ledger: %w", err)
	}
	defer store.Close()

	med, err := media.Open(cfg.Media)
	if err != nil {
		return fmt.Errorf("open media: %w", err)
	}

	accounts := make(map[string]gateway.Account)
	for platform, labels := range cfg.Accounts {
		newBridge, ok := platforms.Registry[platform]
		if !ok {
			return fmt.Errorf("unsupported platform %q", platform)
		}
		for label, raw := range labels {
			id := platform + "." + label
			b := newBridge(core.NewCommon(logging.New("platform."+platform), med))
			if err := config.Decode(bytes.NewReader(raw), b); err != nil {
				return fmt.Errorf("account %q: %w", id, err)
			}
			accounts[id] = gateway.Account{Bridge: b, Platform: platform}
		}
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no accounts configured")
	}

	router, err := gateway.New(cfg, store, accounts, med, log)
	if err != nil {
		return fmt.Errorf("build gateway: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop) // a second signal kills immediately

	log.Info("starting", "accounts", len(accounts), "gateways", len(cfg.Gateways))
	if err := router.Run(ctx); err != nil {
		return fmt.Errorf("run gateway: %w", err)
	}
	return nil
}
