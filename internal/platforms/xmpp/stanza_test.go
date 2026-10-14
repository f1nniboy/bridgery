package xmpp

import (
	"slices"
	"testing"
)

func TestStripFallback(t *testing.T) {
	r := func(s, e int) fallbackBody { return fallbackBody{Start: &s, End: &e} }
	reply := func(bodies ...fallbackBody) []fallback { return []fallback{{For: nsReply, Body: bodies}} }

	cases := []struct {
		name string
		text string
		fbs  []fallback
		want string
	}{
		{"none", "hello", nil, "hello"},
		{"quote prefix", "> bob: hi\nhello", reply(r(0, 10)), "hello"},
		{"runes not bytes", "> ö\nhi", reply(r(0, 4)), "hi"},
		{"multiple ranges", "aXXbYYc", reply(r(1, 3), r(4, 6)), "abc"},
		{"overlapping ranges", "0123456789", reply(r(5, 10), r(0, 8)), ""},
		{"nested ranges", "aXYZb", reply(r(1, 4), r(2, 3)), "ab"},
		{"no range strips all", "> quote", reply(fallbackBody{}), ""},
		{"out of bounds ignored", "hi", reply(r(0, 10)), "hi"},
		{"other namespace ignored", "> q\nhi", []fallback{{For: "urn:other", Body: []fallbackBody{r(0, 4)}}}, "> q\nhi"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stripFallback(c.text, c.fbs); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestFiles(t *testing.T) {
	cases := []struct {
		name string
		urls []string
		want []string // names
	}{
		{"none", nil, nil},
		{"upload", []string{"https://up.example.org/abc/cat%20pic.png"}, []string{"cat pic.png"}},
		{"no path", []string{"https://example.org"}, []string{"file"}},
		{"non-http skipped", []string{"sip:alice@example.org", "ftp://x/y"}, nil},
		{"several", []string{"https://a/1.jpg", "http://b/2.txt"}, []string{"1.jpg", "2.txt"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m inMessage
			for _, u := range c.urls {
				m.OOB = append(m.OOB, oob{URL: u})
			}
			var got []string
			for _, f := range m.files() {
				if !f.Public || f.URL == "" {
					t.Errorf("file %+v not a public url", f)
				}
				got = append(got, f.Name)
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("names = %v, want %v", got, c.want)
			}
		})
	}
}
