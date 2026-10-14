# Discord setup

## 1. Create the bot

> [!IMPORTANT]
> - **Message Content Intent** must be **on**, or the bot receives every message as *empty* and nothing bridges out of Discord.

1. Go to the [Discord Developer Portal](https://discord.com/developers/applications) -> **New Application**, name it, **Create**.
2. Open the **Bot** tab.
3. Under **Privileged Gateway Intents**, enable **Message Content Intent** and save.
4. Click **Reset Token** -> **Copy**.

Put it in the account's `token`, keeping the `Bot ` prefix.

## 2. Invite it

1. Under **OAuth2 -> URL Generator**, tick the `bot` scope.
2. In the permissions that appear, tick **Manage Webhooks**.
3. Copy the generated URL at the bottom, open it, pick your server, and **Authorize**.

bridgery creates and reuses a webhook per channel on its own, so you never have to manage webhooks by hand.

## 3. Add the channel to the gateway

```toml
[[gateways.channels]]
account = 'discord.main'
channel = '123456789012345678'
```
