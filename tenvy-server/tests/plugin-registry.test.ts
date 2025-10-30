import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { PluginManifest } from '../../shared/types/plugin-manifest.js';

vi.mock('$env/dynamic/private', () => ({ env: { DATABASE_URL: ':memory:' } }));

let db: (typeof import('$lib/server/db/index.js'))['db'];
let pluginTable: (typeof import('$lib/server/db/schema.js'))['plugin'];
let registryTable: (typeof import('$lib/server/db/schema.js'))['pluginRegistryEntry'];
let voucherTable: (typeof import('$lib/server/db/schema.js'))['voucher'];
let userTable: (typeof import('$lib/server/db/schema.js'))['user'];
let createPluginRegistryStore: (typeof import('$lib/server/plugins/registry-store.js'))['createPluginRegistryStore'];

beforeAll(async () => {
        ({ db } = await import('$lib/server/db/index.js'));
        ({ plugin: pluginTable, pluginRegistryEntry: registryTable, voucher: voucherTable, user: userTable } =
                await import('$lib/server/db/schema.js'));
        ({ createPluginRegistryStore } = await import('$lib/server/plugins/registry-store.js'));

        const voucherId = 'registry-test-voucher';
        const timestamp = new Date();
        await db
                .insert(voucherTable)
                .values({ id: voucherId, codeHash: 'registry-code', createdAt: timestamp })
                .onConflictDoNothing();
        await db
                .insert(userTable)
                .values({ id: 'admin-1', voucherId, role: 'admin', createdAt: timestamp })
                .onConflictDoNothing();
});

const baseManifest: PluginManifest = {
        id: 'example.registry',
        name: 'Example Registry Plugin',
        version: '1.0.0',
        description: 'Test plugin',
        entry: 'plugin.bin',
        repositoryUrl: 'https://github.com/rootbay/example',
        license: { spdxId: 'MIT' },
        requirements: {},
        distribution: {
                defaultMode: 'manual',
                autoUpdate: false,
                signature: 'sha256',
                signatureHash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
        },
        package: {
                artifact: 'plugin.bin',
                hash: 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
                sizeBytes: 128
	}
};

beforeEach(async () => {
	await db.delete(registryTable).run();
	await db.delete(pluginTable).run();
});

describe('Plugin registry store', () => {
	it('publishes, approves, and revokes entries', async () => {
                const store = createPluginRegistryStore();
                const published = await store.publish({ manifest: baseManifest, actorId: 'admin-1' });

		expect(published.pluginId).toBe(baseManifest.id);
		expect(published.approvalStatus).toBe('pending');

		const approved = await store.approve({ id: published.id, actorId: 'admin-1', note: 'ready' });
		expect(approved.approvalStatus).toBe('approved');
		expect(approved.approvedBy).toBe('admin-1');
		expect(approved.approvalNote).toBe('ready');

		const revoked = await store.revoke({
			id: published.id,
			actorId: 'admin-1',
			reason: 'deprecated'
		});
		expect(revoked.approvalStatus).toBe('rejected');
		expect(revoked.revocationReason).toBe('deprecated');

		const entries = await store.list();
		expect(entries).toHaveLength(1);
		expect(entries[0]?.approvalStatus).toBe('rejected');
	});
});
