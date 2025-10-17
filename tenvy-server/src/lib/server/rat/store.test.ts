import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import path from 'path';
import { createHash } from 'crypto';
import { AgentRegistry } from './store';
import { defaultAgentConfig } from '../../../../../shared/types/config';

const baseMetadata = {
        hostname: 'persisted-host',
        username: 'persisted-user',
        os: 'linux',
        architecture: 'x64'
};

function fingerprint(metadata: typeof baseMetadata & { group?: string }) {
        const hash = createHash('sha256');
        hash.update(metadata.hostname.trim().toLowerCase());
        hash.update('|');
        hash.update(metadata.username.trim().toLowerCase());
        hash.update('|');
        hash.update(metadata.os.trim().toLowerCase());
        hash.update('|');
        hash.update(metadata.architecture.trim().toLowerCase());
        hash.update('|');
        hash.update(metadata.group?.trim().toLowerCase() ?? '');
        return hash.digest('hex');
}

const MAX_RECENT_RESULTS = 25;

describe('AgentRegistry persistence hygiene', () => {
        let tempDir: string;

        beforeEach(() => {
                tempDir = mkdtempSync(path.join(tmpdir(), 'agent-registry-audit-'));
        });

        afterEach(() => {
                rmSync(tempDir, { recursive: true, force: true });
        });

        it('restores persisted agents without clobbering timestamps or configs', async () => {
                const storagePath = path.join(tempDir, 'registry.json');
                const connectedAt = new Date('2024-01-01T00:00:00.000Z');
                const lastSeen = new Date('2024-01-02T12:34:56.000Z');

                const persisted = {
                        version: 1,
                        agents: [
                                {
                                        id: 'agent-1',
                                        key: 'secret-key',
                                        metadata: baseMetadata,
                                        status: 'online',
                                        connectedAt: connectedAt.toISOString(),
                                        lastSeen: lastSeen.toISOString(),
                                        metrics: { memoryBytes: 42 },
                                        config: {
                                                pollIntervalMs: -10,
                                                maxBackoffMs: 1000,
                                                jitterRatio: 5
                                        },
                                        pendingCommands: [
                                                {
                                                        id: 'cmd-1',
                                                        name: 'ping',
                                                        payload: {},
                                                        createdAt: connectedAt.toISOString()
                                                }
                                        ],
                                        recentResults: [
                                                {
                                                        commandId: 'cmd-1',
                                                        success: true,
                                                        completedAt: lastSeen.toISOString(),
                                                        output: 'ok'
                                                }
                                        ],
                                        sharedNotes: [],
                                        fingerprint: fingerprint(baseMetadata)
                                }
                        ]
                } satisfies Record<string, unknown>;

                writeFileSync(storagePath, JSON.stringify(persisted, null, 2), 'utf-8');

                const registry = new AgentRegistry({ storagePath });

                const snapshot = registry.getAgent('agent-1');
                expect(snapshot.status).toBe('offline');
                expect(snapshot.lastSeen).toBe(lastSeen.toISOString());
                expect(snapshot.pendingCommands).toBe(1);
                expect(snapshot.recentResults).toHaveLength(1);

                const sync = registry.syncAgent('agent-1', 'secret-key', {
                        status: 'online',
                        timestamp: new Date().toISOString(),
                        results: []
                });

                expect(sync.commands).toHaveLength(1);
                expect(sync.config.pollIntervalMs).toBe(defaultAgentConfig.pollIntervalMs);
                expect(sync.config.maxBackoffMs).toBeGreaterThanOrEqual(sync.config.pollIntervalMs);
                expect(sync.config.jitterRatio).toBe(defaultAgentConfig.jitterRatio);

                await registry.flush();
        });

        it('deduplicates recent results and protects registry snapshots from mutation', async () => {
                const storagePath = path.join(tempDir, 'registry.json');
                const registry = new AgentRegistry({ storagePath });
                const registration = registry.registerAgent({ metadata: baseMetadata });

                const start = Date.now();
                const duplicateResults = [
                        {
                                commandId: 'cmd-duplicate',
                                success: true,
                                completedAt: new Date(start).toISOString()
                        },
                        {
                                commandId: 'cmd-duplicate',
                                success: true,
                                completedAt: new Date(start + 10).toISOString(),
                                output: 'later'
                        }
                ];

                const extraResults = Array.from({ length: MAX_RECENT_RESULTS + 5 }, (_, idx) => ({
                        commandId: `cmd-${idx}`,
                        success: idx % 2 === 0,
                        completedAt: new Date(start + 100 + idx).toISOString(),
                        error: idx % 2 === 0 ? undefined : 'boom'
                }));

                registry.syncAgent(registration.agentId, registration.agentKey, {
                        status: 'online',
                        timestamp: new Date().toISOString(),
                        results: [...duplicateResults, ...extraResults]
                });

                registry.updateAgentTags(registration.agentId, ['primary']);

                const snapshot = registry.getAgent(registration.agentId);
                expect(snapshot.recentResults).toHaveLength(MAX_RECENT_RESULTS);
                const uniqueIds = new Set(snapshot.recentResults.map((result) => result.commandId));
                expect(uniqueIds.size).toBe(snapshot.recentResults.length);

                snapshot.metadata.hostname = 'mutated-host';
                snapshot.metadata.tags?.push('mutated');
                if (snapshot.recentResults.length > 0) {
                        snapshot.recentResults[0]!.commandId = 'mutated';
                }

                const nextSnapshot = registry.getAgent(registration.agentId);
                expect(nextSnapshot.metadata.hostname).toBe(baseMetadata.hostname);
                expect(nextSnapshot.metadata.tags).toEqual(['primary']);
                expect(nextSnapshot.recentResults[0]?.commandId).not.toBe('mutated');

                const syncResponse = registry.syncAgent(registration.agentId, registration.agentKey, {
                        status: 'online',
                        timestamp: new Date().toISOString(),
                        results: []
                });
                syncResponse.config.pollIntervalMs = 1;

                const followUp = registry.syncAgent(registration.agentId, registration.agentKey, {
                        status: 'online',
                        timestamp: new Date().toISOString(),
                        results: []
                });
                expect(followUp.config.pollIntervalMs).toBe(defaultAgentConfig.pollIntervalMs);

                await registry.flush();
        });
});
