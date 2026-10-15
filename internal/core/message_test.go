package core

import (
	"context"
	"testing"

	"github.com/f1nniboy/bridgery/internal/span"
)

func TestExcerpt(t *testing.T) {
	cases := []struct {
		name    string
		content span.Spans
		n       int
		want    string
	}{
		{"short", span.Plain("hello"), 10, "hello"},
		{"exact", span.Plain("hello"), 5, "hello"},
		{"truncated", span.Plain("hello world"), 5, "hello…"},
		{"trims trailing space", span.Plain("hello world"), 6, "hello…"},
		{"newlines flattened", span.Plain("a\nb"), 10, "a b"},
		{"runes not bytes", span.Plain("héllo wörld"), 5, "héllo…"},
		{"mentions kept whole", span.Of("hi ", span.Mention("bob"), " there"), 5, "hi @bob t…"},
		{"empty", nil, 10, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := Message{Content: c.content}
			if got := m.Excerpt(c.n).Text(); got != c.want {
				t.Errorf("Excerpt(%d) = %q, want %q", c.n, got, c.want)
			}
		})
	}
}

func TestEditHelpers(t *testing.T) {
	m := Message{Content: span.Plain("b")}
	m.Prepend("a ")
	m.Append(" ", span.Mention("c"))
	if got := m.Content.Text(); got != "a b @c" {
		t.Errorf("got %q, want %q", got, "a b @c")
	}
	m.Set("x")
	if got := m.Content.Text(); got != "x" {
		t.Errorf("Set: got %q, want %q", got, "x")
	}
}

func TestAuthorDisplay(t *testing.T) {
	if got := (Author{ID: "1", Name: "bob"}).Display(); got != "bob" {
		t.Errorf("got %q, want bob", got)
	}
	if got := (Author{ID: "1"}).Display(); got != "1" {
		t.Errorf("got %q, want id fallback", got)
	}
}

func TestCapSetWithout(t *testing.T) {
	c := Caps(CapReply, CapEdit)
	w := c.Without(CapReply)
	if w.Has(CapReply) || !w.Has(CapEdit) {
		t.Errorf("Without(reply) = %v", w)
	}
	if !c.Has(CapReply) {
		t.Error("Without mutated the original set")
	}
}

func TestLazyFile(t *testing.T) {
	f := File{Fetch: func(context.Context) ([]byte, error) { return []byte("x"), nil }}
	if f.Empty() {
		t.Error("lazy file reported empty")
	}
	if b, err := f.Bytes(context.Background()); err != nil || string(b) != "x" {
		t.Errorf("Bytes: %q, %v", b, err)
	}
	m := Message{Author: Author{Avatar: f}, Files: []File{f}}.WithoutData()
	if m.Author.Avatar.Fetch != nil || m.Files[0].Fetch != nil {
		t.Error("WithoutData kept Fetch")
	}
}
