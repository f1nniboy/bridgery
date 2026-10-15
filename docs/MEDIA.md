# Media

Platforms differ in how they handle files. Some handle an uploaded file directly (e.g. *Discord*), so bridging an attachment to them just works with no setup. Others don't support file uploads directly (e.g. *IRC*).

Some platforms already hand out a **public link** for an uploaded file. Such a file passes straight through to a link-only platform as-is, with no `[media]` setup needed at all. Re-hosting only applies to files that arrive as raw uploads with no link of their own, and to platforms which need authentication to download media.

To bridge those, bridgery re-hosts the file somewhere with a public URL. The [individual platform pages](platforms) show which platforms accept files directly (the **Media** feature), the rest rely on this.

> [!IMPORTANT]
> On a link-only platform, links are not included in the relayed messages by default. You will need to use the [`files.lua`](../hooks/examples/50-files.lua) hook to display them.

> [!IMPORTANT]
> Without a backend configured, a raw-upload file bridged to a link-only platform is **dropped** (with a log line).

## Configuring

Enable a backend by configuring its `[media.<backend>]` table, as listed in the example configuration. Configure at most one.

Every backend writes each file to `<hash>/<name>` and hands out `base_url/<hash>/<name>`, so `base_url` must be the **root** of whatever publicly serves the storage.
