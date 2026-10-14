package xmpp

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"

	"github.com/f1nniboy/bridgery/internal/core"
)

type room struct {
	channel string
	jid     jid.JID // room@muc
	nick    string  // bot nick

	mu        sync.Mutex
	puppets   map[string]*puppet   // identity key -> puppet
	occupants map[string]core.File // every nick taken in the room -> its inbound avatar
}

type puppet struct {
	nick string
	jid  jid.JID
}

func (r *room) puppetJID(key string) (jid.JID, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.puppets[key]; ok {
		return p.jid, true
	}
	return jid.JID{}, false
}

func (r *room) addPuppet(key string, j jid.JID, base string) *puppet {
	r.mu.Lock()
	defer r.mu.Unlock()
	nick := r.freeNick(base)
	r.occupants[nick] = core.File{}
	p := &puppet{nick: nick, jid: j}
	r.puppets[key] = p
	return p
}

func (r *room) freeNick(base string) string {
	if _, ok := r.occupants[base]; !ok {
		return base
	}
	for n := 1; ; n++ {
		cand := fmt.Sprintf("%s [%d]", base, n)
		if _, ok := r.occupants[cand]; !ok {
			return cand
		}
	}
}

func (r *room) setOccupant(nick string, present bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, known := r.occupants[nick]
	switch {
	case !present:
		delete(r.occupants, nick)
	case !known:
		r.occupants[nick] = core.File{}
	}
}

func (r *room) owns(nick string) bool {
	if nick == r.nick {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.puppets {
		if p.nick == nick {
			return true
		}
	}
	return false
}

func (r *room) occupantAvatar(nick string) (core.File, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.occupants[nick]
	return f, ok && !f.Empty()
}

func (r *room) setOccupantAvatar(nick string, f core.File) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.occupants[nick]; ok {
		r.occupants[nick] = f
	}
}

func (r *room) puppetNick(pj jid.JID) (string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.puppets {
		if p.jid.Equal(pj) {
			return p.nick, true
		}
	}
	return "", false
}

func (r *room) dropPuppet(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.puppets[key]; ok {
		delete(r.occupants, p.nick)
		delete(r.puppets, key)
	}
}

func (r *room) dropPuppetByNick(nick string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.occupants, nick)
	for key, p := range r.puppets {
		if p.nick == nick {
			delete(r.puppets, key)
			return
		}
	}
}

func (r *room) drainPuppets() []*puppet {
	r.mu.Lock()
	defer r.mu.Unlock()
	puppets := make([]*puppet, 0, len(r.puppets))
	for _, p := range r.puppets {
		puppets = append(puppets, p)
	}
	r.puppets = make(map[string]*puppet)
	return puppets
}

func (b *Bridge) ensureRoom(channel string) (*room, error) {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	if r, ok := b.rooms[channel]; ok {
		return r, nil
	}
	rjid, err := jid.Parse(channel)
	if err != nil {
		return nil, fmt.Errorf("invalid room jid %q: %w", channel, err)
	}
	r := &room{
		channel:   channel,
		jid:       rjid.Bare(),
		nick:      b.botNick(),
		puppets:   make(map[string]*puppet),
		occupants: make(map[string]core.File),
	}
	b.rooms[channel] = r
	return r, nil
}

func (b *Bridge) roomByJID(bare string) (*room, bool) {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	for _, r := range b.rooms {
		if r.jid.String() == bare {
			return r, true
		}
	}
	return nil, false
}

func (b *Bridge) puppetOccupants(pj jid.JID) []jid.JID {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	var out []jid.JID
	for _, r := range b.rooms {
		if nick, ok := r.puppetNick(pj); ok {
			if to, err := r.jid.WithResource(nick); err == nil {
				out = append(out, to)
			}
		}
	}
	return out
}

func (b *Bridge) sender(r *room, ev core.Event) (jid.JID, error) {
	if b.isComponent() {
		return b.ensurePuppet(r, ev)
	}
	return b.addr, nil
}

func (b *Bridge) occupantNick(a core.Author) string {
	if b.isComponent() {
		return puppetNick(a)
	}
	return b.botNick()
}

func (b *Bridge) ensurePuppet(r *room, ev core.Event) (jid.JID, error) {
	key := ev.Origin.Account + "\x00" + ev.Author.ID

	pj, ok := r.puppetJID(key)
	if !ok {
		var err error
		if pj, err = jid.New(shortHash(key), b.addr.Domainpart(), ""); err != nil {
			return jid.JID{}, err
		}
		p := r.addPuppet(key, pj, puppetNick(ev.Author))
		av, _ := b.avatar(pj)
		if err := b.joinRoom(r, pj, p.nick, "", av.hash); err != nil {
			r.dropPuppet(key)
			return jid.JID{}, err
		}
	}
	if b.claimAvatar(pj, ev.Author.Avatar.Key) {
		go b.publishAvatar(pj, ev.Author.Avatar)
	}
	return pj, nil
}

func (b *Bridge) joinRoom(r *room, from jid.JID, nick, key, photo string) error {
	to, err := r.jid.WithResource(nick)
	if err != nil {
		return err
	}
	pres := outPresence{
		Presence: stanza.Presence{To: to, From: from},
		X:        &mucX{History: &mucHistory{MaxStanzas: 0}, Password: key},
	}
	if photo != "" {
		pres.VCard = &vcardX{Photo: photo}
	}
	return b.session.Encode(context.Background(), pres)
}

func (b *Bridge) botNick() string {
	if b.Nick != "" {
		return b.Nick
	}
	if j, err := jid.Parse(b.JID); err == nil && j.Localpart() != "" {
		return j.Localpart()
	}
	return "bridge"
}

func (b *Bridge) leaveAll() {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	for _, r := range b.rooms {
		leaveRoom(b, r.jid, r.nick, b.self)
		for _, p := range r.drainPuppets() {
			leaveRoom(b, r.jid, p.nick, p.jid)
		}
	}
}

func leaveRoom(b *Bridge, roomJID jid.JID, nick string, from jid.JID) {
	to, err := roomJID.WithResource(nick)
	if err != nil {
		return
	}
	_ = b.session.Encode(context.Background(), outPresence{
		Presence: stanza.Presence{To: to, From: from, Type: stanza.UnavailablePresence},
	})
}

func puppetNick(a core.Author) string {
	return sanitizeNick(a.Display())
}

func sanitizeNick(nick string) string {
	sanitize := func(r rune) rune {
		switch {
		case r == '/' || unicode.IsControl(r):
			return '-'
		case r > 0xFFFF || unicode.Is(unicode.So, r):
			return -1
		}
		return r
	}
	out := strings.TrimSpace(strings.Map(sanitize, nick))
	if out == "" {
		return "unknown"
	}
	return out
}

func shortHash(v string) string {
	sum := sha1.Sum([]byte(v))
	return hex.EncodeToString(sum[:])[:12]
}
