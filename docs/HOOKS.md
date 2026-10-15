# Hooks

Hooks are small **Lua** scripts that rewrite, filter, or drop messages as they pass through.

They are how bridgery does *presentation*: the core only moves structured messages around, and hooks decide how they are displayed on each platform.

## Why?

This is a pretty big difference from most bridges. While others allow you to change formatting using config options (a nick-format string, a pile of toggles), bridgery keeps all of it in hooks you can read and edit, so you're never stuck with an opinionated default.

The tradeoff is that first-time setup is a little more involved: a fresh bridge does nothing to your messages until you enable hooks, so copying the examples in is a normal, almost must-have, step rather than getting a pre-configured look out of the box.

## Enabling them

Hooks are **opt-in** (*although very much recommended*). A directory of example hooks ships in [`hooks/examples/`](../hooks/examples). Copy the ones you want into your own hooks directory, then point at it:

```sh
mkdir -p hooks && cp hooks/examples/*.lua hooks/
```

```toml
[hooks]
dir = 'hooks'
```

In the Docker image, mount it and use its path: uncomment the mount line in `docker-compose.yml` and set the above config option.

## Editor autocomplete

Open `hooks/` in an editor with the Lua language server for autocomplete and type checking.

## Load order

Files load in filename order, so the `10-`/`20-`/`30-` prefixes on the examples control which runs first.

## Writing your own

A hook registers a handler for one of two phases:

### `receive`

Runs once, before the message is stored or routed, and without a destination. This is where you **filter**: drop spam, block an author, or reject anything you don't want bridged, by returning `false`. Returning anything else (or nothing) keeps it.

Changes you make here are canonical, they get stored and reach every platform, and a dropped message is never stored, so a later edit or delete of it resolves to nothing.

```lua
on("receive", function(ev)
    if ev.author.name == "unwanted" then
        return false -- drops the event
    end
end)
```

### `deliver`

Runs once per destination, after storage, with the target platform in `dst`. This is where you handle **presentation**: change how one platform renders a message without touching the stored copy or the other destinations. Append the author's name where puppets aren't supported, quote a reply inline, include attached file's URLs, etc.

Each destination gets its own copy, so changes stay local to it, and returning `false` drops delivery to just that platform.

```lua
on("deliver", function(ev, dst)
    if dst.platform == "irc" then
        ev.author.name = ev.author:display() .. " [" .. ev.platform .. "]"
        ev:append(" (foo)")
    end
end)
```

## Limitations

Hooks only rewrite or drop messages. They can't send new messages or reply to commands, so bridgery is not a generic framework.
