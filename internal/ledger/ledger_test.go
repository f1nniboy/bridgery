package ledger

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2/unstable"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

var (
	discord = core.Address{Account: "discord", Channel: "1"}
	xmpp    = core.Address{Account: "xmpp", Channel: "room"}
)

func each(t *testing.T, fn func(t *testing.T, l backend)) {
	backends := map[string]func(t *testing.T) backend{
		"memory": func(*testing.T) backend { return &memory{} },
		"sqlite": func(t *testing.T) backend { return &sqlite{Path: filepath.Join(t.TempDir(), "ledger.db")} },
	}
	for name, open := range backends {
		t.Run(name, func(t *testing.T) {
			l := open(t)
			must(t, l.Open())
			t.Cleanup(func() { l.Close() })
			fn(t, l)
		})
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPutGet(t *testing.T) {
	each(t, func(t *testing.T, l backend) {
		in := core.Message{
			Author:  core.Author{ID: "1", Name: "bob", Avatar: core.File{URL: "https://a/b.png", Key: "k", Data: []byte("avatar")}},
			Content: span.Of("hi ", span.Mention("alice")),
			Files:   []core.File{{Name: "a.txt", URL: "https://x/a.txt", Public: true}, {Name: "b.bin", Data: []byte("raw")}},
		}
		must(t, l.Put(t.Context(), "a", in))
		got, ok, err := l.Get(t.Context(), "a")
		must(t, err)
		if !ok || !reflect.DeepEqual(got, in.WithoutData()) {
			t.Errorf("Get = %+v, %v", got, ok)
		}
		if _, ok, _ := l.Get(t.Context(), "missing"); ok {
			t.Error("missing message found")
		}
	})
}

func TestBind(t *testing.T) {
	each(t, func(t *testing.T, l backend) {
		must(t, l.Bind(t.Context(), "a", discord, []string{"d1"}))
		must(t, l.Bind(t.Context(), "a", xmpp, []string{"x2", "x1"}))

		if id, ok, _ := l.Canonical(t.Context(), xmpp, "x1"); !ok || id != "a" {
			t.Errorf("Canonical(x1) = %q, %v", id, ok)
		}
		if got, _, _ := l.Native(t.Context(), "a", xmpp); !slices.Equal(got, []string{"x2", "x1"}) {
			t.Errorf("Native = %v, want bind order", got)
		}
		if _, ok, _ := l.Canonical(t.Context(), discord, "x1"); ok {
			t.Error("native ids leaked across addresses")
		}
	})
}

func TestRebindReplaces(t *testing.T) {
	each(t, func(t *testing.T, l backend) {
		must(t, l.Bind(t.Context(), "a", discord, []string{"old"}))
		must(t, l.Bind(t.Context(), "a", discord, []string{"new"}))
		if _, ok, _ := l.Canonical(t.Context(), discord, "old"); ok {
			t.Error("old native still resolves")
		}
		if id, ok, _ := l.Canonical(t.Context(), discord, "new"); !ok || id != "a" {
			t.Errorf("Canonical(new) = %q, %v", id, ok)
		}
	})
}

func TestDeleteKeepsBindings(t *testing.T) {
	each(t, func(t *testing.T, l backend) {
		must(t, l.Put(t.Context(), "a", core.Message{Author: core.Author{Name: "bob"}}))
		must(t, l.Bind(t.Context(), "a", discord, []string{"d1"}))
		must(t, l.Delete(t.Context(), "a"))
		if _, ok, _ := l.Get(t.Context(), "a"); ok {
			t.Error("message still stored")
		}
		if _, ok, _ := l.Native(t.Context(), "a", discord); !ok {
			t.Error("binding dropped, deletes can no longer propagate")
		}
	})
}

func TestPrune(t *testing.T) {
	each(t, func(t *testing.T, l backend) {
		must(t, l.Put(t.Context(), "a", core.Message{}))
		must(t, l.Bind(t.Context(), "a", discord, []string{"d1"}))

		must(t, l.prune(t.Context(), time.Now().Add(-time.Hour)))
		if _, ok, _ := l.Get(t.Context(), "a"); !ok {
			t.Fatal("recent message pruned")
		}

		must(t, l.prune(t.Context(), time.Now().Add(time.Hour)))
		if _, ok, _ := l.Get(t.Context(), "a"); ok {
			t.Error("old message kept")
		}
		if _, ok, _ := l.Canonical(t.Context(), discord, "d1"); ok {
			t.Error("pruned message still resolves by native id")
		}
	})
}

func TestSqliteReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	first := &sqlite{Path: path}
	must(t, first.Open())
	must(t, first.Put(t.Context(), "a", core.Message{Content: span.Plain("kept")}))
	must(t, first.Close())

	again := &sqlite{Path: path}
	must(t, again.Open())
	defer again.Close()
	if got, ok, _ := again.Get(t.Context(), "a"); !ok || got.Content.Text() != "kept" {
		t.Errorf("after reopen: %+v, %v", got, ok)
	}
}

func TestOpenDecodesMaxAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	l, err := Open(config.Backends{"sqlite": unstable.RawMessage("path = '" + path + "'\nmax_age = '1h'")})
	must(t, err)
	defer l.Close()
	if got := l.(*ledger).maxAge(); got != time.Hour {
		t.Errorf("maxAge = %v, want 1h", got)
	}
}

func TestSweep(t *testing.T) {
	l, err := Open(config.Backends{"memory": unstable.RawMessage("max_age = '200ms'")})
	must(t, err)
	defer l.Close()
	must(t, l.Put(t.Context(), "a", core.Message{}))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok, _ := l.Get(t.Context(), "a"); !ok {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Error("message not swept after max_age")
}
