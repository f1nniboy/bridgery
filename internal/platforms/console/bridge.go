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

func (b *Bridge) Open(ctx context.Context) error {
	go b.readInput()
	return nil
}

func (b *Bridge) readInput() {
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
			Message: core.Message{Author: core.Author{ID: user, Name: user}, Content: span.Plain(line)},
		})
	}
	if err := sc.Err(); err != nil {
		b.Log.Warn("stdin read", "err", err)
	}
}

func (b *Bridge) Join(core.ChannelSpec) error { return nil }
func (b *Bridge) Close() error                { return nil }

func (b *Bridge) Deliver(ctx context.Context, d core.Delivery) (core.Receipt, error) {
	e := d.Event
	fmt.Printf("[%s] %s %s: %s\n", e.Kind, e.Origin, e.Author.Display(), e.Excerpt(1000).Text())
	return core.Receipt{}, nil
}
