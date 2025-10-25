import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Database from 'better-sqlite3';
import { drizzle } from 'drizzle-orm/better-sqlite3';
import { mkdtempSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { createPluginRuntimeStore } from './runtime-store.js';
import { plugin as pluginTable } from '$lib/server/db/schema.js';
import type {
        PluginManifest,
        PluginSignatureVerificationSummary
} from '../../../../../shared/types/plugin-manifest.js';
import type { LoadedPluginManifest } from '$lib/data/plugin-manifests.js';

const PLUGIN_TABLE_DDL = `
CREATE TABLE IF NOT EXISTS plugin (
        id TEXT PRIMARY KEY NOT NULL,
        status TEXT NOT NULL DEFAULT 'active',
        enabled INTEGER NOT NULL DEFAULT 1,
        auto_update INTEGER NOT NULL DEFAULT 0,
        runtime_type TEXT NOT NULL DEFAULT 'native',
        sandboxed INTEGER NOT NULL DEFAULT 0,
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

const baseManifest: PluginManifest = {
        id: 'runtime-test',
        name: 'Runtime Test',
        version: '1.0.0',
        description: 'Fixture plugin manifest used for runtime store tests.',
        entry: 'runtime-test.dll',
        author: 'Tenvy',
        repositoryUrl: 'https://github.com/rootbay/runtime-test',
        license: { spdxId: 'MIT', name: 'MIT License', url: 'https://opensource.org/license/mit' },
        capabilities: ['clipboard.capture'],
        requirements: {
                platforms: ['windows'],
                architectures: ['x86_64'],
                requiredModules: ['clipboard']
        },
        distribution: {
                defaultMode: 'automatic',
                autoUpdate: true,
                signature: 'sha256'
        },
        package: {
                artifact: 'runtime-test.dll',
                sizeBytes: 1024,
                hash: '00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff'
        }
};

const baseVerification: PluginSignatureVerificationSummary = {
        trusted: false,
        signatureType: 'sha256',
        status: 'untrusted',
        hash: baseManifest.package.hash,
        signer: null,
        publicKey: null,
        certificateChain: undefined,
        checkedAt: new Date(),
        signedAt: null,
        error: undefined,
        errorCode: undefined
};

const createLoadedManifest = (): LoadedPluginManifest => ({
        source: 'memory',
        raw: JSON.stringify(baseManifest),
        manifest: structuredClone(baseManifest),
        verification: { ...baseVerification, checkedAt: new Date() }
});

let tempDir: string;
let dbPath: string;

const openRuntimeStore = () => {
	const sqlite = new Database(dbPath);
	sqlite.exec(PLUGIN_TABLE_DDL);
	const drizzleDb = drizzle(sqlite, { schema: { plugin: pluginTable } });
	return { store: createPluginRuntimeStore(drizzleDb), sqlite };
};

beforeEach(() => {
	tempDir = mkdtempSync(join(tmpdir(), 'tenvy-runtime-store-'));
	dbPath = join(tempDir, 'runtime.sqlite');
});

afterEach(() => {
	rmSync(tempDir, { recursive: true, force: true });
});

describe('PluginRuntimeStore', () => {
	it('creates runtime rows with manifest defaults', async () => {
		const { store, sqlite } = openRuntimeStore();

                try {
                        const row = await store.ensure(createLoadedManifest());
                        expect(row.id).toBe(baseManifest.id);
                        expect(row.autoUpdate).toBe(true);
                        expect(row.defaultDeliveryMode).toBe('automatic');
                        expect(row.allowAutoSync).toBe(true);
                        expect(row.runtimeType).toBe('native');
                        expect(row.sandboxed).toBe(false);
                        expect(row.approvalStatus).toBe('pending');
                        expect(row.approvedAt).toBeNull();
                } finally {
                        sqlite.close();
                }
	});

	it('persists runtime updates across store instances', async () => {
		const first = openRuntimeStore();
		const timestamp = new Date('2024-05-19T15:45:00.000Z');

		try {
                        await first.store.ensure(createLoadedManifest());
                        await first.store.update(baseManifest.id, {
                                status: 'disabled',
                                enabled: false,
                                autoUpdate: false,
                                runtimeType: 'wasm',
                                sandboxed: true,
                                manualTargets: 5,
                                autoTargets: 2,
				defaultDeliveryMode: 'manual',
				allowManualPush: false,
				allowAutoSync: false,
				lastManualPushAt: timestamp,
				lastAutoSyncAt: timestamp,
				lastDeployedAt: timestamp,
				lastCheckedAt: timestamp,
				approvalStatus: 'approved',
				approvedAt: timestamp,
				approvalNote: 'ready for launch'
			});
		} finally {
			first.sqlite.close();
		}

		const second = openRuntimeStore();

		try {
                        const row = await second.store.ensure(createLoadedManifest());
			expect(row.status).toBe('disabled');
                        expect(row.enabled).toBe(false);
                        expect(row.autoUpdate).toBe(false);
                        expect(row.runtimeType).toBe('wasm');
                        expect(row.sandboxed).toBe(true);
			expect(row.manualTargets).toBe(5);
			expect(row.autoTargets).toBe(2);
			expect(row.defaultDeliveryMode).toBe('manual');
			expect(row.allowManualPush).toBe(false);
			expect(row.allowAutoSync).toBe(false);
			expect(row.lastManualPushAt?.toISOString()).toBe(timestamp.toISOString());
			expect(row.lastAutoSyncAt?.toISOString()).toBe(timestamp.toISOString());
			expect(row.lastDeployedAt?.toISOString()).toBe(timestamp.toISOString());
			expect(row.lastCheckedAt?.toISOString()).toBe(timestamp.toISOString());
			expect(row.approvalStatus).toBe('approved');
			expect(row.approvedAt?.toISOString()).toBe(timestamp.toISOString());
			expect(row.approvalNote).toBe('ready for launch');
		} finally {
			second.sqlite.close();
		}
	});
});
