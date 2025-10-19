import { drizzle } from 'drizzle-orm/better-sqlite3';
import Database from 'better-sqlite3';
import * as schema from './schema';
import { env } from '$env/dynamic/private';

if (!env.DATABASE_URL) throw new Error('DATABASE_URL is not set');

const client = new Database(env.DATABASE_URL);

client.pragma('foreign_keys = ON');

client.exec(
	`BEGIN;
CREATE TABLE IF NOT EXISTS voucher (
        id TEXT PRIMARY KEY NOT NULL,
        code_hash TEXT NOT NULL,
        created_at INTEGER NOT NULL,
        expires_at INTEGER,
        revoked_at INTEGER,
        redeemed_at INTEGER
);
CREATE UNIQUE INDEX IF NOT EXISTS voucher_code_hash_idx ON voucher (code_hash);

CREATE TABLE IF NOT EXISTS user (
        id TEXT PRIMARY KEY NOT NULL,
        created_at INTEGER NOT NULL,
        voucher_id TEXT NOT NULL,
        role TEXT NOT NULL DEFAULT 'operator',
        passkey_registered INTEGER NOT NULL DEFAULT 0,
        current_challenge TEXT,
        challenge_type TEXT,
        challenge_expires_at INTEGER,
        FOREIGN KEY (voucher_id) REFERENCES voucher(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS session (
        id TEXT PRIMARY KEY NOT NULL,
        user_id TEXT NOT NULL,
        expires_at INTEGER,
        created_at INTEGER NOT NULL,
        description TEXT,
        FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS passkey (
        id TEXT PRIMARY KEY NOT NULL,
        user_id TEXT NOT NULL,
        public_key TEXT NOT NULL,
        counter INTEGER NOT NULL DEFAULT 0,
        device_type TEXT,
        backed_up INTEGER NOT NULL DEFAULT 0,
        transports TEXT,
        created_at INTEGER NOT NULL,
        last_used_at INTEGER,
        FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS recovery_code (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id TEXT NOT NULL,
        code_hash TEXT NOT NULL,
        created_at INTEGER NOT NULL,
        consumed_at INTEGER,
        FOREIGN KEY (user_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS plugin (
        id TEXT PRIMARY KEY NOT NULL,
        status TEXT NOT NULL DEFAULT 'active',
        enabled INTEGER NOT NULL DEFAULT 1,
        auto_update INTEGER NOT NULL DEFAULT 0,
        installations INTEGER NOT NULL DEFAULT 0,
        manual_targets INTEGER NOT NULL DEFAULT 0,
        auto_targets INTEGER NOT NULL DEFAULT 0,
        default_delivery_mode TEXT NOT NULL DEFAULT 'manual',
        allow_manual_push INTEGER NOT NULL DEFAULT 1,
        allow_auto_sync INTEGER NOT NULL DEFAULT 0,
        last_manual_push_at INTEGER,
        last_auto_sync_at INTEGER,
        last_deployed_at INTEGER,
        last_checked_at INTEGER,
        created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
        updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);

CREATE TABLE IF NOT EXISTS agent (
        id TEXT PRIMARY KEY NOT NULL,
        key_hash TEXT NOT NULL,
        metadata TEXT NOT NULL,
        status TEXT NOT NULL DEFAULT 'offline',
        connected_at INTEGER NOT NULL,
        last_seen INTEGER NOT NULL,
        metrics TEXT,
        config TEXT NOT NULL,
        fingerprint TEXT NOT NULL,
        created_at INTEGER NOT NULL,
        updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS agent_fingerprint_idx ON agent (fingerprint);

CREATE TABLE IF NOT EXISTS agent_note (
        agent_id TEXT NOT NULL,
        note_id TEXT NOT NULL,
        ciphertext TEXT NOT NULL,
        nonce TEXT NOT NULL,
        digest TEXT NOT NULL,
        version INTEGER NOT NULL DEFAULT 1,
        updated_at INTEGER NOT NULL,
        PRIMARY KEY (agent_id, note_id),
        FOREIGN KEY (agent_id) REFERENCES agent(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS agent_command (
        id TEXT PRIMARY KEY NOT NULL,
        agent_id TEXT NOT NULL,
        name TEXT NOT NULL,
        payload TEXT NOT NULL,
        created_at INTEGER NOT NULL,
        FOREIGN KEY (agent_id) REFERENCES agent(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS agent_result (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        agent_id TEXT NOT NULL,
        command_id TEXT NOT NULL,
        success INTEGER NOT NULL DEFAULT 1,
        output TEXT,
        error TEXT,
        completed_at INTEGER NOT NULL,
        FOREIGN KEY (agent_id) REFERENCES agent(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS agent_result_command_idx ON agent_result (agent_id, command_id);

CREATE TABLE IF NOT EXISTS audit_event (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        command_id TEXT NOT NULL,
        agent_id TEXT NOT NULL,
        operator_id TEXT,
        command_name TEXT NOT NULL,
        payload_hash TEXT NOT NULL,
        queued_at INTEGER NOT NULL,
        executed_at INTEGER,
        result TEXT,
        FOREIGN KEY (operator_id) REFERENCES user(id) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS audit_event_command_idx ON audit_event (command_id);
CREATE INDEX IF NOT EXISTS audit_event_agent_idx ON audit_event (agent_id);
COMMIT;`
);

const ensureColumn = (table: string, column: string, ddl: string) => {
	const columns = client.prepare(`PRAGMA table_info(${table})`).all() as Array<{ name: string }>;
	const exists = columns.some((entry) => entry.name === column);
	if (!exists) {
		client.exec(`ALTER TABLE ${table} ADD COLUMN ${ddl}`);
	}
};

ensureColumn('passkey', 'last_used_at', 'last_used_at INTEGER');
ensureColumn('user', 'role', "role TEXT NOT NULL DEFAULT 'operator'");

export const db = drizzle(client, { schema });
