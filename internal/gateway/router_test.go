package gateway

import (
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
		{Channels: []config.Channel{ch(discord, ""), ch(irc, "send"), ch(xmpp, "receive")}},
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

func lift(t *testing.T, r *Router, in core.Inbound) (core.Event, bool) {
	t.Helper()
	ev, ok, err := r.lift(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	return ev, ok
}

func lower(t *testing.T, r *Router, ev core.Event, dst core.Address, caps core.CapSet) (core.Delivery, bool) {
	t.Helper()
	d, ok, err := r.lower(t.Context(), ev, dst, caps)
	if err != nil {
		t.Fatal(err)
	}
	return d, ok
}

func inbound(kind core.Kind, native, text string) core.Inbound {
	return core.Inbound{Kind: kind, Origin: discord, NativeID: native, Message: core.Message{Content: span.Plain(text)}}
}

func relay(t *testing.T, r *Router, native, ircNative string) core.Event {
	t.Helper()
	ev, ok := lift(t, r, inbound(core.KindPost, native, "hello"))
	if !ok {
		t.Fatal("post dropped")
	}
	if err := r.ledger.Put(t.Context(), ev.ID, ev.Message); err != nil {
		t.Fatal(err)
	}
	if err := r.ledger.Bind(t.Context(), ev.ID, irc, []string{ircNative}); err != nil {
		t.Fatal(err)
	}
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
		if _, ok := lift(t, r, inbound(core.KindPost, "d1", "hello")); ok {
			t.Error("echo not dropped")
		}
	})

	t.Run("edit resolves to the original cid", func(t *testing.T) {
		r := newRouter(t)
		orig := relay(t, r, "d1", "i1")
		ev, ok := lift(t, r, inbound(core.KindEdit, "d1", "changed"))
		if !ok || ev.ID != orig.ID {
			t.Errorf("edit = %+v, %v", ev, ok)
		}
	})

	t.Run("edit of an unknown message is dropped", func(t *testing.T) {
		r := newRouter(t)
		if _, ok := lift(t, r, inbound(core.KindEdit, "nope", "x")); ok {
			t.Error("unknown edit not dropped")
		}
	})

	t.Run("delete carries the stored content once", func(t *testing.T) {
		r := newRouter(t)
		relay(t, r, "d1", "i1")
		ev, ok := lift(t, r, inbound(core.KindDelete, "d1", ""))
		if !ok || ev.Content.Text() != "hello" {
			t.Errorf("delete = %+v, %v", ev, ok)
		}
		if _, ok := lift(t, r, inbound(core.KindDelete, "d1", "")); ok {
			t.Error("second delete not dropped")
		}
	})

	t.Run("parent resolves to its cid", func(t *testing.T) {
		r := newRouter(t)
		parent := relay(t, r, "d1", "i1")
		in := inbound(core.KindPost, "d2", "reply")
		in.ParentID = "d1"
		ev, _ := lift(t, r, in)
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

		if _, ok := lower(t, r, edit, irc, none); ok {
			t.Error("edit delivered without CapEdit")
		}
		d, ok := lower(t, r, edit, irc, all)
		if !ok || !slices.Equal(d.IDs, []string{"i1"}) {
			t.Errorf("delivery = %+v, %v", d, ok)
		}
		if _, ok := lower(t, r, edit, xmpp, all); ok {
			t.Error("edit delivered to a destination that never got the post")
		}
	})

	t.Run("reply ids only with CapReply", func(t *testing.T) {
		r := newRouter(t)
		parent := relay(t, r, "d1", "i1")
		ev := core.Event{Kind: core.KindPost, ParentID: parent.ID}

		d, _ := lower(t, r, ev, irc, all)
		if !slices.Equal(d.ParentIDs, []string{"i1"}) || d.Parent == nil {
			t.Errorf("with cap: %+v", d)
		}
		d, _ = lower(t, r, ev, irc, none)
		if d.ParentIDs != nil || d.Parent == nil {
			t.Errorf("without cap: %+v", d)
		}
	})

	t.Run("typing needs the cap", func(t *testing.T) {
		r := newRouter(t)
		ev := core.Event{Kind: core.KindTyping}
		if _, ok := lower(t, r, ev, irc, none); ok {
			t.Error("typing delivered without CapTyping")
		}
		if _, ok := lower(t, r, ev, irc, all); !ok {
			t.Error("typing dropped with CapTyping")
		}
	})
}

func withHook(t *testing.T, r *Router, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.lua"), []byte(script), 0o600); err != nil {
		t.Fatal(err)
	}
	var err error
	if r.hooks, err = hook.New(hook.Config{Dir: dir}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
}

func TestHandleEdits(t *testing.T) {
	cases := []struct {
		name, hook, edit string
		relayed          bool
	}{
		{"unchanged", "", "hello", false},
		{"changed", "", "hello there", true},
		{"unchanged after receive hook", `on("receive", function(ev) ev:append("!") end)`, "hello", false},
		{"changed after receive hook", `on("receive", function(ev) ev:append("!") end)`, "hi", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newRouter(t)
			if c.hook != "" {
				withHook(t, r, c.hook)
			}
			q := make(chan core.Event, 2)
			r.routes = map[core.Address][]core.Address{discord: {irc}}
			r.queues = map[core.Address]chan core.Event{irc: q}

			r.handle(inbound(core.KindPost, "d1", "hello"), "discord")
			<-q
			r.handle(inbound(core.KindEdit, "d1", c.edit), "discord")
			if relayed := len(q) == 1; relayed != c.relayed {
				t.Errorf("edit relayed = %v, want %v", relayed, c.relayed)
			}
		})
	}
}

func TestLowerFallback(t *testing.T) {
	r := newRouter(t)
	withHook(t, r, `on("deliver", function(ev, dst) if not dst:can("edit") then ev.kind = "post" end end)`)
	orig := relay(t, r, "d1", "i1")

	d, ok := lower(t, r, core.Event{Kind: core.KindEdit, ID: orig.ID}, irc, core.Caps())
	if !ok || !d.Fallback || d.Event.Kind != core.KindPost {
		t.Errorf("delivery = %+v, %v", d, ok)
	}
	d, _ = lower(t, r, core.Event{Kind: core.KindPost, ID: orig.ID}, irc, core.Caps())
	if d.Fallback {
		t.Error("plain post marked as fallback")
	}
}

func TestDeliverSkipsDisconnectedAccount(t *testing.T) {
	r := newRouter(t)
	r.deliver(t.Context(), irc, core.Event{Kind: core.KindPost})
}
