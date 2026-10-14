CREATE TABLE messages (
    id         TEXT PRIMARY KEY,
    data       TEXT,
    updated_at INTEGER NOT NULL
) STRICT;

CREATE INDEX messages_updated_at ON messages (updated_at);

CREATE TABLE bindings (
    message_id TEXT NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    account    TEXT NOT NULL,
    channel    TEXT NOT NULL,
    native_id  TEXT NOT NULL,
    position   INTEGER NOT NULL,
    PRIMARY KEY (account, channel, native_id)
) STRICT;

CREATE INDEX bindings_message_id ON bindings (message_id, account, channel, position);
