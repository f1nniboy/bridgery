package xmpp

import (
	"testing"

	"mellium.im/xmpp/jid"

	"github.com/f1nniboy/bridgery/internal/core"
)

func TestSanitizeNick(t *testing.T) {
	cases := []struct{ in, want string }{
		{"bob", "bob"},
		{"  bob  ", "bob"},
		{"a/b", "a-b"},
		{"tab\there", "tab-here"},
		{"bob 🎉", "bob"},
		{"★ star", "star"},
		{"🎉", "unknown"},
		{"", "unknown"},
	}
	for _, c := range cases {
		if got := sanitizeNick(c.in); got != c.want {
			t.Errorf("sanitizeNick(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFreeNick(t *testing.T) {
	r := &room{puppets: map[string]*puppet{}, occupants: map[string]core.File{"alice": {}}}
	want := []string{"bob", "bob [1]", "bob [2]"}
	for _, w := range want {
		if got := r.addPuppet(w, jid.JID{}, "bob").nick; got != w {
			t.Errorf("nick = %q, want %q", got, w)
		}
	}
	r.dropPuppet("bob [1]")
	if got := r.addPuppet("x", jid.JID{}, "bob").nick; got != "bob [1]" {
		t.Errorf("freed nick not reused, got %q", got)
	}
}
