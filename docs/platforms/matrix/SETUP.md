# Matrix setup

Matrix connects in one of two modes, and which you can use depends on whether you control the homeserver.

> [!IMPORTANT]
> Encrypted rooms are not supported.

## Normal mode (easy, works anywhere)

The bridge uses a single normal account, on any homeserver you have an account on. Everyone bridged shows up as that one account.

Create an account for the bridge, then get an access token by logging in once:

```sh
curl -X POST https://matrix.example.org/_matrix/client/v3/login \
  -d '{"type": "m.login.password", "identifier": {"type": "m.id.user", "user": "bridgery"}, "password": "the password"}'
```

```toml
[accounts.matrix.main]
homeserver = 'https://matrix.example.org'
token = 'the access_token from the response'
```

> [!NOTE]
> There is no per-user puppeting in normal mode, so enable the [`author.lua`](../../../hooks/examples/30-author.lua) hook to prefix each message with its author, otherwise you can't tell who is talking.

## Puppet mode (needs homeserver admin)

The bridge registers as an **appservice**, which lets each bridged person appear as their own Matrix user with their own name and avatar. This requires **admin access to the homeserver**, because an appservice is registered server-side. Unlike usual appservice setups, bridgery **doesn't** need an open HTTP port for this, it only talks to the homeserver.

Set `puppet_prefix` and leave `token` out:

```toml
[accounts.matrix.main]
homeserver = 'https://matrix.example.org'
puppet_prefix = 'bridge_'
```

On start, bridgery prints a ready-made appservice registration. Save the registration as a file, register it with your homeserver, put the printed `token` into the account, and restart both.

How to register the appservice on various homeserver implementations:

- [**Synapse**](https://element-hq.github.io/synapse/latest/application_services.html)
- [**Tuwunel**](https://matrix-construct.github.io/tuwunel/appservices.html)

The bridge listens as `@<puppet_prefix>bot`, e.g. `@bridge_bot:example.org`, and invites the puppets into rooms itself.

## Rooms

A channel is a room ID or alias. Invite the bridge account (or `@<puppet_prefix>bot`) to rooms that need an invite.

```toml
[[gateways.channels]]
account = 'matrix.main'
channel = '#myroom:example.org'
```

## Files

Downloading Matrix media requires authorization, so for platforms that don't support media uploads themselves (*e.g. IRC*), Matrix files need [media re-hosting](../../MEDIA.md).
