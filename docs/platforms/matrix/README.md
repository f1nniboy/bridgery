<!-- This file was generated. Do not edit. -->

# Matrix

- [Setup](SETUP.md)

## Features

| Feature | Status |
|---|:-:|
| Reply | ✅ |
| Edit | ✅ |
| Delete | ✅ |
| Typing | ✅ |
| Puppet | ✅ |
| Media | ✅ |

## Configuration

```toml
[accounts.matrix.mymatrix]
# homeserver URL
homeserver = 'https://matrix.example.org'
# access token (normal mode) or appservice as_token (puppet mode)
# in puppet mode, leave empty once to print an appservice registration
token = '${MATRIX_TOKEN}'
# puppet localpart prefix, enables puppet mode
# must match the appservice registration's user namespace
# puppet_prefix = 'bridge_'
```
