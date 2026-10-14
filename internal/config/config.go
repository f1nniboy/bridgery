package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/hook"
	"github.com/f1nniboy/bridgery/internal/logging"
)

type Config struct {
	Log      logging.Config                            `toml:"log"`
	Ledger   Backends                                  `toml:"ledger"`
	Runtime  Runtime                                   `toml:"runtime"`
	Hooks    hook.Config                               `toml:"hooks"`
	Media    Backends                                  `toml:"media"`
	Accounts map[string]map[string]unstable.RawMessage `toml:"accounts"` // platform -> label -> settings
	Gateways []Gateway                                 `toml:"gateways"`
}

type Runtime struct {
	QueueSize int      `toml:"queue_size,commented" comment:"per-destination max queued messages\nyou probably won't have to change this"`
	Timeout   Duration `toml:"timeout,commented" comment:"per-delivery timeout"`
}

type Gateway struct {
	Channels []Channel `toml:"channels" comment:"channels relayed to each other"`
}

type Channel struct {
	Account string `toml:"account"`
	Channel string `toml:"channel"`
	Mode    string `toml:"mode,commented" comment:"in: write-only, messages are never relayed here\nout: read-only, messages sent here are never relayed\ninout (default): normal"`

	Key Secret `toml:"key,commented" comment:"bridge-specific join secret (e.g. IRC)"`
}

func (c Channel) Address() core.Address {
	return core.Address{Account: c.Account, Channel: c.Channel}
}

func (c Channel) Sources() bool  { return c.Mode != "out" }
func (c Channel) Receives() bool { return c.Mode != "in" }

type Duration time.Duration

func (d *Duration) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func Default() Config {
	return Config{Runtime: Runtime{QueueSize: 256, Timeout: Duration(30 * time.Second)}}
}

func Load(path string) (Config, error) {
	cfg := Default()

	f, err := os.Open(path)
	if err != nil {
		return cfg, err
	}
	defer f.Close()

	if err := Decode(f, &cfg); err != nil {
		return cfg, err
	}
	return cfg, cfg.Validate()
}

func Decode(r io.Reader, v any) error {
	err := toml.NewDecoder(r).EnableUnmarshalerInterface().DisallowUnknownFields().Decode(v)
	var strict *toml.StrictMissingError
	if !errors.As(err, &strict) {
		return err
	}
	keys := make([]string, len(strict.Errors))
	for i, e := range strict.Errors {
		keys[i] = strings.Join(e.Key(), ".")
	}
	return fmt.Errorf("unknown field %s", strings.Join(keys, ", "))
}

func (c Config) Validate() error {
	if c.Runtime.QueueSize <= 0 {
		return errors.New("config: runtime.queue_size must be greater than 0")
	}
	if c.Runtime.Timeout <= 0 {
		return errors.New("config: runtime.timeout must be greater than 0")
	}
	for _, gw := range c.Gateways {
		for _, ch := range gw.Channels {
			switch ch.Mode {
			case "", "in", "out", "inout":
			default:
				return fmt.Errorf("config: channel %s/%s: unknown mode %q", ch.Account, ch.Channel, ch.Mode)
			}
		}
	}
	return nil
}
