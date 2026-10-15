package matrix

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

func (b *Bridge) skipHistory(_ context.Context, resp *mautrix.RespSync, since string) bool {
	for _, r := range resp.Rooms.Join {
		r.Timeline.Events = dropHistory(r.Timeline.Events, b.client.UserID, since == "")
	}
	return true
}

func dropHistory(evs []*event.Event, self id.UserID, all bool) []*event.Event {
	cut := -1
	if all {
		cut = len(evs) - 1
	} else {
		for i, ev := range evs {
			if joined(ev, self) {
				cut = i
			}
		}
	}
	if cut < 0 {
		return evs
	}
	kept := make([]*event.Event, 0, len(evs))
	for i, ev := range evs {
		if i > cut || ev.StateKey != nil {
			kept = append(kept, ev)
		}
	}
	return kept
}

func joined(ev *event.Event, self id.UserID) bool {
	if ev.Type != event.StateMember || ev.GetStateKey() != self.String() || ev.Content.Raw["membership"] != string(event.MembershipJoin) {
		return false
	}
	prev := ev.Unsigned.PrevContent
	return prev == nil || prev.Raw["membership"] != string(event.MembershipJoin)
}

func (b *Bridge) onMessage(ctx context.Context, ev *event.Event) {
	r, ok := b.roomByID(ev.RoomID)
	if !ok || b.ownEvent(ev) {
		return
	}
	in := core.Inbound{Kind: core.KindPost, Origin: core.Address{Channel: r.name}, NativeID: ev.ID.String()}
	c := ev.Content.AsMessage()
	if target := c.RelatesTo.GetReplaceID(); target != "" {
		if c.NewContent == nil {
			return
		}
		in.Kind, in.NativeID, c = core.KindEdit, target.String(), c.NewContent
	} else {
		c.RemoveReplyFallback()
		in.ParentID = c.RelatesTo.GetNonFallbackReplyTo().String()
	}
	text, files := b.content(c)
	if text == "" && len(files) == 0 {
		return
	}
	in.Message = core.Message{Author: b.author(ctx, r, ev.Sender), Content: span.Plain(text), Files: files}
	b.Send(in)
}

// https://spec.matrix.org/v1.16/client-server-api/#mroommessage-msgtypes
func (b *Bridge) content(c *event.MessageEventContent) (string, []core.File) {
	switch c.MsgType {
	case event.MsgText, event.MsgNotice, event.MsgEmote:
		return c.Body, nil
	case event.MsgImage, event.MsgFile, event.MsgVideo, event.MsgAudio:
		if c.URL == "" {
			return c.GetCaption(), nil
		}
		f := core.File{Name: c.GetFileName(), Key: string(c.URL), Fetch: b.fetch(c.URL)}
		if c.Info != nil {
			f.MIME = c.Info.MimeType
		}
		return c.GetCaption(), []core.File{f}
	}
	return "", nil
}

func (b *Bridge) onRedaction(_ context.Context, ev *event.Event) {
	r, ok := b.roomByID(ev.RoomID)
	if !ok || b.ownEvent(ev) {
		return
	}
	target := ev.Redacts
	if target == "" {
		target = ev.Content.AsRedaction().Redacts
	}
	b.Send(core.Inbound{
		Kind:     core.KindDelete,
		Origin:   core.Address{Channel: r.name},
		NativeID: target.String(),
	})
}

func (b *Bridge) onTyping(ctx context.Context, ev *event.Event) {
	r, ok := b.roomByID(ev.RoomID)
	if !ok {
		return
	}
	now := ev.Content.AsTyping().UserIDs
	for _, u := range started(r.typing, now) {
		if b.ownUser(u) {
			continue
		}
		b.Send(core.Inbound{
			Kind:    core.KindTyping,
			Origin:  core.Address{Channel: r.name},
			Message: core.Message{Author: b.author(ctx, r, u)},
		})
	}
	r.typing = now
}

func started(prev, now []id.UserID) []id.UserID {
	var out []id.UserID
	for _, u := range now {
		if !slices.Contains(prev, u) {
			out = append(out, u)
		}
	}
	return out
}

func (b *Bridge) onLeave(ctx context.Context, resp *mautrix.RespSync, _ string) bool {
	for rid, left := range resp.Rooms.Leave {
		r, ok := b.roomByID(rid)
		if !ok {
			continue
		}
		err := errors.New("removed from the room")
		if banned(left.Timeline.Events, b.client.UserID) {
			err = fmt.Errorf("banned from the room: %w", core.ErrPermanent)
		}
		b.dropRoom(r)
		b.Report(ctx, core.Status{Event: core.ChannelLost, Channel: r.name, Err: err})
	}
	return true
}

func banned(evs []*event.Event, self id.UserID) bool {
	for _, ev := range slices.Backward(evs) {
		if ev.Type == event.StateMember && ev.GetStateKey() == self.String() {
			return ev.Content.Raw["membership"] == string(event.MembershipBan)
		}
	}
	return false
}

func (b *Bridge) onEncryption(ctx context.Context, ev *event.Event) {
	r, ok := b.roomByID(ev.RoomID)
	if !ok {
		return
	}
	b.dropRoom(r)
	b.Report(ctx, core.Status{Event: core.ChannelLost, Channel: r.name, Err: errEncrypted})
}

func (b *Bridge) author(ctx context.Context, r *room, user id.UserID) core.Author {
	a := core.Author{ID: user.String()}
	m, _ := b.stateStore.TryGetMember(ctx, r.id, user)
	if m == nil {
		return a
	}
	a.Name = m.Displayname
	if m.AvatarURL != "" {
		a.Avatar = core.File{Name: m.AvatarURL.ParseOrIgnore().FileID, Key: string(m.AvatarURL), Fetch: b.fetch(m.AvatarURL)}
	}
	return a
}

func (b *Bridge) ownEvent(ev *event.Event) bool {
	if b.PuppetPrefix == "" {
		return ev.Sender == b.client.UserID && ev.Unsigned.TransactionID != ""
	}
	return b.ownUser(ev.Sender)
}

func (b *Bridge) ownUser(u id.UserID) bool {
	if u == b.client.UserID {
		return true
	}
	return b.PuppetPrefix != "" && u.Homeserver() == b.client.UserID.Homeserver() && strings.HasPrefix(u.Localpart(), b.PuppetPrefix)
}
