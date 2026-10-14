# Ledger

When a message is bridged, each platform stores its own copy with its own ID. For a later **edit**, **delete**, or **reply** to follow that message to the other platforms, the bridge has to know those copies are all the same message. The **ledger** is what remembers that.

It gives each bridged message a shared canonical identifier and tracks which copy on each platform belongs to it. When an event comes in, the ledger matches it back and applies the same change to the copies on the other platforms.

> [!IMPORTANT]
> If a message is no longer in the ledger, an edit, delete, or reply to it simply does nothing on the other platforms, the original relay stays as it is. The message itself is unaffected. How long a message stays reachable depends on the backend.

## Configuring

The ledger works with no configuration by default. Configure it only to choose or tune a backend, via its `[ledger.<backend>]` table, as listed in the example configuration.

### `memory`

The default. Keeps recent messages in memory, evicting the oldest once the limit is reached. It is **not persisted**, it starts empty on every restart.

`max_size` defaults to 10000 messages. Setting it to `0` makes it **unbounded**, which keeps every message forever, so only do that on small bridges.
