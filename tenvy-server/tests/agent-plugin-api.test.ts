import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { db } from '$lib/server/db/index.js';
import { plugin as pluginTable } from '$lib/server/db/schema.js';
import { eq } from 'drizzle-orm';

const mockEnv = vi.hoisted(() => {
        process.env.DATABASE_URL = ':memory:';
        return { env: { DATABASE_URL: ':memory:' } };
});

vi.mock('$env/dynamic/private', () => mockEnv, { virtual: true });

const authorizeAgent = vi.fn();

vi.mock('$lib/server/rat/store.js', async () => {
        const mod = await vi.importActual<typeof import('$lib/server/rat/store.js')>(
                '$lib/server/rat/store.js'
        );
        return {
                ...mod,
                registry: {
                        ...mod.registry,
                        authorizeAgent
                },
                RegistryError: mod.RegistryError
        };
});

describe('agent plugin API', () => {
        let manifestDir: string;
        let trustPath: string;
        const manifestId = 'test-plugin';
        const artifactContent = 'artifact payload';

        beforeEach(async () => {
                manifestDir = mkdtempSync(join(tmpdir(), 'tenvy-agent-manifests-'));
                const manifestPath = join(manifestDir, `${manifestId}.json`);
                const artifactPath = join(manifestDir, 'pkg.zip');
                writeFileSync(
                        manifestPath,
                        JSON.stringify({
                                id: manifestId,
                                name: 'Test Plugin',
                                version: '1.0.0',
                                entry: 'plugin.exe',
                                repositoryUrl: 'https://github.com/rootbay/test-plugin',
                                license: { spdxId: 'MIT' },
                                requirements: {},
                                distribution: {
                                        defaultMode: 'automatic',
                                        autoUpdate: true,
                                        signature: 'sha256'
                                },
                                package: { artifact: 'pkg.zip', hash: 'abc123' }
                        })
                );
                writeFileSync(artifactPath, artifactContent);

                trustPath = join(manifestDir, 'trust.json');
                writeFileSync(trustPath, JSON.stringify({ sha256AllowList: ['abc123'] }));

                process.env.TENVY_PLUGIN_MANIFEST_DIR = manifestDir;
                process.env.TENVY_PLUGIN_TRUST_CONFIG = trustPath;
                mockEnv.env = {
                        DATABASE_URL: ':memory:',
                        TENVY_PLUGIN_MANIFEST_DIR: manifestDir,
                        TENVY_PLUGIN_TRUST_CONFIG: trustPath
                };

                authorizeAgent.mockReset();
        });

        afterEach(async () => {
                await db.delete(pluginTable);
                rmSync(manifestDir, { recursive: true, force: true });
                delete process.env.TENVY_PLUGIN_MANIFEST_DIR;
                delete process.env.TENVY_PLUGIN_TRUST_CONFIG;
                mockEnv.env = { DATABASE_URL: ':memory:' };
        });

        it('returns manifest snapshots and artifacts for authorized agents', async () => {
                const sharedModule = await import('../src/routes/api/agents/[id]/plugins/_shared.js');
                const { telemetryStore } = sharedModule;
                await telemetryStore.getManifestSnapshot();
                await db
                        .update(pluginTable)
                        .set({ approvalStatus: 'approved', approvedAt: new Date() })
                        .where(eq(pluginTable.id, manifestId));
                (telemetryStore as { manifestSnapshot?: unknown }).manifestSnapshot = null;

                const listModule = await import('../src/routes/api/agents/[id]/plugins/+server.js');
                const manifestModule = await import(
                        '../src/routes/api/agents/[id]/plugins/[pluginId]/+server.js'
                );
                const artifactModule = await import(
                        '../src/routes/api/agents/[id]/plugins/[pluginId]/artifact/+server.js'
                );

                const requestHeaders = { Authorization: 'Bearer agent-key' };

                const listResponse = await listModule.GET({
                        params: { id: 'agent-1' },
                        request: new Request('https://controller.test', { headers: requestHeaders })
                } as Parameters<typeof listModule.GET>[0]);

                expect(authorizeAgent).toHaveBeenCalledWith('agent-1', 'agent-key');

                const snapshot = (await listResponse.json()) as {
                        version: string;
                        manifests: Array<{ pluginId: string; manifestDigest: string }>;
                };
                expect(snapshot.manifests[0]?.pluginId).toBe(manifestId);

                const manifestResponse = await manifestModule.GET({
                        params: { id: 'agent-1', pluginId: manifestId },
                        request: new Request('https://controller.test', { headers: requestHeaders })
                } as Parameters<typeof manifestModule.GET>[0]);

                const manifestText = await manifestResponse.text();
                expect(manifestText).toContain(`"id":"${manifestId}"`);

                const artifactResponse = await artifactModule.GET({
                        params: { id: 'agent-1', pluginId: manifestId },
                        request: new Request('https://controller.test', { headers: requestHeaders })
                } as Parameters<typeof artifactModule.GET>[0]);

                const artifactBuffer = await artifactResponse.arrayBuffer();
                expect(Buffer.from(artifactBuffer).toString()).toBe(artifactContent);
        });
});
