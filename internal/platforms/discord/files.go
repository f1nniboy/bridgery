package discord

import (
	"bytes"
	"context"

	"github.com/bwmarrin/discordgo"

	"github.com/f1nniboy/bridgery/internal/core"
)

func attachments(m *discordgo.Message) []core.File {
	files := make([]core.File, 0, len(m.Attachments))
	for _, a := range m.Attachments {
		files = append(files, core.File{Name: a.Filename, MIME: a.ContentType, URL: a.URL, Public: true, Key: a.ID})
	}
	return files
}

func (b *Bridge) uploads(ctx context.Context, ch string, files []core.File) []*discordgo.File {
	if len(files) == 0 {
		return nil
	}
	limit := b.uploadLimit(ch)
	var out []*discordgo.File
	for _, f := range files {
		data, err := f.Bytes(ctx)
		if err != nil {
			b.Log.Warn("file fetch, dropping file", "file", f.Name, "err", err)
			continue
		}
		if len(data) > limit {
			b.Log.Warn("file over upload limit, dropping file", "file", f.Name, "size", len(data), "limit", limit)
			continue
		}
		out = append(out, &discordgo.File{Name: f.Name, ContentType: f.MIME, Reader: bytes.NewReader(data)})
	}
	return out
}

func (b *Bridge) uploadLimit(ch string) int {
	if c, err := b.session.State.Channel(ch); err == nil {
		if g, err := b.session.State.Guild(c.GuildID); err == nil {
			// https://support.discord.com/hc/en-us/articles/360028038352-Server-Boosting-FAQ
			switch g.PremiumTier {
			case discordgo.PremiumTier3:
				return 100 << 20
			case discordgo.PremiumTier2:
				return 50 << 20
			}
		}
	}
	// https://support.discord.com/hc/en-us/articles/25444343291031-File-Attachments-FAQ
	return 20 << 20
}
