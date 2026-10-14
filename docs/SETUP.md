# Setup

This walks you from nothing to a minimal bridge setup. If you just want the reference, every option is documented inline in [`config.example.toml`](../config.example.toml).

## 1. Install

```sh
git clone https://github.com/f1nniboy/bridgery
cd bridgery
cp config.example.toml config.toml
```

You can run it once `config.toml` is filled in, with either:

```sh
docker compose up -d
# or
go run ./cmd/bridgery
```

## 2. The architecture

Two concepts:

- An **account** is one connection to one platform, e.g. a Discord bot or an IRC account. You define it under `[accounts.<platform>.<label>]`. The `<label>` is yours to pick, and the account is then referred to as `<platform>.<label>` (e.g. `discord.main`).
- A **gateway** is a set of **channels that relay to each other**. Each channel names an account and a channel or room on it. Every message arriving in one channel of a gateway is delivered to all the others (which can be changed too, see below).

The same account can appear in any number of gateways and channels, so one Discord bot can bridge several channels at once.

A channel can also be one-directional with `mode`:

| `mode` | meaning |
| --- | --- |
| `inout` (default) | relays both ways |
| `in` | messages sent here are relayed out, but nothing is relayed into this channel |
| `out` | this channel receives from others, but messages sent here are never relayed |

## 3. A minimal example: Discord & IRC

```toml
[accounts.discord.main]
token = 'Bot MTEyMzQ1Nj...'

[accounts.irc.libera]
server = 'irc.libera.chat'
tls = true
nick = 'mybridge'

[[gateways]]
  [[gateways.channels]]
  account = 'discord.main'
  channel = '123456789012345678'

  [[gateways.channels]]
  account = 'irc.libera'
  channel = '#mychannel'
```

That's a working bridge, but without hooks IRC sees each message with no author (see step 4).

See [docs/platforms](platforms) for what every platform can and can't do, and how to set them up.

## 4. Hooks (recommended)

Out of the box, a message from Discord arrives on IRC as just its text, with no indication of who sent it. **Hooks** are small Lua scripts that fix that (prefix the author's name, quote replies, append file links, add fallback avatars). Example hooks ship in [`hooks/examples/`](../hooks/examples). They are **opt-in**, so nothing loads unless you copy them into a directory and point at it:

```sh
mkdir -p hooks && cp hooks/examples/*.lua hooks/
```

```toml
[hooks]
dir = 'hooks' # or an absolute path, '/hooks' in the Docker image
```

For Docker, uncomment the `./hooks:/hooks` volume in `docker-compose.yml` and set `dir = '/hooks'`. See [HOOKS.md](HOOKS.md), which also covers the editor autocomplete.

## 5. Files across platforms

For example, Discord ingests uploaded files directly, but IRC and XMPP can only share a **link**. A file that already arrives as a public link (a Discord attachment, say) reaches them as-is. Only files that arrive as raw uploads need re-hosting, which needs somewhere to serve from. See [MEDIA.md](MEDIA.md) if you want to set this up.

## 6. Edits, deletes, and replies

These follow a message across platforms automatically, with no setup. An edit or delete on one side updates the bridged copies on the others, and replies stay threaded. To do this the bridge keeps track of recent messages. See [LEDGER.md](LEDGER.md) if you want to change where and how messages are stored.

## 7. Run and verify

Start it, and you should see each account log as `connected`, then messages flowing between the channels. Have fun with your new bridge! 🎉
