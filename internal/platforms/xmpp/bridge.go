package xmpp

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"
	"time"

	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp"
	"mellium.im/xmpp/component"
	"mellium.im/xmpp/dial"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/stanza"
	"mellium.im/xmpp/stream"

	"github.com/f1nniboy/bridgery/internal/await"
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
	addr    jid.JID       // configured jid (component domain or account)
	self    jid.JID       // what we put in outgoing "from"
	done    chan struct{} // closed when the stream ends

	roomMu sync.Mutex
	rooms  map[string]*room

	joins  await.Map[string, error]  // room -> join outcome
	echoes await.Map[string, string] // origin-id -> stanza-id

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
		return negotiationErr(err)
	}

	b.done = make(chan struct{})
	go b.serve(ctx, b.done)
	return nil
}

// https://datatracker.ietf.org/doc/html/rfc6120#section-4.9.3
func negotiationErr(err error) error {
	if se, ok := errors.AsType[stream.Error](err); ok {
		switch se.Err {
		case "connection-timeout", "internal-server-error", "remote-connection-failed", "reset", "resource-constraint", "see-other-host", "system-shutdown":
			return err
		}
		return fmt.Errorf("%w: %w", err, core.ErrPermanent)
	}
	if _, ok := errors.AsType[net.Error](err); ok || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.Canceled) {
		return err
	}
	return fmt.Errorf("%w: %w", err, core.ErrPermanent)
}

func (b *Bridge) Join(ctx context.Context, ch core.ChannelSpec) error {
	r, err := b.ensureRoom(ch.Name)
	if err != nil {
		return err
	}
	failure, err := b.joins.Await(ctx, r.channel, 30*time.Second, func() error {
		return b.joinRoom(ctx, r, b.self, r.nick, ch.Key, "")
	})
	if err != nil {
		return err
	}
	return failure
}

func (b *Bridge) Close() error {
	select {
	case <-b.done:
	default:
		b.leaveAll()
	}
	err := b.session.Close()
	b.conn.Close()
	return err
}

func (b *Bridge) serve(ctx context.Context, done chan struct{}) {
	err := b.session.Serve(xmpp.HandlerFunc(func(t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
		return b.handle(ctx, t, start)
	}))
	close(done)
	if ctx.Err() != nil {
		return // closed by us
	}
	b.Log.Warn("xmpp connection lost", "err", err)
	b.Report(ctx, core.Status{Event: core.Disconnected})
}

func (b *Bridge) handle(ctx context.Context, t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
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
		b.onPresence(ctx, p)
	case "iq":
		var iq inIQ
		if err := d.Decode(&iq); err != nil {
			return err
		}
		return b.onIQ(t, iq)
	}
	return nil
}

func (b *Bridge) onPresence(ctx context.Context, p inPresence) {
	r, ok := b.roomByJID(p.From.Bare().String())
	if !ok {
		return
	}
	nick := p.From.Resourcepart()

	if p.Type == stanza.ErrorPresence {
		switch {
		case nick == r.nick:
			err := joinErr(p.Error.reason(), p.Error.permanent())
			if !b.joins.Resolve(r.channel, err) {
				b.Report(ctx, core.Status{Event: core.ChannelLost, Channel: r.channel, Err: err})
			}
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
				b.Report(ctx, core.Status{Event: core.ChannelLost, Channel: r.channel, Err: joinErr(fmt.Sprintf("muc status %d", code), permanent)})
			}
		case b.isComponent() && removed: // our puppet
			r.dropPuppetByNick(nick)
		default:
			r.setOccupant(nick, false)
		}
		return
	}
	if nick == r.nick && p.MUC.selfJoined() {
		b.joins.Resolve(r.channel, nil)
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
	if m.OriginID.ID != "" && b.echoes.Resolve(m.OriginID.ID, sid) {
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

func (b *Bridge) Deliver(ctx context.Context, d core.Delivery) (core.Receipt, error) {
	r, err := b.ensureRoom(d.Dest.Channel)
	if err != nil {
		return core.Receipt{}, err
	}
	from, err := b.sender(ctx, r, d.Event)
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

func (b *Bridge) write(ctx context.Context, v any) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return b.session.Encode(ctx, v)
}

func (b *Bridge) send(ctx context.Context, msg outMessage) (string, error) {
	oid := string(core.NewID())
	msg.OriginID = &originID{ID: oid}
	id, err := b.echoes.Await(ctx, oid, echoTimeout, func() error { return b.session.Encode(ctx, msg) })
	if errors.Is(err, await.ErrTimeout) {
		b.Log.Warn("xmpp echo timeout", "origin_id", oid)
		return "", nil
	}
	return id, err
}
