# XMPP setup

XMPP connects in one of two modes, and which you can use depends on whether you control the server.

## Client mode (easy, works anywhere)

The bridge logs in as a single normal account and works against any server you have an account on. Everyone bridged shows up as that one account.

```toml
[accounts.xmpp.main]
jid = 'bridge@example.org' # a normal account
password = 'the account password'
# nick = '' # nickname in rooms, defaults to the jid localpart
```

> [!NOTE]
> There is no per-user puppeting in client mode, so enable the [`author.lua`](../../../hooks/examples/30-author.lua) hook to prefix each message with its author, otherwise you can't tell who is talking (like IRC).

## Component mode (puppets, needs server admin)

The bridge registers as an XMPP **component**, which lets each bridged person appear as their own occupant with their own name and avatar (pretty much like Discord webhooks). This requires **admin access to the XMPP server**, because a component is configured server-side.

```toml
[accounts.xmpp.main]
jid = 'bridge.example.org' # a subdomain for the component, not a user account
password = 'the component secret'
server = 'localhost:5347' # the server's component port
```

You will also have to register it on the server:

- [**Prosody**](https://prosody.im/doc/components#adding_an_external_component)
- [**ejabberd**](https://docs.ejabberd.im/admin/configuration/listen/#ejabberd_service)

Restart the server after adding it, then point `server` at the component port (**5347** by default).

## Rooms

A channel is just a MUC room JID:

```toml
[[gateways.channels]]
account = 'xmpp.main'
channel = 'myroom@conference.example.org'
# key = '' # room password, if the room has one
```
