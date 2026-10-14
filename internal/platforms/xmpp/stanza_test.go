package xmpp

import "testing"

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
