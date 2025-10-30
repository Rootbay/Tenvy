-- Track plugin registry lifecycle events and manifest metadata.
CREATE TABLE IF NOT EXISTS plugin_registry_entry (
        id TEXT PRIMARY KEY,
        plugin_id TEXT NOT NULL,
        version TEXT NOT NULL,
        manifest TEXT NOT NULL,
        manifest_digest TEXT NOT NULL,
        artifact_hash TEXT,
        artifact_size_bytes INTEGER,
        metadata TEXT,
        approval_status TEXT NOT NULL DEFAULT 'pending',
        published_by TEXT REFERENCES user(id) ON DELETE SET NULL,
        published_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
        approved_by TEXT REFERENCES user(id) ON DELETE SET NULL,
        approved_at INTEGER,
        approval_note TEXT,
        revoked_by TEXT REFERENCES user(id) ON DELETE SET NULL,
        revoked_at INTEGER,
        revocation_reason TEXT,
        created_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER)),
        updated_at INTEGER NOT NULL DEFAULT (CAST(strftime('%s','now') AS INTEGER))
);

CREATE UNIQUE INDEX IF NOT EXISTS plugin_registry_entry_plugin_version_idx
        ON plugin_registry_entry (plugin_id, version);

CREATE INDEX IF NOT EXISTS plugin_registry_entry_status_idx
        ON plugin_registry_entry (approval_status);
