package discord

import (
	"errors"

	"github.com/bwmarrin/discordgo"
)

var errNoPerm = errors.New("missing permission")

type webhook struct {
	id    string
	token string
}

func (b *Bridge) webhookFor(ch string) (webhook, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if h, ok := b.hooks[ch]; ok {
		return h, nil
	}

	perms, err := b.session.State.UserChannelPermissions(b.session.State.User.ID, ch)
	if err == nil && perms&discordgo.PermissionManageWebhooks == 0 {
		return webhook{}, errNoPerm
	}

	existing, err := b.session.ChannelWebhooks(ch)
	if err != nil {
		return webhook{}, err
	}
	for _, w := range existing {
		if w.User != nil && b.self(w.User.ID, "") {
			h := webhook{w.ID, w.Token}
			b.hooks[ch] = h
			return h, nil
		}
	}
	w, err := b.session.WebhookCreate(ch, b.session.State.User.Username, "")
	if err != nil {
		return webhook{}, err
	}
	h := webhook{w.ID, w.Token}
	b.hooks[ch] = h
	return h, nil
}
