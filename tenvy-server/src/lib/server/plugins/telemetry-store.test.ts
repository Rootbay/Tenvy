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
import type { PluginManifest } from '../../../../../shared/types/plugin-manifest.js';

vi.mock('$env/dynamic/private', () => import('../../../../tests/mocks/env-dynamic-private'));

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
                distribution: {
                        defaultMode: 'automatic',
                        autoUpdate: true,
                        signature: { type: 'sha256', hash, signature: 'signed' }
                },
                requirements: {
                        platforms: ['windows'],
                        architectures: ['x86_64'],
			requiredModules: []
		},
                package: {
                        artifact: 'plugin.dll',
                        sizeBytes: 1024,
                        hash
                }
        };
}

beforeEach(async () => {
        process.env.DATABASE_URL = ':memory:';
        manifestDir = mkdtempSync(join(tmpdir(), 'tenvy-plugin-manifests-'));
        writeFileSync(join(manifestDir, 'test-plugin.json'), JSON.stringify(createManifest('abc123')));

        policyPath = join(manifestDir, 'trust.json');
        writeFileSync(
                policyPath,
                JSON.stringify({
                        sha256AllowList: ['abc123']
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

		const now = new Date().toISOString();
                await store.syncAgent('agent-1', baseMetadata, [
                        {
                                pluginId: 'test-plugin',
                                version: '1.0.0',
                                status: 'installed',
                                hash: 'abc123',
				lastDeployedAt: now,
				lastCheckedAt: now,
				error: null
			}
		]);

		const installations = await store.listAgentPlugins('agent-1');
		expect(installations).toHaveLength(1);
		expect(installations[0]?.status).toBe('installed');

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
		const now = new Date().toISOString();

		await store.syncAgent('agent-2', baseMetadata, [
			{
				pluginId: 'test-plugin',
				version: '1.0.0',
				status: 'installed',
				hash: 'deadbeef',
				lastDeployedAt: now,
				lastCheckedAt: now,
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
});
