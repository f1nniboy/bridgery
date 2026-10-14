<!-- This file was generated. Do not edit. -->

# IRC

- [Setup](SETUP.md)

## Features

| Feature | Status |
|---|:-:|
| Reply | ✅ |
| Edit | ❌ |
| Delete | ✅ |
| Typing | ✅ |
| Puppet | ❌ |
| Media | ❌ |

## Configuration

```toml
[accounts.irc.myirc]
# host[:port]
server = 'irc.libera.chat'
# connect with TLS (default port 6697, else 6667)
tls = true
# nickname
nick = 'mybot'
# username, defaults to nick
# user = ''
# real name, defaults to nick
# realname = ''
# SASL account for authentication
# sasl_user = ''
# SASL password
# sasl_pass = ''
# server password (PASS)
# password = ''
```
