import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { eq } from 'drizzle-orm';
import { AgentRegistry, MAX_PENDING_COMMANDS } from './store';
import { db } from '$lib/server/db';
import {
        agent as agentTable,
        agentCommand as agentCommandTable,
        agentNote as agentNoteTable,
        agentResult as agentResultTable
} from '$lib/server/db/schema';
import type { AgentRegistryEvent } from '../../../../../shared/types/registry-events';

vi.mock('$env/dynamic/private', () => import('../../../../tests/mocks/env-dynamic-private'));

const baseMetadata = {
        hostname: 'persisted-host',
        username: 'persisted-user',
        os: 'linux',
        architecture: 'x64'
};

async function clearRegistryTables() {
        await db.delete(agentNoteTable);
        await db.delete(agentCommandTable);
        await db.delete(agentResultTable);
        await db.delete(agentTable);
}

describe('AgentRegistry database integration', () => {
        beforeEach(async () => {
                await clearRegistryTables();
        });

        afterEach(async () => {
                await clearRegistryTables();
        });

        it('persists agent metadata, commands, notes, and results across instances', async () => {
                const registry = new AgentRegistry();
                const registration = registry.registerAgent({ metadata: baseMetadata });

                const queued = registry.queueCommand(registration.agentId, {
                        name: 'ping',
                        payload: { message: 'hello' }
                });

                const noteTimestamp = new Date().toISOString();
                registry.syncSharedNotes(registration.agentId, registration.agentKey, [
                        {
                                id: 'note-1',
                                visibility: 'shared',
                                ciphertext: 'ciphertext',
                                nonce: 'nonce',
                                digest: 'digest',
                                version: 1,
                                updatedAt: noteTimestamp
                        }
                ]);

                registry.syncAgent(registration.agentId, registration.agentKey, {
                        status: 'online',
                        timestamp: new Date().toISOString(),
                        results: [
                                {
                                        commandId: queued.command.id,
                                        success: true,
                                        completedAt: new Date().toISOString(),
                                        output: 'pong'
                                }
                        ]
                });

                await registry.flush();

                const restored = new AgentRegistry();
                const snapshot = restored.getAgent(registration.agentId);

                expect(snapshot.metadata.hostname).toBe(baseMetadata.hostname);
                expect(snapshot.pendingCommands).toBe(0);
                expect(snapshot.recentResults).toHaveLength(1);

                const restoredNotes = restored.syncSharedNotes(registration.agentId, registration.agentKey, []);
                expect(restoredNotes).toHaveLength(1);
                expect(restoredNotes[0]?.id).toBe('note-1');
        });

        it('clamps pending commands under concurrent queueing and persists trimmed snapshot', async () => {
                const registry = new AgentRegistry();
                const { agentId } = registry.registerAgent({ metadata: baseMetadata });

                const queueCount = MAX_PENDING_COMMANDS + 32;
                await Promise.all(
                        Array.from({ length: queueCount }, (_, index) =>
                                Promise.resolve(
                                        registry.queueCommand(agentId, {
                                                name: 'ping',
                                                payload: { index }
                                        })
                                )
                        )
                );

                const snapshot = registry.getAgent(agentId);
                expect(snapshot.pendingCommands).toBe(MAX_PENDING_COMMANDS);

                await registry.flush();
                const persistedCommands = await db
                        .select({ id: agentCommandTable.id })
                        .from(agentCommandTable)
                        .where(eq(agentCommandTable.agentId, agentId));
                expect(persistedCommands).toHaveLength(MAX_PENDING_COMMANDS);
        });

        it('fans out subscription events to multiple listeners', () => {
                const registry = new AgentRegistry();
                const registration = registry.registerAgent({ metadata: baseMetadata });

                const eventsA: AgentRegistryEvent[] = [];
                const eventsB: AgentRegistryEvent[] = [];

                const unsubscribeA = registry.subscribe((event) => {
                        eventsA.push(event);
                });
                const unsubscribeB = registry.subscribe((event) => {
                        eventsB.push(event);
                });

                const queued = registry.queueCommand(registration.agentId, {
                        name: 'ping',
                        payload: {}
                });

                registry.updateAgentTags(registration.agentId, ['primary']);
                registry.syncSharedNotes(registration.agentId, registration.agentKey, [
                        {
                                id: 'broadcast-note',
                                visibility: 'shared',
                                ciphertext: 'cipher',
                                nonce: 'nonce',
                                digest: 'digest',
                                version: 1,
                                updatedAt: new Date().toISOString()
                        }
                ]);

                expect(eventsA.some((event) => event.type === 'command' && event.command.id === queued.command.id)).toBe(true);
                expect(eventsB.some((event) => event.type === 'command' && event.command.id === queued.command.id)).toBe(true);
                expect(eventsA.some((event) => event.type === 'agent')).toBe(true);
                expect(eventsB.some((event) => event.type === 'notes')).toBe(true);

                unsubscribeA();
                unsubscribeB();

                const before = eventsA.length;
                registry.queueCommand(registration.agentId, { name: 'ping', payload: {} });
                expect(eventsA.length).toBe(before);
        });
});
