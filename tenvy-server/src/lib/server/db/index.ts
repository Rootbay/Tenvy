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
        created_at INTEGER NOT NULL,
        updated_at INTEGER NOT NULL
);
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

export const db = drizzle(client, { schema });
