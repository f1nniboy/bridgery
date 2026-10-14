# IRC setup

# Message authors

IRC has no per-user puppeting, so everyone bridged shares the bot's nick.

Enable the [`author.lua`](../../../hooks/examples/30-author.lua) hook to prefix each message with its author, otherwise you can't tell who is talking.

## Registered nick

Many networks require a registered nick to speak in channels. Register the nick with the network's `NickServ`, then set `sasl_user` and `sasl_pass` on the account to authenticate over SASL.
