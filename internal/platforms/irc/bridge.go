package irc

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
	"github.com/f1nniboy/bridgery/internal/span"
)

var requestCaps = []string{
	"message-tags",
	"server-time",
	"echo-message",
	"labeled-response",
	"batch",
	"account-tag",
	"draft/message-redaction",
}

const registerTimeout = 30 * time.Second
const sendDelay = 1000 * time.Millisecond

type Bridge struct {
	*core.Common `toml:"-"`

	Server   string        `toml:"server" comment:"host[:port]"`
	TLS      bool          `toml:"tls" comment:"connect with TLS (default port 6697, else 6667)"`
	Nick     string        `toml:"nick" comment:"nickname"`
	User     string        `toml:"user,commented" comment:"username, defaults to nick"`
	RealName string        `toml:"realname,commented" comment:"real name, defaults to nick"`
	SASLUser string        `toml:"sasl_user,commented" comment:"SASL account for authentication"`
	SASLPass config.Secret `toml:"sasl_pass,commented" comment:"SASL password"`
	Password config.Secret `toml:"password,commented" comment:"server password (PASS)"`

	conn     *ircevent.Connection
	throttle *time.Ticker
}

func (b *Bridge) Name() string { return "IRC" }

func (b *Bridge) Example() core.Bridge {
	return &Bridge{Server: "irc.libera.chat", TLS: true, Nick: "mybot"}
}

func (b *Bridge) Caps() core.CapSet {
	if b.conn == nil {
		return core.Caps(core.CapReply, core.CapDelete, core.CapTyping)
	}
	c := core.CapSet{}
	bound := b.acked("message-tags", "labeled-response", "echo-message")
	if bound {
		c[core.CapReply] = true
	}
	if bound && b.acked("draft/message-redaction") {
		c[core.CapDelete] = true
	}
	if b.acked("message-tags") {
		c[core.CapTyping] = true
	}
	return c
}

func (b *Bridge) Open(ctx context.Context) error {
	if b.Nick == "" {
		return errors.New("nick is required")
	}
	host, port, err := net.SplitHostPort(b.Server)
	if err != nil {
		host = b.Server
		port = "6667"
		if b.TLS {
			port = "6697"
		}
	}
	addr := net.JoinHostPort(host, port)

	user := b.User
	if user == "" {
		user = b.Nick
	}
	real := b.RealName
	if real == "" {
		real = b.Nick
	}
	b.conn = &ircevent.Connection{
		Server:      addr,
		Nick:        b.Nick,
		User:        user,
		RealName:    real,
		Password:    string(b.Password),
		RequestCaps: requestCaps,
		UseTLS:      b.TLS,
	}
	if b.TLS {
		b.conn.TLSConfig = &tls.Config{ServerName: host}
	}
	if b.SASLUser != "" {
		b.conn.UseSASL = true
		b.conn.SASLLogin = b.SASLUser
		b.conn.SASLPassword = string(b.SASLPass)
	}
	b.conn.Log = slog.NewLogLogger(b.Log.Handler(), slog.LevelWarn)
	b.throttle = time.NewTicker(sendDelay)

	ready := make(chan struct{}, 1)
	b.conn.AddConnectCallback(func(ircmsg.Message) {
		select {
		case ready <- struct{}{}:
		default:
		}

		if bot := b.conn.ISupport()["BOT"]; bot != "" {
			_ = b.conn.Send("MODE", b.conn.CurrentNick(), "+"+bot)
		}
		b.Report(core.Status{Event: core.Ready})
	})
	b.conn.AddCallback("PRIVMSG", b.onMessage)
	b.conn.AddCallback("TAGMSG", b.onTyping)
	b.conn.AddCallback("REDACT", b.onRedact)
	b.conn.AddCallback("KICK", b.onKick)
	b.conn.AddCallback("JOIN", b.onJoin)

	if err := b.conn.Connect(); err != nil {
		return err
	}
	select {
	case <-ready:
	case <-ctx.Done():
		b.conn.Quit()
		return ctx.Err()
	case <-time.After(registerTimeout):
		b.conn.Quit()
		return errors.New("timed out waiting for registration")
	}
	go b.conn.Loop()
	return nil
}

func (b *Bridge) Join(ch core.ChannelSpec) error {
	if ch.Key != "" {
		return b.conn.Send("JOIN", ch.Name, ch.Key)
	}
	return b.conn.Join(ch.Name)
}

func (b *Bridge) onKick(msg ircmsg.Message) {
	if len(msg.Params) >= 2 && msg.Params[1] == b.conn.CurrentNick() {
		b.Report(core.Status{Event: core.ChannelDown, Channel: msg.Params[0]})
	}
}

func (b *Bridge) onJoin(msg ircmsg.Message) {
	if msg.Nick() == b.conn.CurrentNick() && len(msg.Params) >= 1 {
		b.Report(core.Status{Event: core.ChannelUp, Channel: msg.Params[0]})
	}
}

func (b *Bridge) onMessage(msg ircmsg.Message) {
	nick := msg.Nick()
	if nick == b.conn.CurrentNick() || len(msg.Params) < 2 {
		return
	}
	_, id := msg.GetTag("msgid")
	_, parent := msg.GetTag("+draft/reply")
	b.Send(core.Inbound{
		Kind:     core.KindPost,
		Origin:   core.Address{Channel: msg.Params[0]},
		NativeID: id,
		ParentID: parent,
		Message:  core.Message{Author: core.Author{ID: nick, Name: nick}, Content: span.Plain(msg.Params[1])},
	})
}

func (b *Bridge) onTyping(msg ircmsg.Message) {
	nick := msg.Nick()
	if nick == b.conn.CurrentNick() || len(msg.Params) < 1 {
		return
	}
	if _, v := msg.GetTag("+typing"); v != "active" {
		return
	}
	b.Send(core.Inbound{
		Kind:    core.KindTyping,
		Origin:  core.Address{Channel: msg.Params[0]},
		Message: core.Message{Author: core.Author{ID: nick, Name: nick}},
	})
}

func (b *Bridge) onRedact(msg ircmsg.Message) {
	if msg.Nick() == b.conn.CurrentNick() || len(msg.Params) < 2 {
		return
	}
	b.Send(core.Inbound{
		Kind:     core.KindDelete,
		Origin:   core.Address{Channel: msg.Params[0]},
		NativeID: msg.Params[1],
	})
}

func (b *Bridge) acked(names ...string) bool {
	caps := b.conn.AcknowledgedCaps()
	for _, n := range names {
		if _, ok := caps[n]; !ok {
			return false
		}
	}
	return true
}

func (b *Bridge) Deliver(ctx context.Context, d core.Delivery) (core.Receipt, error) {
	ch := d.Dest.Channel
	switch d.Event.Kind {
	case core.KindPost:
		replyTo := ""
		if len(d.ParentIDs) > 0 {
			replyTo = d.ParentIDs[0]
		}
		var ids []string
		for line := range strings.SplitSeq(d.Event.Content.Text(), "\n") {
			if line == "" {
				continue
			}
			id, err := b.say(ctx, ch, line, replyTo)
			if err != nil {
				return core.Receipt{}, err
			}
			if id != "" {
				ids = append(ids, id)
			}
			replyTo = "" // anchor the reply to the first line only
		}
		return core.Receipt{IDs: ids}, nil

	case core.KindDelete:
		for _, id := range d.IDs {
			if err := b.conn.Send("REDACT", ch, id); err != nil {
				return core.Receipt{}, err
			}
		}
		return core.Receipt{}, nil

	case core.KindTyping:
		return core.Receipt{}, b.conn.SendWithTags(map[string]string{"+typing": "active"}, "TAGMSG", ch)
	}
	return core.Receipt{}, nil
}

func (b *Bridge) say(ctx context.Context, ch, line, replyTo string) (string, error) {
	select {
	case <-b.throttle.C:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	var tags map[string]string
	if replyTo != "" {
		tags = map[string]string{"+draft/reply": replyTo}
	}
	if !b.acked("labeled-response", "echo-message") {
		return "", b.conn.SendWithTags(tags, "PRIVMSG", ch, line)
	}
	batch, err := b.conn.GetLabeledResponse(tags, "PRIVMSG", ch, line)
	if errors.Is(err, ircevent.NoLabeledResponse) {
		return "", nil // sent, server just gave us no msgid to bind
	}
	if err != nil {
		return "", err
	}
	return msgid(batch), nil
}

func msgid(batch *ircevent.Batch) string {
	if batch == nil {
		return ""
	}
	if batch.Command == "BATCH" {
		for _, item := range batch.Items {
			if id := msgid(item); id != "" {
				return id
			}
		}
		return ""
	}
	_, id := batch.GetTag("msgid")
	return id
}

func (b *Bridge) Close() error {
	b.throttle.Stop()
	b.conn.Quit()
	return nil
}
