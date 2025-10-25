import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest';
import { createHash } from 'node:crypto';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { db } from '$lib/server/db/index.js';
import { plugin as pluginTable } from '$lib/server/db/schema.js';
import { eq } from 'drizzle-orm';
import { refreshSignaturePolicy } from '$lib/server/plugins/signature-policy.js';
import { PluginTelemetryStore } from '$lib/server/plugins/telemetry-store.js';

const RELEASE_SIGNER = 'release';
const RELEASE_PUBLIC_KEY = 'ea9ceca1c7c7176859b235e095cbca9b5755746b741865cab5458d6f0e754cc2';
const RELEASE_SIGNATURE_TIMESTAMP = '2024-01-01T00:00:00Z';
const RELEASE_ARTIFACT_SIGNATURE =
        '2b4a75ee35bc4f9f9b15b84e8c993886f1ae98ce73e289df36c03835fc2920a71e9df3018650e99b0e48df6483d1adb112efec64edcd883725ef1e9dcc7c040b';

const mockEnv = vi.hoisted(() => {
        process.env.DATABASE_URL = ':memory:';
        return { env: { DATABASE_URL: ':memory:' } };
});

vi.mock('$env/dynamic/private', () => mockEnv, { virtual: true });

const authorizeAgent = vi.fn();
const getAgent = vi.fn();

vi.mock('$lib/server/rat/store.js', async () => {
        const mod = await vi.importActual<typeof import('$lib/server/rat/store.js')>(
                '$lib/server/rat/store.js'
        );
                return {
                        ...mod,
                        registry: {
                                ...mod.registry,
                                authorizeAgent,
                                getAgent
                        },
                        RegistryError: mod.RegistryError
                };
});

describe('agent plugin API', () => {
        let manifestDir: string;
        let trustDir: string;
        let trustPath: string;
        const manifestId = 'test-plugin';
        const artifactContent = 'artifact payload';

        beforeEach(async () => {
                manifestDir = mkdtempSync(join(tmpdir(), 'tenvy-agent-manifests-'));
                const manifestPath = join(manifestDir, `${manifestId}.json`);
        const artifactPath = join(manifestDir, 'pkg.zip');
        const artifactBuffer = Buffer.from(artifactContent, 'utf8');
        const artifactHash = createHash('sha256').update(artifactBuffer).digest('hex');
        writeFileSync(artifactPath, artifactBuffer);
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
                                signature: 'ed25519',
                                signatureHash: artifactHash,
                                signatureSigner: RELEASE_SIGNER,
                                signatureValue: RELEASE_ARTIFACT_SIGNATURE,
                                signatureTimestamp: RELEASE_SIGNATURE_TIMESTAMP
                        },
                        package: {
                                artifact: 'pkg.zip',
                                hash: artifactHash,
                                sizeBytes: artifactBuffer.byteLength
                        }
                        })
                );

                trustDir = mkdtempSync(join(tmpdir(), 'tenvy-plugin-trust-'));
                trustPath = join(trustDir, 'trust.json');
        writeFileSync(
                trustPath,
                JSON.stringify({
                        sha256AllowList: [artifactHash],
                        ed25519PublicKeys: { [RELEASE_SIGNER]: RELEASE_PUBLIC_KEY }
                })
        );

                process.env.TENVY_PLUGIN_MANIFEST_DIR = manifestDir;
                process.env.TENVY_PLUGIN_TRUST_CONFIG = trustPath;
                mockEnv.env = {
                        DATABASE_URL: ':memory:',
                        TENVY_PLUGIN_MANIFEST_DIR: manifestDir,
                        TENVY_PLUGIN_TRUST_CONFIG: trustPath
                };

                refreshSignaturePolicy();

                authorizeAgent.mockReset();
                getAgent.mockReset();
                getAgent.mockReturnValue({ id: 'agent-1' });
        });

        afterEach(async () => {
                await db.delete(pluginTable);
                rmSync(manifestDir, { recursive: true, force: true });
                rmSync(trustDir, { recursive: true, force: true });
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

        it('accepts plugin uploads and persists runtime metadata', async () => {
                const { POST } = await import('../src/routes/api/plugins/+server.ts');

                const artifactPayload = 'uploaded artifact payload';
                const artifactBuffer = Buffer.from(artifactPayload, 'utf8');
                const artifactHash = createHash('sha256').update(artifactBuffer).digest('hex');

                writeFileSync(
                        trustPath,
                        JSON.stringify({
                                sha256AllowList: [
                                        createHash('sha256').update(artifactContent, 'utf8').digest('hex'),
                                        artifactHash
                                ],
                                ed25519PublicKeys: { [RELEASE_SIGNER]: RELEASE_PUBLIC_KEY }
                        })
                );
                refreshSignaturePolicy();

                const manifest = {
                        id: 'uploaded-plugin',
                        name: 'Uploaded Plugin',
                        version: '1.0.0',
                        entry: 'uploaded.exe',
                        repositoryUrl: 'https://github.com/rootbay/uploaded-plugin',
                        license: { spdxId: 'MIT' },
                        requirements: { requiredModules: [] },
                        distribution: {
                                defaultMode: 'manual',
                                autoUpdate: false,
                                signature: 'sha256',
                                signatureHash: artifactHash
                        },
                        package: {
                                artifact: 'uploaded.zip',
                                hash: artifactHash,
                                sizeBytes: artifactBuffer.byteLength
                        }
                } satisfies Record<string, unknown>;

                const form = new FormData();
                form.set(
                        'manifest',
                        new File([JSON.stringify(manifest)], 'manifest.json', { type: 'application/json' })
                );
                form.set(
                        'artifact',
                        new File([artifactBuffer], 'uploaded.zip', { type: 'application/octet-stream' })
                );

                const response = await POST({
                        request: new Request('https://controller.test/api/plugins', {
                                method: 'POST',
                                body: form
                        })
                } as Parameters<typeof POST>[0]);

                expect(response.status).toBe(201);
                const body = (await response.json()) as {
                        plugin: { id: string; version: string; approvalStatus?: string };
                        approvalStatus: string;
                };
                expect(body.plugin.id).toBe('uploaded-plugin');
                expect(body.plugin.version).toBe('1.0.0');
                expect(body.approvalStatus).toBe('pending');

                const [row] = await db
                        .select()
                        .from(pluginTable)
                        .where(eq(pluginTable.id, 'uploaded-plugin'));
                expect(row?.approvalStatus).toBe('pending');
        });

        it('rejects uploads with invalid manifests', async () => {
                const { POST } = await import('../src/routes/api/plugins/+server.ts');

                const artifactBuffer = Buffer.from('broken', 'utf8');

                const manifest = {
                        id: '',
                        name: '',
                        version: 'not-a-version',
                        entry: '',
                        requirements: {},
                        distribution: { defaultMode: 'invalid', autoUpdate: false, signature: 'sha256' },
                        package: { artifact: '', hash: '' }
                } satisfies Record<string, unknown>;

                const form = new FormData();
                form.set(
                        'manifest',
                        new File([JSON.stringify(manifest)], 'manifest.json', { type: 'application/json' })
                );
                form.set('artifact', new File([artifactBuffer], 'broken.zip', { type: 'application/octet-stream' }));

                await expect(
                        POST({
                                request: new Request('https://controller.test/api/plugins', {
                                        method: 'POST',
                                        body: form
                                })
                        } as Parameters<typeof POST>[0])
                ).rejects.toMatchObject({ status: 400 });
        });

        it('rejects uploads with duplicate version numbers', async () => {
                const { POST } = await import('../src/routes/api/plugins/+server.ts');

                const artifactBuffer = Buffer.from(artifactContent, 'utf8');
                const manifestHash = createHash('sha256').update(artifactBuffer).digest('hex');

                const manifest = {
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
                                signature: 'sha256',
                                signatureHash: manifestHash
                        },
                        package: {
                                artifact: 'pkg.zip',
                                hash: manifestHash,
                                sizeBytes: artifactBuffer.byteLength
                        }
                } satisfies Record<string, unknown>;

                const form = new FormData();
                form.set(
                        'manifest',
                        new File([JSON.stringify(manifest)], 'manifest.json', { type: 'application/json' })
                );
                form.set(
                        'artifact',
                        new File([artifactBuffer], 'pkg.zip', { type: 'application/octet-stream' })
                );

                await expect(
                        POST({
                                request: new Request('https://controller.test/api/plugins', {
                                        method: 'POST',
                                        body: form
                                })
                        } as Parameters<typeof POST>[0])
                ).rejects.toMatchObject({ status: 409 });
        });

        it('records manual stage requests for clients', async () => {
                const bootstrapStore = new PluginTelemetryStore();
                await bootstrapStore.getManifestSnapshot();
                const approvedAt = new Date();
                await db
                        .update(pluginTable)
                        .set({ approvalStatus: 'approved', approvedAt })
                        .where(eq(pluginTable.id, manifestId));
                (bootstrapStore as { manifestSnapshot?: unknown }).manifestSnapshot = null;

                const stageModule = await import(
                        '../src/routes/api/clients/[id]/plugins/[pluginId]/stage/+server.ts'
                );

                const response = await stageModule.POST({
                        params: { id: 'agent-1', pluginId: manifestId },
                        request: new Request('https://controller.test/api/clients/agent-1/plugins/test-plugin/stage', {
                                method: 'POST'
                        })
                } as Parameters<typeof stageModule.POST>[0]);

                expect(getAgent).toHaveBeenCalledWith('agent-1');
                expect(response.status).toBe(200);

                const payload = (await response.json()) as { plugin: { id: string } };
                expect(payload.plugin.id).toBe(manifestId);

                const [pluginRow] = await db
                        .select({ lastManualPushAt: pluginTable.lastManualPushAt })
                        .from(pluginTable)
                        .where(eq(pluginTable.id, manifestId));
                expect(pluginRow?.lastManualPushAt).toBeInstanceOf(Date);

                const verificationStore = new PluginTelemetryStore();
                const snapshot = await verificationStore.getManifestSnapshot();
                const descriptor = snapshot.manifests.find((entry) => entry.pluginId === manifestId);
                expect(descriptor?.manualPushAt).not.toBeNull();

                const delta = await verificationStore.getManifestDelta({
                        digests: { [manifestId]: descriptor?.manifestDigest ?? '' }
                });
                expect(delta.updated.some((entry) => entry.pluginId === manifestId)).toBe(true);
        });
});
