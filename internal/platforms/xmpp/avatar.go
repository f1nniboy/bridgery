package xmpp

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"strings"
	"unicode"

	"mellium.im/xmlstream"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"

	"github.com/f1nniboy/bridgery/internal/core"
)

// inbound: an occupant advertised a photo hash (XEP-0153), fetch it if new
func (b *Bridge) trackAvatar(r *room, nick string, from jid.JID, hash string) {
	if hash == "" {
		r.setOccupantAvatar(nick, core.File{})
		return
	}
	if cur, ok := r.occupantAvatar(nick); ok && cur.Key == hash {
		return
	}
	go b.fetchAvatar(r, nick, from, hash)
}

func (b *Bridge) fetchAvatar(r *room, nick string, from jid.JID, hash string) {
	ctx, cancel := context.WithTimeout(context.Background(), iqTimeout)
	defer cancel()
	var v vCard
	err := b.session.UnmarshalIQElement(ctx,
		xmlstream.Wrap(nil, xml.StartElement{Name: xml.Name{Space: nsVCard, Local: "vCard"}}),
		stanza.IQ{Type: stanza.GetIQ, From: b.self, To: from},
		&v)
	if err != nil {
		b.Log.Warn("xmpp vcard fetch", "nick", nick, "err", err)
		return
	}
	if v.Photo == nil || v.Photo.BinVal == "" {
		return
	}
	data, err := base64.StdEncoding.DecodeString(stripSpace(v.Photo.BinVal))
	if err != nil {
		b.Log.Warn("xmpp vcard decode", "nick", nick, "err", err)
		return
	}
	name := hash
	// TODO: maybe a bit hacky?
	if ext, ok := strings.CutPrefix(v.Photo.Type, "image/"); ok && ext != "" {
		name += "." + ext
	}
	r.setOccupantAvatar(nick, core.File{Data: data, Key: hash, Name: name, MIME: v.Photo.Type})
}

// outbound: publish a puppet's avatar, once per identity, so xmpp clients can see it
type puppetAvatar struct {
	want string // source avatar key being published
	key  string // source avatar key of data
	data []byte
	mime string
	hash string // sha1 of data
}

func (b *Bridge) claimAvatar(pj jid.JID, key string) bool {
	b.avatarMu.Lock()
	defer b.avatarMu.Unlock()
	a, ok := b.avatars[pj.String()]
	if ok && a.want == key {
		return false
	}
	if !ok {
		a = &puppetAvatar{}
		b.avatars[pj.String()] = a
	}
	a.want = key
	return true
}

func (b *Bridge) publishAvatar(pj jid.JID, av core.File) {
	var data []byte
	mime := av.MIME
	if !av.Empty() {
		ctx, cancel := context.WithTimeout(context.Background(), iqTimeout)
		defer cancel()
		var err error
		if data, err = av.Bytes(ctx); err != nil {
			b.Log.Warn("xmpp puppet avatar fetch", "jid", pj.String(), "err", err)
			b.settleAvatar(pj, av.Key, nil)
			return
		}
		if mime == "" {
			mime = http.DetectContentType(data)
		}
	}
	hash := ""
	if len(data) > 0 {
		sum := sha1.Sum(data)
		hash = hex.EncodeToString(sum[:])
	}
	if !b.settleAvatar(pj, av.Key, &puppetAvatar{key: av.Key, data: data, mime: mime, hash: hash}) {
		return
	}
	for _, to := range b.puppetOccupants(pj) {
		_ = b.session.Encode(context.Background(), outPresence{
			Presence: stanza.Presence{To: to, From: pj},
			VCard:    &vcardX{Photo: hash},
		})
	}
}

func (b *Bridge) settleAvatar(pj jid.JID, key string, next *puppetAvatar) bool {
	b.avatarMu.Lock()
	defer b.avatarMu.Unlock()
	id := pj.String()
	a, ok := b.avatars[id]
	if !ok || a.want != key {
		return false // superseded by a newer avatar
	}
	if next == nil { // fetch failed
		if a.key == key {
			delete(b.avatars, id) // nothing published yet, retry from scratch
		} else {
			a.want = a.key // retry on the next message
		}
		return false
	}
	changed := a.hash != next.hash
	next.want = key
	*a = *next
	return changed
}

func (b *Bridge) avatar(pj jid.JID) (puppetAvatar, bool) {
	b.avatarMu.Lock()
	defer b.avatarMu.Unlock()
	if a, ok := b.avatars[pj.String()]; ok {
		return *a, true
	}
	return puppetAvatar{}, false
}

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}
