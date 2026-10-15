package matrix

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/f1nniboy/bridgery/internal/core"
)

type puppet struct {
	client *mautrix.Client

	mu     sync.Mutex
	name   string
	avatar string // source avatar key
}

func (b *Bridge) sender(ctx context.Context, r *room, ev core.Event) (*mautrix.Client, error) {
	if b.PuppetPrefix == "" {
		return b.client, nil
	}
	uid := id.NewUserID(b.PuppetPrefix+ev.PuppetID(), b.client.UserID.Homeserver())
	p, err := b.puppet(ctx, uid)
	if err != nil {
		return nil, err
	}
	if err := b.enter(ctx, r, p.client); err != nil {
		return nil, err
	}
	p.profile(ctx, ev.Author, b.Log)
	return p.client, nil
}

// the client that sent eid
func (b *Bridge) original(ctx context.Context, r *room, eid string) (*mautrix.Client, *event.Event, error) {
	ev, err := b.client.GetEvent(ctx, r.id, id.EventID(eid))
	if err != nil {
		return nil, nil, err
	}
	if ev.Sender == b.client.UserID {
		return b.client, ev, nil
	}
	p, err := b.puppet(ctx, ev.Sender)
	if err != nil {
		return nil, nil, err
	}
	return p.client, ev, nil
}

func (b *Bridge) puppet(ctx context.Context, uid id.UserID) (*puppet, error) {
	b.puppetMu.Lock()
	p, ok := b.puppets[uid]
	b.puppetMu.Unlock()
	if ok {
		return p, nil
	}

	cli, err := b.newClient(uid)
	if err != nil {
		return nil, err
	}
	cli.SetAppServiceUserID = true
	if err := register(ctx, cli); err != nil {
		return nil, err
	}

	b.puppetMu.Lock()
	defer b.puppetMu.Unlock()
	if p, ok := b.puppets[uid]; ok {
		return p, nil
	}
	p = &puppet{client: cli}
	b.puppets[uid] = p
	return p, nil
}

func (b *Bridge) enter(ctx context.Context, r *room, cli *mautrix.Client) error {
	if b.stateStore.IsInRoom(ctx, r.id, cli.UserID) {
		return nil
	}
	_, err := cli.JoinRoomByID(ctx, r.id)
	if !errors.Is(err, mautrix.MForbidden) {
		return err
	}
	if _, err := b.client.InviteUser(ctx, r.id, &mautrix.ReqInviteUser{UserID: cli.UserID}); err != nil {
		return err
	}
	_, err = cli.JoinRoomByID(ctx, r.id)
	return err
}

func (p *puppet) profile(ctx context.Context, a core.Author, log *slog.Logger) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if name := a.Display(); name != p.name {
		if err := p.client.SetDisplayName(ctx, name); err != nil {
			log.Warn("matrix puppet name", "user", p.client.UserID, "err", err)
		} else {
			p.name = name
		}
	}

	key := cmp.Or(a.Avatar.Key, a.Avatar.URL)
	if key == p.avatar {
		return
	}
	var uri id.ContentURI
	if !a.Avatar.Empty() {
		c, err := upload(ctx, p.client, a.Avatar)
		if err != nil {
			log.Warn("matrix puppet avatar", "user", p.client.UserID, "err", err)
			return
		}
		uri = c.URL.ParseOrIgnore()
	}
	if err := p.client.SetAvatarURL(ctx, uri); err != nil {
		log.Warn("matrix puppet avatar", "user", p.client.UserID, "err", err)
		return
	}
	p.avatar = key
}

// https://spec.matrix.org/v1.16/application-service-api/#registration
func (b *Bridge) printRegistration() {
	fmt.Fprintf(os.Stderr, `
Matrix puppet mode needs an appservice registration. Save this as a file,
register it with your homeserver, then set the token in the config to '%[1]s' and restart.

id: bridgery
url: null
as_token: %[1]s
hs_token: %[2]s
sender_localpart: %[3]ssender
rate_limited: false
namespaces:
  users:
    - exclusive: true
      regex: '@%[4]s.*'

`, rand.Text(), rand.Text(), b.PuppetPrefix, regexp.QuoteMeta(b.PuppetPrefix))
}

func register(ctx context.Context, cli *mautrix.Client) error {
	_, _, err := cli.Register(ctx, &mautrix.ReqRegister[any]{Type: mautrix.AuthTypeAppservice, Username: cli.UserID.Localpart(), InhibitLogin: true})
	if errors.Is(err, mautrix.MUserInUse) {
		return nil
	}
	return err
}
