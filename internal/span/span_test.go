package span

import (
	"slices"
	"testing"
)

func TestOf(t *testing.T) {
	m := Mention("bob")
	cases := []struct {
		name  string
		parts []any
		want  Spans
	}{
		{"none", nil, nil},
		{"string", []any{"hi"}, Plain("hi")},
		{"empty string dropped", []any{"", "x"}, Plain("x")},
		{"mixed", []any{"a ", m, Spans{m}}, Spans{{KindText, "a "}, m, m}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Of(c.parts...); !slices.Equal(got, c.want) {
				t.Errorf("Of(%v) = %v, want %v", c.parts, got, c.want)
			}
		})
	}
}

func TestOfPanicsOnUnsupported(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic")
		}
	}()
	Of(42)
}

func TestText(t *testing.T) {
	cases := []struct {
		in   Spans
		want string
	}{
		{nil, ""},
		{Plain("hi"), "hi"},
		{Spans{Mention("bob")}, "@bob"},
		{Of("hey ", Mention("bob"), "!"), "hey @bob!"},
	}
	for _, c := range cases {
		if got := c.in.Text(); got != c.want {
			t.Errorf("Text(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
