package ledger

import (
	"slices"
	"testing"

	"github.com/f1nniboy/bridgery/internal/core"
)

var (
	discord = core.Address{Account: "discord", Channel: "1"}
	xmpp    = core.Address{Account: "xmpp", Channel: "room"}
)

func open(t *testing.T, max int) *memory {
	t.Helper()
	m := &memory{MaxSize: max}
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBind(t *testing.T) {
	m := open(t, 0)
	m.Bind("a", discord, []string{"d1"})
	m.Bind("a", xmpp, []string{"x1", "x2"})

	if id, ok := m.Canonical(xmpp, "x2"); !ok || id != "a" {
		t.Errorf("Canonical(x2) = %q, %v", id, ok)
	}
	if got, _ := m.Native("a", xmpp); !slices.Equal(got, []string{"x1", "x2"}) {
		t.Errorf("Native = %v", got)
	}
	if _, ok := m.Canonical(discord, "x1"); ok {
		t.Error("native ids leaked across addresses")
	}
}

func TestRebindReplaces(t *testing.T) {
	m := open(t, 0)
	m.Bind("a", discord, []string{"old"})
	m.Bind("a", discord, []string{"new"})

	if _, ok := m.Canonical(discord, "old"); ok {
		t.Error("old native still resolves")
	}
	if id, ok := m.Canonical(discord, "new"); !ok || id != "a" {
		t.Errorf("Canonical(new) = %q, %v", id, ok)
	}
}

func TestDeleteKeepsBindings(t *testing.T) {
	m := open(t, 0)
	m.Put("a", core.Message{Author: core.Author{Name: "bob"}})
	m.Bind("a", discord, []string{"d1"})
	m.Delete("a")

	if _, ok := m.Get("a"); ok {
		t.Error("message still stored")
	}
	if _, ok := m.Native("a", discord); !ok {
		t.Error("binding dropped, deletes can no longer propagate")
	}
}

func TestEviction(t *testing.T) {
	m := open(t, 2)
	for _, id := range []core.CID{"a", "b"} {
		m.Put(id, core.Message{})
		m.Bind(id, discord, []string{string(id)})
	}
	m.Put("a", core.Message{}) // touch a, so b is now oldest
	m.Put("c", core.Message{})

	if _, ok := m.Get("b"); ok {
		t.Error("b should have been evicted")
	}
	if _, ok := m.Canonical(discord, "b"); ok {
		t.Error("evicted message still resolves by native id")
	}
	for _, id := range []core.CID{"a", "c"} {
		if _, ok := m.Get(id); !ok {
			t.Errorf("%s evicted", id)
		}
	}
}

func TestNativeReturnsCopy(t *testing.T) {
	m := open(t, 0)
	m.Bind("a", discord, []string{"d1"})
	got, _ := m.Native("a", discord)
	got[0] = "mutated"
	if again, _ := m.Native("a", discord); again[0] != "d1" {
		t.Error("Native exposed internal slice")
	}
}
