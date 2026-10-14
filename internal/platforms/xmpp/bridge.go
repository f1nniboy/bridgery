package xmpp

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp"
	"mellium.im/xmpp/component"
	"mellium.im/xmpp/dial"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

const (
	dialTimeout  = 10 * time.Second
	echoTimeout  = 10 * time.Second
	iqTimeout    = 10 * time.Second
	writeTimeout = 10 * time.Second
)

type Bridge struct {
	*core.Common `toml:"-"`

	JID      string        `toml:"jid" comment:"component domain (puppet mode) or user@domain (normal mode)"`
	Password config.Secret `toml:"password" comment:"component secret (puppet mode) or account password (normal mode)"`
	Server   string        `toml:"server,commented" comment:"connect address host:port, required for component, else SRV lookup"`
	Nick     string        `toml:"nick,commented" comment:"bot nickname in rooms, defaults to jid localpart"`

	conn    net.Conn
	session *xmpp.Session
	addr    jid.JID // configured jid (component domain or account)
	self    jid.JID // what we put in outgoing "from"
	alive   atomic.Bool

	roomMu sync.Mutex
	rooms  map[string]*room

	pendingMu sync.Mutex
	pending   map[string]chan string // origin-id -> stanza-id echo waiter

	avatarMu sync.Mutex
	avatars  map[string]*puppetAvatar // puppet bare jid -> its published avatar
}

func (b *Bridge) Name() string { return "XMPP" }

func (b *Bridge) Example() core.Bridge {
	return &Bridge{JID: "bridger@example.org", Password: "123", Server: "localhost:5347"}
}

func (b *Bridge) isComponent() bool {
	j, err := jid.Parse(b.JID)
	return err == nil && j.Localpart() == ""
}

func (b *Bridge) Caps() core.CapSet {
	if b.session == nil || b.isComponent() {
		return core.Caps(core.CapReply, core.CapTyping, core.CapDelete, core.CapPuppet)
	}
	return core.Caps(core.CapReply, core.CapTyping, core.CapDelete)
}

func (b *Bridge) Open(ctx context.Context) error {
	if b.JID == "" || b.Password == "" {
		return fmt.Errorf("jid and password are required: %w", core.ErrPermanent)
	}
	var err error
	if b.addr, err = jid.Parse(b.JID); err != nil {
		return fmt.Errorf("parse jid %q: %w, %w", b.JID, err, core.ErrPermanent)
	}

	b.rooms = make(map[string]*room)
	b.pending = make(map[string]chan string)
	b.avatars = make(map[string]*puppetAvatar)

	if b.isComponent() {
		if b.Server == "" {
			return fmt.Errorf("component mode requires server: %w", core.ErrPermanent)
		}
	}

	if b.Server != "" {
		dialer := net.Dialer{Timeout: dialTimeout}
		b.conn, err = dialer.DialContext(ctx, "tcp", b.Server)
	} else {
		b.conn, err = dial.Client(ctx, "tcp", b.addr)
	}
	if err != nil {
		return err
	}

	if b.isComponent() {
		b.self, _ = jid.New(b.botNick(), b.addr.Domainpart(), "")
		b.session, err = component.NewSession(ctx, b.addr, []byte(b.Password), b.conn)
	} else {
		b.self = b.addr
		features := []xmpp.StreamFeature{
			xmpp.StartTLS(&tls.Config{ServerName: b.addr.Domainpart()}),
			xmpp.SASL("", string(b.Password), sasl.ScramSha256, sasl.ScramSha1, sasl.Plain),
			xmpp.BindResource(),
		}
		b.session, err = xmpp.NewClientSession(ctx, b.addr, b.conn, features...)
	}
	if err != nil {
		b.conn.Close()
		return fmt.Errorf("%w: %w", err, core.ErrPermanent) // server rejected our stream
	}

	b.alive.Store(true)
	go b.serve()
	b.Report(core.Status{Event: core.Ready})
	return nil
}

func (b *Bridge) Join(ch core.ChannelSpec) error {
	r, err := b.ensureRoom(ch.Name)
	if err != nil {
		return err
	}
	return b.joinRoom(r, b.self, r.nick, ch.Key, "")
}

func (b *Bridge) Close() error {
	if b.alive.Swap(false) {
		b.leaveAll()
	}
	err := b.session.Close()
	b.conn.Close()
	return err
}

func (b *Bridge) serve() {
	err := b.session.Serve(xmpp.HandlerFunc(b.handle))
	if !b.alive.Swap(false) {
		return // closed by us
	}
	b.Log.Warn("xmpp connection lost", "err", err)
	b.Report(core.Status{Event: core.Disconnected})
}

func (b *Bridge) handle(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
	d := xml.NewTokenDecoder(xmlstream.MultiReader(xmlstream.Token(*start), t))
	switch start.Name.Local {
	case "message":
		var m inMessage
		if err := d.Decode(&m); err != nil {
			return err
		}
		b.onMessage(m)
	case "presence":
		var p inPresence
		if err := d.Decode(&p); err != nil {
			return err
		}
		b.onPresence(p)
	case "iq":
		var iq inIQ
		if err := d.Decode(&iq); err != nil {
			return err
		}
		return b.onIQ(t, iq)
	}
	return nil
}

func (b *Bridge) onPresence(p inPresence) {
	r, ok := b.roomByJID(p.From.Bare().String())
	if !ok {
		return
	}
	nick := p.From.Resourcepart()

	if p.Type == stanza.ErrorPresence {
		switch {
		case nick == r.nick:
			b.Report(core.Status{Event: core.ChannelDown, Channel: r.channel, Err: joinErr(p.Error.reason(), p.Error.permanent())})
		case b.isComponent() && nick != "":
			r.dropPuppetByNick(nick)
		}
		return
	}
	if nick == "" {
		return
	}
	if p.Type == stanza.UnavailablePresence {
		code, permanent := p.MUC.removal()
		removed := code != 0
		switch {
		case nick == r.nick: // the bridge bot
			if removed {
				b.Report(core.Status{Event: core.ChannelDown, Channel: r.channel, Err: joinErr(fmt.Sprintf("muc status %d", code), permanent)})
			}
		case b.isComponent() && removed: // our puppet
			r.dropPuppetByNick(nick)
		default:
			r.setOccupant(nick, false)
		}
		return
	}
	if nick == r.nick && p.MUC.selfJoined() {
		b.Report(core.Status{Event: core.ChannelUp, Channel: r.channel})
	}
	r.setOccupant(nick, true)
	if p.VCard == nil || r.owns(nick) {
		return
	}
	b.trackAvatar(r, nick, p.From, p.VCard.Photo)
}

func joinErr(reason string, permanent bool) error {
	if permanent {
		return fmt.Errorf("%s: %w", reason, core.ErrPermanent)
	}
	return errors.New(reason)
}

func (b *Bridge) onMessage(m inMessage) {
	if m.Type != stanza.GroupChatMessage {
		return
	}
	// drop encrypted messages so we never bridge their fallback bodies
	if m.Encryption != nil {
		return
	}
	// the MUC delivers messages to every occupant we own (bot + puppets),
	// so we only act on the bot's received message
	if m.To.Bare().String() != b.self.Bare().String() {
		return
	}
	nick := m.From.Resourcepart()
	r, ok := b.roomByJID(m.From.Bare().String())
	if !ok {
		return
	}

	sid := m.stanzaID(r.jid)

	// for echo, so we know the ID of our messages
	if m.OriginID.ID != "" && b.deliverEcho(m.OriginID.ID, sid) {
		return
	}
	if r.owns(nick) {
		return
	}

	if m.Retract.ID != "" {
		b.Send(core.Inbound{
			Kind:     core.KindDelete,
			Origin:   core.Address{Channel: r.channel},
			NativeID: m.Retract.ID,
		})
		return
	}

	if m.Composing != nil {
		b.Send(core.Inbound{
			Kind:    core.KindTyping,
			Origin:  core.Address{Channel: r.channel},
			Message: core.Message{Author: core.Author{ID: nick, Name: nick}},
		})
		return
	}

	if m.Body == "" {
		return
	}
	text := stripFallback(m.Body, m.Fallback)
	files := m.files()
	// if the message body is just a file url, drop the message body (fallback)
	if slices.ContainsFunc(files, func(f core.File) bool { return f.URL == text }) {
		text = ""
	}
	author := core.Author{ID: nick, Name: nick}
	if av, ok := r.occupantAvatar(nick); ok {
		author.Avatar = av
	}
	b.Send(core.Inbound{
		Kind:     core.KindPost,
		Origin:   core.Address{Channel: r.channel},
		NativeID: sid,
		ParentID: m.Reply.ID,
		Message: core.Message{
			Author:  author,
			Content: span.Plain(text),
			Files:   files,
		},
	})
}

func (b *Bridge) onIQ(t xmlstream.TokenReadEncoder, iq inIQ) error {
	if iq.Type != stanza.GetIQ || iq.VCard == nil {
		return nil
	}
	av, _ := b.avatar(iq.To.Bare())
	if len(av.data) == 0 {
		return nil
	}
	_, err := xmlstream.Copy(t, iq.Result(vcardPayload(av.mime, av.data)))
	return err
}

func (b *Bridge) deliverEcho(oid, sid string) bool {
	b.pendingMu.Lock()
	ch, ok := b.pending[oid]
	b.pendingMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- sid:
	default:
	}
	return true
}

func (b *Bridge) Deliver(ctx context.Context, d core.Delivery) (core.Receipt, error) {
	r, err := b.ensureRoom(d.Dest.Channel)
	if err != nil {
		return core.Receipt{}, err
	}
	from, err := b.sender(r, d.Event)
	if err != nil {
		return core.Receipt{}, err
	}
	base := stanza.Message{To: r.jid, From: from, Type: stanza.GroupChatMessage}

	switch d.Event.Kind {
	case core.KindPost:
		msg := outMessage{Message: base, Body: d.Event.Content.Text(), Active: &chatState{}}
		if len(d.ParentIDs) > 0 {
			rep := &reply{ID: d.ParentIDs[0]}
			if d.Parent != nil {
				if to, err := r.jid.WithResource(b.occupantNick(d.Parent.Author)); err == nil {
					rep.To = to.String()
				}
			}
			msg.Reply = rep
		}
		id, err := b.send(ctx, msg)
		if err != nil {
			return core.Receipt{}, err
		}
		if id == "" {
			return core.Receipt{}, nil
		}
		return core.Receipt{IDs: []string{id}}, nil

	case core.KindDelete:
		for _, id := range d.IDs {
			m := outMessage{Message: base, Retract: &retract{ID: id}, Store: &storeHint{}}
			if err := b.session.Encode(ctx, m); err != nil {
				return core.Receipt{}, err
			}
		}
		return core.Receipt{}, nil

	case core.KindTyping:
		return core.Receipt{}, b.session.Encode(ctx, outMessage{Message: base, Composing: &chatState{}})
	}
	return core.Receipt{}, nil
}

func (b *Bridge) write(v any) error {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	return b.session.Encode(ctx, v)
}

func (b *Bridge) send(ctx context.Context, msg outMessage) (string, error) {
	oid := string(core.NewID())
	msg.OriginID = &originID{ID: oid}

	ch := make(chan string, 1)
	b.pendingMu.Lock()
	b.pending[oid] = ch
	b.pendingMu.Unlock()
	defer func() {
		b.pendingMu.Lock()
		delete(b.pending, oid)
		b.pendingMu.Unlock()
	}()

	if err := b.session.Encode(ctx, msg); err != nil {
		return "", err
	}
	wait, cancel := context.WithTimeout(ctx, echoTimeout)
	defer cancel()
	select {
	case id := <-ch:
		return id, nil
	case <-wait.Done():
		b.Log.Warn("xmpp echo timeout", "origin_id", oid)
		return "", nil
	}
}
