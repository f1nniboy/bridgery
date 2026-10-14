package main

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/hook"
	"github.com/f1nniboy/bridgery/internal/ledger"
	"github.com/f1nniboy/bridgery/internal/logging"
	"github.com/f1nniboy/bridgery/internal/media"
	"github.com/f1nniboy/bridgery/internal/platforms"
)

type genConfig struct {
	config.Config
	Ledger   map[string]any            `toml:"ledger,commented"`
	Media    map[string]any            `toml:"media,commented"`
	Accounts map[string]map[string]any `toml:"accounts"`
}

func main() {
	base := config.Default()
	base.Log = logging.Config{Level: "info", Format: "text"}
	base.Hooks = hook.Config{Dir: "hooks"}
	base.Gateways = []config.Gateway{{Channels: []config.Channel{
		{Account: "discord.main", Channel: "123456789012345678"},
		{Account: "irc.libera", Channel: "#mychannel"},
	}}}

	cfg := genConfig{
		Config: base,
		Ledger: map[string]any{"memory": config.ExampleBackend(ledger.Registry, "memory"), "sqlite": config.ExampleBackend(ledger.Registry, "sqlite")},
		Media:  map[string]any{"local": config.ExampleBackend(media.Registry, "local"), "s3": config.ExampleBackend(media.Registry, "s3")},
		Accounts: map[string]map[string]any{
			"discord": {"main": platforms.Example("discord")},
			"irc":     {"libera": platforms.Example("irc")},
		},
	}

	b, err := toml.Marshal(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := os.WriteFile("config.example.toml", b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
