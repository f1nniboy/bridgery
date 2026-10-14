package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestToSpans(t *testing.T) {
	bob := &discordgo.User{ID: "1", Username: "bob", GlobalName: "Bob"}
	cases := []struct {
		name     string
		content  string
		mentions []*discordgo.User
		want     string
		spans    int
	}{
		{"plain", "hello", nil, "hello", 1},
		{"mention uses display name", "hi <@1>!", []*discordgo.User{bob}, "hi @Bob!", 3},
		{"legacy nick mention", "<@!1>", []*discordgo.User{bob}, "@Bob", 1},
		{"unknown mention falls back to id", "<@2>", nil, "@2", 1},
		{"adjacent mentions", "<@1><@1>", []*discordgo.User{bob}, "@Bob@Bob", 2},
		{"empty", "", nil, "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := toSpans(&discordgo.Message{Content: c.content, Mentions: c.mentions})
			if got.Text() != c.want || len(got) != c.spans {
				t.Errorf("got %q (%d spans), want %q (%d spans)", got.Text(), len(got), c.want, c.spans)
			}
		})
	}
}

func TestAuthorName(t *testing.T) {
	u := &discordgo.User{ID: "1", Username: "bob", GlobalName: "Bob"}
	cases := []struct {
		name   string
		user   *discordgo.User
		member *discordgo.Member
		want   string
	}{
		{"username", &discordgo.User{ID: "1", Username: "bob"}, nil, "bob"},
		{"global name wins", u, nil, "Bob"},
		{"nick wins", u, &discordgo.Member{Nick: "bobby"}, "bobby"},
		{"empty nick ignored", u, &discordgo.Member{}, "Bob"},
	}
	for _, c := range cases {
		if got := author(c.user, c.member).Name; got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
