package matrix

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/f1nniboy/bridgery/internal/config"
	"github.com/f1nniboy/bridgery/internal/core"
)

type Bridge struct {
	*core.Common `toml:"-"`

	Homeserver   string        `toml:"homeserver" comment:"homeserver URL"`
	Token        config.Secret `toml:"token" comment:"access token (normal mode) or appservice as_token (puppet mode)\nin puppet mode, leave empty once to print an appservice registration"`
	PuppetPrefix string        `toml:"puppet_prefix,commented" comment:"puppet localpart prefix, enables puppet mode\nmust match the appservice registration's user namespace"`

	client *mautrix.Client

	syncStore  mautrix.SyncStore
	stateStore mautrix.StateStore

	roomMu sync.Mutex
	rooms  map[id.RoomID]*room

	puppetMu sync.Mutex
	puppets  map[id.UserID]*puppet
}

type room struct {
	id     id.RoomID
	name   string
	typing []id.UserID
}

func (b *Bridge) Name() string { return "Matrix" }

func (b *Bridge) Example() core.Bridge {
	return &Bridge{Homeserver: "https://matrix.example.org", Token: "${MATRIX_TOKEN}", PuppetPrefix: "bridge_"}
}

func (b *Bridge) Caps() core.CapSet {
	if b.PuppetPrefix != "" {
		return core.Caps(core.CapReply, core.CapEdit, core.CapDelete, core.CapTyping, core.CapMedia, core.CapPuppet)
	}
	return core.Caps(core.CapReply, core.CapEdit, core.CapDelete, core.CapTyping, core.CapMedia)
}

var errEncrypted = fmt.Errorf("room is encrypted and we don't support encrypted rooms: %w", core.ErrPermanent)

func (b *Bridge) Open(ctx context.Context) error {
	if b.Homeserver == "" {
		return fmt.Errorf("homeserver is required: %w", core.ErrPermanent)
	}
	if b.Token == "" {
		if b.PuppetPrefix != "" {
			b.printRegistration()
			return fmt.Errorf("register the appservice printed above, then set token: %w", core.ErrPermanent)
		}
		return fmt.Errorf("token is required: %w", core.ErrPermanent)
	}
	if b.syncStore == nil {
		b.syncStore, b.stateStore = mautrix.NewMemorySyncStore(), mautrix.NewMemoryStateStore()
		b.rooms = make(map[id.RoomID]*room)
		b.puppets = make(map[id.UserID]*puppet)
	}
	cli, err := b.newClient("")
	if err != nil {
		return fmt.Errorf("%w: %w", err, core.ErrPermanent)
	}
	who, err := cli.Whoami(ctx)
	if errors.Is(err, mautrix.MUnknownToken) {
		return fmt.Errorf("%w: %w", err, core.ErrPermanent)
	}
	if err != nil {
		return err
	}
	cli.UserID = who.UserID
	if b.PuppetPrefix != "" {
		cli.UserID = id.NewUserID(b.PuppetPrefix+"bot", who.UserID.Homeserver())
		cli.SetAppServiceUserID = true
		if err := register(ctx, cli); err != nil {
			return err
		}
	}
	cli.Store = b.syncStore

	s := &syncer{DefaultSyncer: mautrix.NewDefaultSyncer(), log: b.Log}
	s.OnSync(b.skipHistory)
	s.OnSync(b.onLeave)
	s.OnEvent(cli.StateStoreSyncHandler)
	s.OnEventType(event.EventMessage, b.onMessage)
	s.OnEventType(event.EventRedaction, b.onRedaction)
	s.OnEventType(event.EphemeralEventTyping, b.onTyping)
	s.OnEventType(event.StateEncryption, b.onEncryption)
	cli.Syncer = s

	b.client = cli

	go b.sync(ctx, cli)
	return nil
}

func (b *Bridge) newClient(user id.UserID) (*mautrix.Client, error) {
	cli, err := mautrix.NewClient(b.Homeserver, user, string(b.Token))
	if err != nil {
		return nil, err
	}
	cli.Log = zerolog.Nop()
	cli.DefaultHTTPRetries = 3
	cli.StateStore = b.stateStore
	return cli, nil
}

func (b *Bridge) sync(ctx context.Context, cli *mautrix.Client) {
	err := cli.SyncWithContext(ctx)
	if ctx.Err() != nil {
		return
	}
	b.Log.Warn("matrix sync stopped", "err", err)
	b.Report(ctx, core.Status{Event: core.Disconnected})
}

type syncer struct {
	*mautrix.DefaultSyncer
	log *slog.Logger
}

func (s *syncer) OnFailedSync(res *mautrix.RespSync, err error) (time.Duration, error) {
	s.log.Warn("matrix sync failed", "err", err)
	return s.DefaultSyncer.OnFailedSync(res, err)
}

func (b *Bridge) Join(ctx context.Context, ch core.ChannelSpec) error {
	resp, err := b.client.JoinRoom(ctx, ch.Name, nil)
	if err != nil {
		return err
	}
	var enc event.EncryptionEventContent
	err = b.client.StateEvent(ctx, resp.RoomID, event.StateEncryption, "", &enc)
	switch {
	case err == nil:
		return errEncrypted
	case !errors.Is(err, mautrix.MNotFound):
		return err
	}
	b.addRoom(&room{id: resp.RoomID, name: ch.Name})
	return nil
}

func (b *Bridge) Close() error { return nil }

func (b *Bridge) addRoom(r *room) {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	b.rooms[r.id] = r
}

func (b *Bridge) dropRoom(r *room) {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	delete(b.rooms, r.id)
}

func (b *Bridge) roomByID(rid id.RoomID) (*room, bool) {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	r, ok := b.rooms[rid]
	return r, ok
}

func (b *Bridge) roomByName(name string) (*room, bool) {
	b.roomMu.Lock()
	defer b.roomMu.Unlock()
	for _, r := range b.rooms {
		if r.name == name {
			return r, true
		}
	}
	return nil, false
}

func (b *Bridge) Deliver(ctx context.Context, d core.Delivery) (core.Receipt, error) {
	r, ok := b.roomByName(d.Dest.Channel)
	if !ok {
		return core.Receipt{}, fmt.Errorf("not in room %s", d.Dest.Channel)
	}

	switch d.Event.Kind {
	case core.KindPost:
		cli, err := b.sender(ctx, r, d.Event)
		if err != nil {
			return core.Receipt{}, err
		}
		return b.post(ctx, cli, r, d)

	case core.KindEdit:
		if len(d.IDs) == 0 {
			return core.Receipt{}, nil
		}
		cli, orig, err := b.original(ctx, r, d.IDs[0])
		if err != nil || orig.Content.Raw["msgtype"] != string(event.MsgText) {
			return core.Receipt{}, err
		}
		c := &event.MessageEventContent{MsgType: event.MsgText, Body: d.Event.Content.Text(), Mentions: &event.Mentions{}}
		c.SetEdit(orig.ID)
		_, err = cli.SendMessageEvent(ctx, r.id, event.EventMessage, c)
		return core.Receipt{}, err

	case core.KindDelete:
		for _, eid := range d.IDs {
			cli, _, err := b.original(ctx, r, eid)
			if err == nil {
				_, err = cli.RedactEvent(ctx, r.id, id.EventID(eid))
			}
			if err != nil {
				return core.Receipt{}, err
			}
		}
		return core.Receipt{}, nil

	case core.KindTyping:
		cli, err := b.sender(ctx, r, d.Event)
		if err == nil {
			_, err = cli.UserTyping(ctx, r.id, true, 10*time.Second)
		}
		return core.Receipt{}, err
	}
	return core.Receipt{}, nil
}

func (b *Bridge) post(ctx context.Context, cli *mautrix.Client, r *room, d core.Delivery) (core.Receipt, error) {
	var contents []*event.MessageEventContent
	if text := d.Event.Content.Text(); text != "" {
		contents = append(contents, &event.MessageEventContent{MsgType: event.MsgText, Body: text, Mentions: &event.Mentions{}})
	}
	for _, f := range d.Event.Files {
		c, err := upload(ctx, cli, f)
		if err != nil {
			b.Log.Warn("matrix upload, dropping file", "file", f.Name, "err", err)
			continue
		}
		contents = append(contents, c)
	}

	if len(contents) > 0 && len(d.ParentIDs) > 0 {
		contents[0].RelatesTo = (&event.RelatesTo{}).SetReplyTo(id.EventID(d.ParentIDs[0]))
	}

	var ids []string
	var err error
	for _, c := range contents {
		var resp *mautrix.RespSendEvent
		if resp, err = cli.SendMessageEvent(ctx, r.id, event.EventMessage, c); err != nil {
			b.Log.Warn("matrix send", "room", r.name, "err", err)
			continue
		}
		ids = append(ids, resp.EventID.String())
	}
	if len(ids) == 0 {
		return core.Receipt{}, err
	}
	return core.Receipt{IDs: ids}, nil
}
