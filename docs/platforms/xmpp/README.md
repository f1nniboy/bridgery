<!-- This file was generated. Do not edit. -->

# XMPP

- [Setup](SETUP.md)

## Features

| Feature | Status |
|---|:-:|
| Reply | ✅ |
| Edit | ❌ |
| Delete | ✅ |
| Typing | ✅ |
| Puppet | ✅ |
| Media | ❌ |

## Configuration

```toml
[accounts.xmpp.myxmpp]
# component domain (puppet mode) or user@domain (normal mode)
jid = 'bridger@example.org'
# component secret (puppet mode) or account password (normal mode)
password = '123'
# connect address host:port, required for component, else SRV lookup
# server = 'localhost:5347'
# bot nickname in rooms, defaults to jid localpart
# nick = ''
```
