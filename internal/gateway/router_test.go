package gateway

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/hook"
	"github.com/f1nniboy/bridgery/internal/ledger"
	"github.com/f1nniboy/bridgery/internal/span"
)

var (
	discord = core.Address{Account: "discord", Channel: "1"}
	irc     = core.Address{Account: "irc", Channel: "#a"}
	xmpp    = core.Address{Account: "xmpp", Channel: "room"}
)

func TestBuildRoutes(t *testing.T) {
	ch := func(a core.Address, mode string) config.Channel {
		return config.Channel{Account: a.Account, Channel: a.Channel, Mode: mode}
	}
	routes := buildRoutes([]config.Gateway{
		{Channels: []config.Channel{ch(discord, ""), ch(irc, "in"), ch(xmpp, "out")}},
		{Channels: []config.Channel{ch(discord, ""), ch(xmpp, "")}},
	})

	cases := []struct {
		from core.Address
		want []core.Address
	}{
		{discord, []core.Address{xmpp}},
		{irc, []core.Address{discord, xmpp}},
		{xmpp, []core.Address{discord}},
	}
	for _, c := range cases {
		if got := routes[c.from]; !slices.Equal(got, c.want) {
			t.Errorf("routes[%s] = %v, want %v", c.from, got, c.want)
		}
	}
}

func newRouter(t *testing.T) *Router {
	t.Helper()
	l, err := ledger.Open(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &Router{ledger: l, accounts: map[string]*Account{
		"discord": {Platform: "discord"},
		"irc":     {Platform: "irc"},
	}, log: slog.New(slog.DiscardHandler)}
}

func inbound(kind core.Kind, native, text string) core.Inbound {
	return core.Inbound{Kind: kind, Origin: discord, NativeID: native, Message: core.Message{Content: span.Plain(text)}}
}

func relay(t *testing.T, r *Router, native, ircNative string) core.Event {
	t.Helper()
	ev, ok := r.lift(inbound(core.KindPost, native, "hello"))
	if !ok {
		t.Fatal("post dropped")
	}
	r.ledger.Put(ev.ID, ev.Message)
	r.ledger.Bind(ev.ID, irc, []string{ircNative})
	return ev
}

func TestLift(t *testing.T) {
	t.Run("post gets a cid and platform", func(t *testing.T) {
		r := newRouter(t)
		ev := relay(t, r, "d1", "i1")
		if ev.ID == "" || ev.Platform != "discord" {
			t.Errorf("ev = %+v", ev)
		}
	})

	t.Run("echo of a known native is dropped", func(t *testing.T) {
		r := newRouter(t)
		relay(t, r, "d1", "i1")
		if _, ok := r.lift(inbound(core.KindPost, "d1", "hello")); ok {
			t.Error("echo not dropped")
		}
	})

	t.Run("edit resolves to the original cid", func(t *testing.T) {
		r := newRouter(t)
		orig := relay(t, r, "d1", "i1")
		ev, ok := r.lift(inbound(core.KindEdit, "d1", "changed"))
		if !ok || ev.ID != orig.ID {
			t.Errorf("edit = %+v, %v", ev, ok)
		}
	})

	t.Run("edit with unchanged content is dropped", func(t *testing.T) {
		r := newRouter(t)
		relay(t, r, "d1", "i1")
		if _, ok := r.lift(inbound(core.KindEdit, "d1", "hello")); ok {
			t.Error("no-op edit not dropped")
		}
	})

	t.Run("edit of an unknown message is dropped", func(t *testing.T) {
		r := newRouter(t)
		if _, ok := r.lift(inbound(core.KindEdit, "nope", "x")); ok {
			t.Error("unknown edit not dropped")
		}
	})

	t.Run("delete carries the stored content once", func(t *testing.T) {
		r := newRouter(t)
		relay(t, r, "d1", "i1")
		ev, ok := r.lift(inbound(core.KindDelete, "d1", ""))
		if !ok || ev.Content.Text() != "hello" {
			t.Errorf("delete = %+v, %v", ev, ok)
		}
		if _, ok := r.lift(inbound(core.KindDelete, "d1", "")); ok {
			t.Error("second delete not dropped")
		}
	})

	t.Run("parent resolves to its cid", func(t *testing.T) {
		r := newRouter(t)
		parent := relay(t, r, "d1", "i1")
		in := inbound(core.KindPost, "d2", "reply")
		in.ParentID = "d1"
		ev, _ := r.lift(in)
		if ev.ParentID != parent.ID {
			t.Errorf("ParentID = %q, want %q", ev.ParentID, parent.ID)
		}
	})
}

func TestLower(t *testing.T) {
	all := core.Caps(core.AllCaps...)
	none := core.Caps()

	t.Run("edit needs the cap and a binding", func(t *testing.T) {
		r := newRouter(t)
		orig := relay(t, r, "d1", "i1")
		edit := core.Event{Kind: core.KindEdit, ID: orig.ID}

		if _, ok := r.lower(edit, irc, none); ok {
			t.Error("edit delivered without CapEdit")
		}
		d, ok := r.lower(edit, irc, all)
		if !ok || !slices.Equal(d.IDs, []string{"i1"}) {
			t.Errorf("delivery = %+v, %v", d, ok)
		}
		if _, ok := r.lower(edit, xmpp, all); ok {
			t.Error("edit delivered to a destination that never got the post")
		}
	})

	t.Run("reply ids only with CapReply", func(t *testing.T) {
		r := newRouter(t)
		parent := relay(t, r, "d1", "i1")
		ev := core.Event{Kind: core.KindPost, ParentID: parent.ID}

		d, _ := r.lower(ev, irc, all)
		if !slices.Equal(d.ParentIDs, []string{"i1"}) || d.Parent == nil {
			t.Errorf("with cap: %+v", d)
		}
		d, _ = r.lower(ev, irc, none)
		if d.ParentIDs != nil || d.Parent == nil {
			t.Errorf("without cap: %+v", d)
		}
	})

	t.Run("typing needs the cap", func(t *testing.T) {
		r := newRouter(t)
		ev := core.Event{Kind: core.KindTyping}
		if _, ok := r.lower(ev, irc, none); ok {
			t.Error("typing delivered without CapTyping")
		}
		if _, ok := r.lower(ev, irc, all); !ok {
			t.Error("typing dropped with CapTyping")
		}
	})
}

func TestLowerFallback(t *testing.T) {
	dir := t.TempDir()
	script := `on("deliver", function(ev, dst) if not dst:can("edit") then ev.kind = "post" end end)`
	if err := os.WriteFile(filepath.Join(dir, "a.lua"), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	r := newRouter(t)
	var err error
	if r.hooks, err = hook.New(hook.Config{Dir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	orig := relay(t, r, "d1", "i1")

	d, ok := r.lower(core.Event{Kind: core.KindEdit, ID: orig.ID}, irc, core.Caps())
	if !ok || !d.Fallback || d.Event.Kind != core.KindPost {
		t.Errorf("delivery = %+v, %v", d, ok)
	}
	d, _ = r.lower(core.Event{Kind: core.KindPost, ID: orig.ID}, irc, core.Caps())
	if d.Fallback {
		t.Error("plain post marked as fallback")
	}
}

func TestDeliverSkipsDisconnectedAccount(t *testing.T) {
	r := newRouter(t)
	r.deliver(t.Context(), irc, core.Event{Kind: core.KindPost})
}
