import { describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { drizzle } from 'drizzle-orm/better-sqlite3';
import { join } from 'node:path';
import { createPluginRepository } from '../src/lib/data/plugins.js';
import { createPluginRuntimeStore } from '../src/lib/server/plugins/runtime-store.js';
import { plugin as pluginTable } from '../src/lib/server/db/schema.js';

const manifestDirectory = join(process.cwd(), 'resources/plugin-manifests');

const PLUGIN_TABLE_DDL = `
CREATE TABLE plugin (
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
`;

const createRepository = () => {
	const sqlite = new Database(':memory:');
	sqlite.exec(PLUGIN_TABLE_DDL);
	const drizzleDb = drizzle(sqlite, { schema: { plugin: pluginTable } });
	const runtimeStore = createPluginRuntimeStore(drizzleDb);
	const repository = createPluginRepository({ runtimeStore, directory: manifestDirectory });
	return { repository };
};

describe('plugin repository', () => {
	it('derives plugin views from manifests and runtime state', async () => {
		const { repository } = createRepository();
		const plugins = await repository.list();

		expect(plugins.length).toBeGreaterThan(0);
		const clipboard = plugins.find((plugin) => plugin.id === 'clipboard-sync');
		expect(clipboard).toBeDefined();
		expect(clipboard?.artifact).toContain('clipboard');
		expect(clipboard?.distribution.defaultMode).toBeTypeOf('string');
	});

	it('persists updates across reads', async () => {
		const { repository } = createRepository();

		await repository.update('clipboard-sync', {
			enabled: false,
			status: 'disabled'
		});

		const disabled = await repository.get('clipboard-sync');
		expect(disabled.enabled).toBe(false);
		expect(disabled.status).toBe('disabled');

		await repository.update('clipboard-sync', {
			enabled: true,
			status: 'active',
			distribution: {
				allowAutoSync: true,
				autoTargets: 12
			}
		});

		const reenabled = await repository.get('clipboard-sync');
		expect(reenabled.enabled).toBe(true);
		expect(reenabled.status).toBe('active');
		expect(reenabled.distribution.allowAutoSync).toBe(true);
		expect(reenabled.distribution.autoTargets).toBe(12);
	});
});
