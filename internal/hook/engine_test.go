package hook

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

func load(t *testing.T, scripts ...string) (*Engine, error) {
	t.Helper()
	dir := t.TempDir()
	for i, s := range scripts {
		name := filepath.Join(dir, string(rune('a'+i))+".lua")
		if err := os.WriteFile(name, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return New(Config{Dir: dir}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func engine(t *testing.T, scripts ...string) *Engine {
	t.Helper()
	e, err := load(t, scripts...)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func post(text string, files ...core.File) core.Event {
	return core.Event{Kind: core.KindPost, Message: core.Message{
		Author:  core.Author{ID: "1", Name: "bob"},
		Content: span.Plain(text),
		Files:   files,
	}}
}

func TestReturnValues(t *testing.T) {
	cases := []struct {
		name string
		body string
		keep bool
	}{
		{"no return keeps", ``, true},
		{"bare return keeps", `return`, true},
		{"nil keeps", `return nil`, true},
		{"true keeps", `return true`, true},
		{"false drops", `return false`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := engine(t, `on("receive", function(ev) `+c.body+` end)`)
			if _, keep := e.Receive(post("hi")); keep != c.keep {
				t.Errorf("keep = %v, want %v", keep, c.keep)
			}
		})
	}
}

func TestHandlersChainInOrder(t *testing.T) {
	e := engine(t,
		`on("deliver", function(ev) ev:append("1") end)`,
		`on("deliver", function(ev) ev:append("2") end)`,
	)
	out, _ := e.Deliver(post("x"), Target{})
	if got := out.Content.Text(); got != "x12" {
		t.Errorf("got %q, want x12", got)
	}
}

func TestErrorRollsBack(t *testing.T) {
	e := engine(t,
		`on("deliver", function(ev) ev:append(" a") end)`,
		`on("deliver", function(ev) ev:append(" b"); error("boom") end)`,
	)
	out, keep := e.Deliver(post("x"), Target{})
	if !keep {
		t.Fatal("failing hook dropped the event")
	}
	if got := out.Content.Text(); got != "x a" {
		t.Errorf("got %q, want partial edits of the failing hook discarded", got)
	}
}

func TestBadPartFailsOpen(t *testing.T) {
	e := engine(t, `on("deliver", function(ev) ev:append(42) end)`)
	out, keep := e.Deliver(post("x"), Target{})
	if !keep || out.Content.Text() != "x" {
		t.Errorf("got %q, %v", out.Content.Text(), keep)
	}
}

func TestReceiveKeepsKind(t *testing.T) {
	e := engine(t, `on("receive", function(ev) ev.kind = "delete" end)`)
	out, _ := e.Receive(post("x"))
	if out.Kind != core.KindPost {
		t.Errorf("kind = %q, receive must not change it", out.Kind)
	}
}

func TestTarget(t *testing.T) {
	e := engine(t, `on("deliver", function(ev, dst)
		if dst:can("reply") then ev:append(" reply") end
		ev:append(" " .. dst.platform .. " " .. dst.parent:excerpt(3)[1].text)
	end)`)
	parent := &core.Message{Content: span.Plain("parent")}
	out, _ := e.Deliver(post("x"), Target{Platform: "irc", Caps: core.Caps(core.CapReply), Parent: parent})
	if got := out.Content.Text(); got != "x reply irc par…" {
		t.Errorf("got %q", got)
	}
}

func TestIpairs(t *testing.T) {
	e := engine(t, `on("deliver", function(ev)
		local n = 0
		for i, f in ipairs(ev.files) do
			f.url = f.url .. "#" .. i
			n = n + 1
		end
		for _, s in ipairs({"t"}) do ev:append(s) end
		ev:append(tostring(n))
	end)`)

	t.Run("no files", func(t *testing.T) {
		out, _ := e.Deliver(post("x"), Target{})
		if got := out.Content.Text(); got != "xt0" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("elements are live, input untouched", func(t *testing.T) {
		in := post("x", core.File{URL: "a"}, core.File{URL: "b"})
		out, _ := e.Deliver(in, Target{})
		if out.Files[0].URL != "a#1" || out.Files[1].URL != "b#2" {
			t.Errorf("files = %+v", out.Files)
		}
		if in.Files[0].URL != "a" {
			t.Error("hook mutated the caller's slice")
		}
		if got := out.Content.Text(); got != "xt2" {
			t.Errorf("got %q", got)
		}
	})
}

func TestUnknownPhase(t *testing.T) {
	if _, err := load(t, `on("recieve", function() end)`); err == nil {
		t.Error("expected load error for unknown phase")
	}
}
