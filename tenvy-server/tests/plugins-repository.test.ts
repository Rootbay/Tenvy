import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { drizzle } from 'drizzle-orm/better-sqlite3';
import { join } from 'node:path';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { createPluginRepository } from '../src/lib/data/plugins.js';
import { createPluginRuntimeStore } from '../src/lib/server/plugins/runtime-store.js';
import { plugin as pluginTable } from '../src/lib/server/db/schema.js';

const manifestFixture = {
	id: 'clipboard-sync',
	name: 'Clipboard Sync',
	version: '1.4.2',
	description: 'Synchronize clipboard activity across operator sessions.',
	entry: 'clipboard-sync.dll',
	author: 'Tenvy Labs',
	repositoryUrl: 'https://github.com/rootbay/tenvy-clipboard-sync',
	license: {
		spdxId: 'MIT',
		name: 'MIT License',
		url: 'https://opensource.org/license/mit'
	},
	categories: ['collection'],
	capabilities: [
		{
			name: 'clipboard.capture',
			module: 'clipboard',
			description: 'Capture remote clipboard history and relay updates in real time.'
		}
	],
	requirements: {
		minAgentVersion: '1.2.0',
		platforms: ['windows'],
		architectures: ['x86_64'],
		requiredModules: ['clipboard']
	},
	distribution: {
		defaultMode: 'automatic',
		autoUpdate: true,
		signature: { type: 'none' }
	},
	package: {
		artifact: 'clipboard-sync-1.4.2.dll',
		sizeBytes: 18_743_296
	}
};

const PLUGIN_TABLE_DDL = `
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
        signature_status TEXT NOT NULL DEFAULT 'unsigned',
        signature_trusted INTEGER NOT NULL DEFAULT 0,
        signature_type TEXT NOT NULL DEFAULT 'none',
        signature_hash TEXT,
        signature_signer TEXT,
        signature_public_key TEXT,
        signature_checked_at INTEGER,
        signature_signed_at INTEGER,
        signature_error TEXT,
        signature_error_code TEXT,
        signature_chain TEXT,
        approval_status TEXT NOT NULL DEFAULT 'pending',
        approved_at INTEGER,
        approval_note TEXT,
        created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
        updated_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now'))
);
`;

let tempDir: string;
let dbPath: string;
let manifestDir: string;

const openRepository = () => {
	const sqlite = new Database(dbPath);
	sqlite.exec(PLUGIN_TABLE_DDL);
	const drizzleDb = drizzle(sqlite, { schema: { plugin: pluginTable } });
	const runtimeStore = createPluginRuntimeStore(drizzleDb);
	const repository = createPluginRepository({ runtimeStore, directory: manifestDir });
	return { repository, runtimeStore, sqlite };
};

beforeEach(() => {
	tempDir = mkdtempSync(join(tmpdir(), 'tenvy-plugin-repo-'));
	dbPath = join(tempDir, 'runtime.sqlite');
	manifestDir = join(tempDir, 'manifests');
	mkdirSync(manifestDir, { recursive: true });
	writeFileSync(join(manifestDir, `${manifestFixture.id}.json`), JSON.stringify(manifestFixture));
});

afterEach(() => {
	rmSync(tempDir, { recursive: true, force: true });
});

describe('plugin repository', () => {
	it('derives plugin views from manifests and runtime state', async () => {
		const { repository, sqlite } = openRepository();

		try {
			const plugins = await repository.list();
			expect(plugins.length).toBeGreaterThan(0);

			const clipboard = plugins.find((plugin) => plugin.id === 'clipboard-sync');
			expect(clipboard?.name).toBe('Clipboard Sync');
			expect(clipboard?.artifact).toContain('clipboard');
			expect(clipboard?.distribution.defaultMode).toBe('automatic');
			expect(clipboard?.distribution.allowAutoSync).toBe(true);
			expect(clipboard?.requiredModules.map((module) => module.id)).toContain('clipboard');
			expect(clipboard?.signature.status).toBe('unsigned');
			expect(clipboard?.signature.trusted).toBe(false);
		} finally {
			sqlite.close();
		}
	});

	it('persists runtime updates across repository instances', async () => {
		const first = openRepository();
		const timestamp = new Date('2024-05-18T10:30:00.000Z');

		try {
			await first.repository.update('clipboard-sync', {
				status: 'disabled',
				enabled: false,
				autoUpdate: false,
				lastDeployedAt: timestamp,
				lastCheckedAt: timestamp,
				approvalStatus: 'approved',
				approvedAt: timestamp,
				approvalNote: 'ship it',
				distribution: {
					defaultMode: 'manual',
					allowManualPush: false,
					allowAutoSync: false,
					manualTargets: 3,
					autoTargets: 1,
					lastManualPushAt: timestamp,
					lastAutoSyncAt: timestamp
				}
			});
		} finally {
			first.sqlite.close();
		}

		const second = openRepository();

		try {
			const plugin = await second.repository.get('clipboard-sync');
			expect(plugin.status).toBe('disabled');
			expect(plugin.enabled).toBe(false);
			expect(plugin.distribution.allowAutoSync).toBe(false);
			expect(plugin.distribution.allowManualPush).toBe(false);
			expect(plugin.distribution.manualTargets).toBe(3);
			expect(plugin.distribution.autoTargets).toBe(1);
			expect(plugin.approvalStatus).toBe('approved');
			expect(plugin.approvedAt).toBe(timestamp.toISOString());

			const runtimeRow = await second.runtimeStore.find('clipboard-sync');
			expect(runtimeRow?.lastManualPushAt?.toISOString()).toBe(timestamp.toISOString());
			expect(runtimeRow?.lastAutoSyncAt?.toISOString()).toBe(timestamp.toISOString());
			expect(runtimeRow?.lastDeployedAt?.toISOString()).toBe(timestamp.toISOString());
			expect(runtimeRow?.lastCheckedAt?.toISOString()).toBe(timestamp.toISOString());
			expect(runtimeRow?.approvalNote).toBe('ship it');
		} finally {
			second.sqlite.close();
		}
	});
});
