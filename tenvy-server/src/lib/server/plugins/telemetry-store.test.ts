import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { and, eq } from 'drizzle-orm';
import { PluginTelemetryStore } from './telemetry-store.js';
import { refreshSignaturePolicy } from '$lib/server/plugins/signature-policy.js';
import { createPluginRuntimeStore } from './runtime-store.js';
import { loadPluginManifests } from '$lib/data/plugin-manifests.js';
import { db } from '$lib/server/db/index.js';
import {
	agent as agentTable,
	plugin as pluginTable,
	pluginInstallation as pluginInstallationTable,
	auditEvent as auditEventTable
} from '$lib/server/db/schema.js';
import type { AgentMetadata } from '../../../../../shared/types/agent.js';
import type {
	PluginInstallationTelemetry,
	PluginManifest
} from '../../../../../shared/types/plugin-manifest.js';

vi.mock('$env/dynamic/private', () => import('../../../../tests/mocks/env-dynamic-private'));

const manifestHash = '00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff';
const mismatchedHash = 'ffeeddccbbaa99887766554433221100ffeeddccbbaa99887766554433221100';

const baseMetadata: AgentMetadata = {
	hostname: 'agent.local',
	username: 'operator',
	os: 'Windows 11',
	architecture: 'amd64',
	ipAddress: '10.0.0.5',
	publicIpAddress: '10.0.0.5',
	tags: [],
	version: '1.2.3',
	group: undefined,
	location: undefined
};

let manifestDir: string;
let policyPath: string;

function createManifest(hash: string): PluginManifest {
	return {
		id: 'test-plugin',
		name: 'Test Plugin',
		version: '1.0.0',
		entry: 'plugin.dll',
		repositoryUrl: 'https://github.com/rootbay/test-plugin',
		license: {
			spdxId: 'MIT',
			name: 'MIT License'
		},
		capabilities: ['clipboard.capture'],
		distribution: {
			defaultMode: 'automatic',
			autoUpdate: true,
			signature: 'sha256',
			signatureHash: hash
		},
		requirements: {
			platforms: ['windows'],
			architectures: ['x86_64'],
			requiredModules: ['clipboard']
		},
		package: {
			artifact: 'plugin.dll',
			sizeBytes: 1024,
			hash
		}
	};
}

const expectDateCloseTo = (value: Date | null | undefined, expectedMs: number) => {
	expect(value).not.toBeNull();
	const actual = value?.getTime();
	expect(actual).toBeDefined();
	if (actual !== undefined) {
		expect(Math.abs(actual - expectedMs)).toBeLessThanOrEqual(1000);
	}
};

const descriptorFingerprint = (descriptor: {
	manifestDigest: string;
	manualPushAt: string | null;
}) =>
	descriptor.manualPushAt && descriptor.manualPushAt.trim().length > 0
		? `${descriptor.manifestDigest}:${descriptor.manualPushAt}`
		: descriptor.manifestDigest;

beforeEach(async () => {
	process.env.DATABASE_URL = ':memory:';
	manifestDir = mkdtempSync(join(tmpdir(), 'tenvy-plugin-manifests-'));
	writeFileSync(
		join(manifestDir, 'test-plugin.json'),
		JSON.stringify(createManifest(manifestHash))
	);

	policyPath = join(manifestDir, 'trust.json');
	writeFileSync(
		policyPath,
		JSON.stringify({
			sha256AllowList: [manifestHash]
		})
	);
	process.env.TENVY_PLUGIN_TRUST_CONFIG = policyPath;
	refreshSignaturePolicy();

	const now = new Date();
	await db.insert(agentTable).values([
		{
			id: 'agent-1',
			keyHash: 'hash-agent-1',
			metadata: JSON.stringify(baseMetadata),
			status: 'online',
			connectedAt: now,
			lastSeen: now,
			metrics: JSON.stringify({}),
			config: JSON.stringify({}),
			fingerprint: 'fingerprint-agent-1',
			createdAt: now,
			updatedAt: now
		},
		{
			id: 'agent-2',
			keyHash: 'hash-agent-2',
			metadata: JSON.stringify(baseMetadata),
			status: 'online',
			connectedAt: now,
			lastSeen: now,
			metrics: JSON.stringify({}),
			config: JSON.stringify({}),
			fingerprint: 'fingerprint-agent-2',
			createdAt: now,
			updatedAt: now
		}
	]);
});

afterEach(async () => {
	await db.delete(pluginInstallationTable);
	await db.delete(pluginTable);
	await db.delete(auditEventTable);
	await db.delete(agentTable);
	rmSync(manifestDir, { recursive: true, force: true });
	delete process.env.TENVY_PLUGIN_TRUST_CONFIG;
	refreshSignaturePolicy();
});

describe('PluginTelemetryStore', () => {
	it('records successful installation telemetry', async () => {
		const runtimeStore = createPluginRuntimeStore();
		const [record] = await loadPluginManifests({ directory: manifestDir });
		expect(record).toBeDefined();
		await runtimeStore.ensure(record!);
		await runtimeStore.update(record!.manifest.id, {
			approvalStatus: 'approved',
			approvedAt: new Date()
		});

		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});

		const now = Date.now();
		await store.syncAgent('agent-1', baseMetadata, [
			{
				pluginId: 'test-plugin',
				version: '1.0.0',
				status: 'installed',
				hash: manifestHash,
				timestamp: now,
				error: null
			}
		]);

		const installations = await store.listAgentPlugins('agent-1');
		expect(installations).toHaveLength(1);
		const installation = installations[0];
		expect(installation?.status).toBe('installed');
		expectDateCloseTo(installation?.lastCheckedAt ?? null, now);
		expectDateCloseTo(installation?.lastDeployedAt ?? null, now);

		const [runtime] = await db.select().from(pluginTable).where(eq(pluginTable.id, 'test-plugin'));
		expect(runtime.installations).toBe(1);
	});

	it('blocks mismatched hashes and records audit events', async () => {
		const runtimeStore = createPluginRuntimeStore();
		const [record] = await loadPluginManifests({ directory: manifestDir });
		expect(record).toBeDefined();
		await runtimeStore.ensure(record!);
		await runtimeStore.update(record!.manifest.id, {
			approvalStatus: 'approved',
			approvedAt: new Date()
		});

		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});
		const now = Date.now();

		await store.syncAgent('agent-2', baseMetadata, [
			{
				pluginId: 'test-plugin',
				version: '1.0.0',
				status: 'installed',
				hash: mismatchedHash,
				timestamp: now,
				error: null
			}
		]);

		const installations = await store.listAgentPlugins('agent-2');
		expect(installations[0]?.status).toBe('blocked');
		expect(installations[0]?.error).toContain('hash mismatch');

		const audits = await db
			.select()
			.from(auditEventTable)
			.where(
				and(eq(auditEventTable.agentId, 'agent-2'), eq(auditEventTable.commandName, 'plugin-sync'))
			);
		expect(audits.length).toBeGreaterThan(0);
	});

	it('retrieves individual plugin telemetry records', async () => {
		const runtimeStore = createPluginRuntimeStore();
		const [record] = await loadPluginManifests({ directory: manifestDir });
		expect(record).toBeDefined();
		await runtimeStore.ensure(record!);
		await runtimeStore.update(record!.manifest.id, {
			approvalStatus: 'approved',
			approvedAt: new Date()
		});

		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});

		const now = Date.now();
		await store.syncAgent('agent-1', baseMetadata, [
			{
				pluginId: 'test-plugin',
				version: '1.0.0',
				status: 'installed',
				hash: manifestHash,
				timestamp: now,
				error: null
			}
		]);

		const telemetry = await store.getAgentPlugin('agent-1', 'test-plugin');
		expect(telemetry).not.toBeNull();
		expect(telemetry?.status).toBe('installed');
		expect(telemetry?.version).toBe('1.0.0');
		expectDateCloseTo(telemetry?.lastCheckedAt ?? null, now);

		const missing = await store.getAgentPlugin('agent-1', 'missing-plugin');
		expect(missing).toBeNull();
	});

	it('accepts legacy ISO timestamp strings', async () => {
		const runtimeStore = createPluginRuntimeStore();
		const [record] = await loadPluginManifests({ directory: manifestDir });
		expect(record).toBeDefined();
		await runtimeStore.ensure(record!);
		await runtimeStore.update(record!.manifest.id, {
			approvalStatus: 'approved',
			approvedAt: new Date()
		});

		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});

		const iso = new Date().toISOString();
		const legacyPayload = {
			pluginId: 'test-plugin',
			version: '1.0.0',
			status: 'installed',
			hash: manifestHash,
			timestamp: iso,
			error: null
		} as unknown as PluginInstallationTelemetry;

		await store.syncAgent('agent-1', baseMetadata, [legacyPayload]);

		const telemetry = await store.getAgentPlugin('agent-1', 'test-plugin');
		expectDateCloseTo(telemetry?.lastCheckedAt ?? null, new Date(iso).getTime());
	});

	it('exposes approved manifest snapshots and deltas', async () => {
		const runtimeStore = createPluginRuntimeStore();
		const [record] = await loadPluginManifests({ directory: manifestDir });
		expect(record).toBeDefined();
		await runtimeStore.ensure(record!);
		const approvedAt = new Date();
		approvedAt.setMilliseconds(0);
		await runtimeStore.update(record!.manifest.id, {
			approvalStatus: 'approved',
			approvedAt
		});

		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});

		const snapshot = await store.getManifestSnapshot();
		expect(snapshot.manifests).toHaveLength(1);
		const descriptor = snapshot.manifests[0];
		expect(descriptor.pluginId).toBe('test-plugin');
		expect(descriptor.manifestDigest).toMatch(/^[0-9a-f]{64}$/);
		expect(descriptor.approvedAt).toBe(approvedAt.toISOString());
		expect(descriptor.manualPushAt).toBeNull();

		const fullDelta = await store.getManifestDelta({ digests: {} });
		expect(fullDelta.updated).toHaveLength(1);
		expect(fullDelta.updated[0]?.pluginId).toBe('test-plugin');

		const noDelta = await store.getManifestDelta({
			version: snapshot.version,
			digests: { 'test-plugin': descriptor.manifestDigest }
		});
		expect(noDelta.updated).toHaveLength(0);
		expect(noDelta.removed).toHaveLength(0);

		const approvedManifest = await store.getApprovedManifest('test-plugin');
		expect(approvedManifest?.descriptor.manifestDigest).toBe(descriptor.manifestDigest);
	});

	it('excludes disabled plugins from agent manifest deltas', async () => {
		const runtimeStore = createPluginRuntimeStore();
		const [record] = await loadPluginManifests({ directory: manifestDir });
		expect(record).toBeDefined();
		await runtimeStore.ensure(record!);
		await runtimeStore.update(record!.manifest.id, {
			approvalStatus: 'approved',
			approvedAt: new Date()
		});

		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});

		const initialDelta = await store.getAgentManifestDelta('agent-1', { digests: {} });
		expect(initialDelta.updated).toHaveLength(1);
		const descriptor = initialDelta.updated[0]!;
		const fingerprint = descriptorFingerprint({
			manifestDigest: descriptor.manifestDigest,
			manualPushAt: descriptor.manualPushAt ?? null
		});

		const knownState = {
			version: initialDelta.version,
			digests: { [descriptor.pluginId]: fingerprint }
		};

		await store.syncAgent('agent-1', baseMetadata, [
			{
				pluginId: descriptor.pluginId,
				version: descriptor.version ?? '1.0.0',
				status: 'installed',
				hash: manifestHash,
				timestamp: Date.now(),
				error: null
			}
		]);

		await store.updateAgentPlugin('agent-1', descriptor.pluginId, { enabled: false });

		const afterDisable = await store.listAgentPlugins('agent-1');
		const disabledRecord = afterDisable.find((entry) => entry.pluginId === descriptor.pluginId);
		expect(disabledRecord?.enabled).toBe(false);

		const removalDelta = await store.getAgentManifestDelta('agent-1', knownState);
		expect(removalDelta.removed).toEqual([descriptor.pluginId]);
		expect(removalDelta.updated).toHaveLength(0);
		expect(removalDelta.version).not.toBe(initialDelta.version);

		const removedState = {
			version: removalDelta.version,
			digests: {}
		};

		await store.updateAgentPlugin('agent-1', descriptor.pluginId, { enabled: true });

		const restorationDelta = await store.getAgentManifestDelta('agent-1', removedState);
		expect(restorationDelta.removed).toHaveLength(0);
		expect(restorationDelta.updated.some((entry) => entry.pluginId === descriptor.pluginId)).toBe(
			true
		);
	});

	it('records manual push timestamps and surfaces them in manifest deltas', async () => {
		const runtimeStore = createPluginRuntimeStore();
		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});

		await store.getManifestSnapshot();
		const approvedAt = new Date();
		await db
			.update(pluginTable)
			.set({ approvalStatus: 'approved', approvedAt })
			.where(eq(pluginTable.id, 'test-plugin'));

		(store as { manifestSnapshot?: unknown }).manifestSnapshot = null;

		const baseline = await store.getManifestSnapshot();
		const descriptor = baseline.manifests[0];
		expect(descriptor.manualPushAt).toBeNull();

		await store.recordManualPush('agent-1', 'test-plugin');

		const refreshed = await store.getManifestSnapshot();
		const updated = refreshed.manifests[0];
		expect(updated.manualPushAt).not.toBeNull();

		const delta = await store.getManifestDelta({
			digests: { 'test-plugin': descriptor.manifestDigest }
		});
		expect(delta.updated).toHaveLength(1);
		expect(delta.updated[0]?.manualPushAt).toBe(updated.manualPushAt);
	});

	it('omits conflicting manifests from snapshots and deltas', async () => {
		const conflictManifest = createManifest(manifestHash);
		conflictManifest.version = '2.0.0';
		writeFileSync(join(manifestDir, 'test-plugin-alt.json'), JSON.stringify(conflictManifest));

		const runtimeStore = createPluginRuntimeStore();
		const records = await loadPluginManifests({ directory: manifestDir });
		for (const record of records) {
			await runtimeStore.ensure(record);
		}
		const approvedAt = new Date();
		await runtimeStore.update('test-plugin', { approvalStatus: 'approved', approvedAt });

		const store = new PluginTelemetryStore({
			runtimeStore,
			manifestDirectory: manifestDir
		});

		const snapshot = await store.getManifestSnapshot();
		expect(snapshot.manifests).toHaveLength(0);

		const delta = await store.getManifestDelta({ digests: {} });
		expect(delta.updated).toHaveLength(0);
		expect(delta.removed).toHaveLength(0);

		const approved = await store.getApprovedManifest('test-plugin');
		expect(approved).toBeNull();
	});
});
