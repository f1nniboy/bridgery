package discord

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/websocket"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

type Bridge struct {
	*core.Common `toml:"-"`

	Token config.Secret `toml:"token" comment:"Discord bot token (include Bot prefix)"`

	session *discordgo.Session
	mu      sync.Mutex
	hooks   map[string]webhook // channel id -> webhook
}

func (b *Bridge) Name() string { return "Discord" }

func (b *Bridge) Example() core.Bridge {
	return &Bridge{Token: "Bot ${DISCORD_TOKEN}"}
}

func (b *Bridge) Caps() core.CapSet {
	return core.Caps(core.CapEdit, core.CapDelete, core.CapTyping, core.CapPuppet, core.CapMedia)
}

// https://docs.discord.com/developers/topics/opcodes-and-status-codes#gateway-gateway-close-event-codes
var permCloseCodes = []int{4004, 4010, 4011, 4012, 4013, 4014}

func (b *Bridge) Open(ctx context.Context) error {
	if b.Token == "" {
		return fmt.Errorf("token is required: %w", core.ErrPermanent)
	}
	s, err := discordgo.New(string(b.Token))
	if err != nil {
		return err
	}
	s.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	b.session = s
	b.hooks = map[string]webhook{}

	s.AddHandler(b.onCreate)
	s.AddHandler(b.onUpdate)
	s.AddHandler(b.onDelete)

	if err := s.Open(); err != nil {
		var ce *websocket.CloseError
		if errors.As(err, &ce) && slices.Contains(permCloseCodes, ce.Code) {
			return fmt.Errorf("%w: %w", err, core.ErrPermanent)
		}
		return err
	}
	return nil
}

func (b *Bridge) self(id, webhookID string) bool {
	if b.session.State.User != nil && id == b.session.State.User.ID {
		return true
	}
	if webhookID == "" {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, h := range b.hooks {
		if h.id == webhookID {
			return true
		}
	}
	return false
}

func author(u *discordgo.User, member *discordgo.Member) core.Author {
	name := u.Username
	if u.GlobalName != "" {
		name = u.GlobalName
	}
	if member != nil && member.Nick != "" {
		name = member.Nick
	}
	return core.Author{ID: u.ID, Name: name, Avatar: core.File{URL: u.AvatarURL(""), Public: true, Key: u.Avatar}}
}

func (b *Bridge) Join(core.ChannelSpec) error { return nil }

func (b *Bridge) Close() error {
	if b.session == nil {
		return nil
	}
	return b.session.Close()
}

func (b *Bridge) onCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if b.self(m.Author.ID, m.WebhookID) {
		return
	}
	content, files := extract(m.Message)
	in := core.Inbound{
		Kind:     core.KindPost,
		Origin:   core.Address{Channel: m.ChannelID},
		NativeID: m.ID,
		Message: core.Message{
			Author:  author(m.Author, m.Member),
			Content: content,
			Files:   files,
		},
	}
	if r := m.MessageReference; r != nil && r.Type == discordgo.MessageReferenceTypeDefault {
		in.ParentID = r.MessageID
	}
	b.Send(in)
}

func (b *Bridge) onUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	// a nil edit timestamp is an embed update, not a real edit
	if m.Author == nil || m.EditedTimestamp == nil || b.self(m.Author.ID, m.WebhookID) {
		return
	}
	content, files := extract(m.Message)
	in := core.Inbound{
		Kind:     core.KindEdit,
		Origin:   core.Address{Channel: m.ChannelID},
		NativeID: m.ID,
		Message: core.Message{
			Author:  author(m.Author, m.Member),
			Content: content,
			Files:   files,
		},
	}
	if r := m.MessageReference; r != nil && r.Type == discordgo.MessageReferenceTypeDefault {
		in.ParentID = r.MessageID
	}
	b.Send(in)
}

func (b *Bridge) onDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	b.Send(core.Inbound{
		Kind:     core.KindDelete,
		Origin:   core.Address{Channel: m.ChannelID},
		NativeID: m.ID,
	})
}

func (b *Bridge) Deliver(ctx context.Context, d core.Delivery) (core.Receipt, error) {
	ch := d.Dest.Channel

	if d.Event.Kind == core.KindTyping {
		return core.Receipt{}, b.session.ChannelTyping(ch, discordgo.WithContext(ctx))
	}

	h, err := b.webhookFor(ch)
	if errors.Is(err, errNoPerm) {
		b.Log.Warn("missing Manage Webhooks permission, dropping message", "channel", ch)
		return core.Receipt{}, nil
	}
	if err != nil {
		return core.Receipt{}, err
	}

	switch d.Event.Kind {
	case core.KindPost:
		text := d.Event.Content.Text()
		files := b.uploads(ctx, ch, d.Event.Files)
		if text == "" && len(files) == 0 {
			return core.Receipt{}, nil
		}
		avatar, err := b.Media.URL(ctx, d.Event.Author.Avatar)
		if err != nil {
			b.Log.Warn("avatar resolve", "err", err)
		}
		msg, err := b.session.WebhookExecute(h.id, h.token, true, &discordgo.WebhookParams{
			Content:         text,
			Username:        d.Event.Author.Display(),
			AvatarURL:       avatar,
			Files:           files,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		}, discordgo.WithContext(ctx))
		if err != nil {
			return core.Receipt{}, err
		}
		return core.Receipt{IDs: []string{msg.ID}}, nil

	case core.KindEdit:
		text := d.Event.Content.Text()
		msg, err := b.session.WebhookMessageEdit(h.id, h.token, d.IDs[0], &discordgo.WebhookEdit{
			Content:         &text,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		}, discordgo.WithContext(ctx))
		if err != nil {
			return core.Receipt{}, err
		}
		return core.Receipt{IDs: []string{msg.ID}}, nil

	case core.KindDelete:
		for _, id := range d.IDs {
			if err := b.session.WebhookMessageDelete(h.id, h.token, id, discordgo.WithContext(ctx)); err != nil {
				return core.Receipt{}, err
			}
		}
		return core.Receipt{}, nil
	}
	return core.Receipt{}, nil
}

func extract(m *discordgo.Message) (span.Spans, []core.File) {
	content := toSpans(m)
	files := attachments(m)
	for _, s := range m.MessageSnapshots {
		if s.Message == nil {
			continue
		}
		if fwd := toSpans(s.Message); len(fwd) > 0 {
			content = append(content, fwd...)
		}
		files = append(files, attachments(s.Message)...)
	}
	return content, files
}
