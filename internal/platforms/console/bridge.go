package console

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

type Bridge struct {
	*core.Common `toml:"-"`

	User string `toml:"user,commented" comment:"name used for stdin messages"`
}

func (b *Bridge) Name() string         { return "Console" }
func (b *Bridge) Example() core.Bridge { return &Bridge{} }
func (b *Bridge) Caps() core.CapSet    { return nil }

func (b *Bridge) Open(context.Context) error { return nil }

func (b *Bridge) Join(_ context.Context, ch core.ChannelSpec) error {
	go b.readInput(ch.Name)
	return nil
}

func (b *Bridge) readInput(channel string) {
	user := b.User
	if user == "" {
		user = "console"
	}
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		b.Send(core.Inbound{
			Kind:    core.KindPost,
			Origin:  core.Address{Channel: channel},
			Message: core.Message{Author: core.Author{ID: user, Name: user}, Content: span.Plain(line)},
		})
	}
	if err := sc.Err(); err != nil {
		b.Log.Warn("stdin read", "err", err)
	}
}

func (b *Bridge) Close() error { return nil }

func (b *Bridge) Deliver(ctx context.Context, d core.Delivery) (core.Receipt, error) {
	e := d.Event
	fmt.Printf("[%s] %s %s: %s\n", e.Kind, e.Origin, e.Author.Display(), e.Excerpt(1000).Text())
	return core.Receipt{}, nil
}
