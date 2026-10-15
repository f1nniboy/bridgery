package discord

import (
	"regexp"

	"github.com/bwmarrin/discordgo"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

func extract(m *discordgo.Message) (span.Spans, []core.File) {
	content := toSpans(m)
	files := attachments(m)
	for _, s := range m.MessageSnapshots {
		if s.Message == nil {
			continue
		}
		content = append(content, toSpans(s.Message)...)
		files = append(files, attachments(s.Message)...)
	}
	return content, files
}

var reMention = regexp.MustCompile(`<@!?(\d+)>`)

func toSpans(m *discordgo.Message) span.Spans {
	names := make(map[string]string, len(m.Mentions))
	for _, u := range m.Mentions {
		names[u.ID] = author(u, nil).Name
	}

	src := m.Content
	texts := reMention.Split(src, -1)
	matches := reMention.FindAllStringSubmatch(src, -1)

	var out span.Spans
	for i, match := range matches {
		out = append(out, span.Plain(texts[i])...)
		name := names[match[1]]
		if name == "" {
			name = match[1]
		}
		out = append(out, span.Mention(name))
	}
	return append(out, span.Plain(texts[len(matches)])...)
}
