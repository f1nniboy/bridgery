# Media

Platforms differ in how they handle files. Some ingest an uploaded file directly (e.g. *Discord*), so bridging an attachment to them just works with no setup. Others can only carry a **link** (e.g. *IRC*), not the file itself.

Some source platforms already hand out a **public link** for an uploaded file. Such a file passes straight through to a link-only platform as-is, with no `[media]` setup needed at all. Re-hosting only applies to files that arrive as raw uploads with no link of their own.

To bridge those, bridgery re-hosts the file somewhere with a public URL. The [individual platform pages](platforms) show which platforms accept files directly, the rest rely on this.

> [!IMPORTANT]
> On a link-only platform, links are not included in the relayed messages by default. You will need to use the [`files.lua`](../hooks/examples/50-files.lua) hook to display them.

> [!IMPORTANT]
> Without a backend configured, a raw-upload file bridged to a link-only platform is **dropped** (with a log line). Files that already carry a public link bridge fine regardless.

## Configuring

Enable a backend by configuring its `[media.<backend>]` table, as listed in the example configuration; the one you fill in is the one used. Configure at most one, more than one is an error, and none leaves re-hosting disabled.

Every backend writes each file to `<hash>/<name>` and hands out `base_url/<hash>/<name>`, so `base_url` must be the **root** of whatever publicly serves the storage, with no path of its own.

### `local`

Write files to a directory you serve yourself with any web server. For Docker, don't forget to mount a volume for the stored media.

### `s3`

Upload files to any S3-compatible bucket. The bucket must be **publicly readable**, set `base_url` to that publicly available endpoint.
