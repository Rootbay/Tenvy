import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentRegistry } from '../src/lib/server/rat/store';
import { db } from '../src/lib/server/db';
import {
        agent as agentTable,
        agentCommand as agentCommandTable,
        agentNote as agentNoteTable,
        agentResult as agentResultTable,
        auditEvent as auditEventTable,
        registrySubscription as registrySubscriptionTable,
        pluginInstallation as pluginInstallationTable,
        plugin as pluginTable,
        user as userTable,
        voucher as voucherTable
} from '../src/lib/server/db/schema';
import type { AgentRegistryEvent } from '../../shared/types/registry-events';

vi.mock('$env/dynamic/private', () => import('./mocks/env-dynamic-private'));

const baseMetadata = {
        hostname: 'broadcast-host',
        username: 'broadcast-user',
        os: 'linux',
        architecture: 'x64'
};

function clearTables() {
        db.delete(agentNoteTable).run();
        db.delete(agentCommandTable).run();
        db.delete(agentResultTable).run();
        db.delete(auditEventTable).run();
        db.delete(pluginInstallationTable).run();
        db.delete(pluginTable).run();
        db.delete(agentTable).run();
        db.delete(userTable).run();
        db.delete(voucherTable).run();
        db.delete(registrySubscriptionTable).run();
}

describe('AgentRegistry shared broadcast', () => {
        beforeEach(() => {
                clearTables();
        });

        afterEach(() => {
                clearTables();
        });

        it('fans out command updates and hydrates snapshots for new viewers', async () => {
                const registry = new AgentRegistry();
                const registration = registry.registerAgent({ metadata: baseMetadata });

                const eventsA: AgentRegistryEvent[] = [];
                const eventsB: AgentRegistryEvent[] = [];

                const subscriptionA = registry.subscribeForAdmin('viewer-a', (event) => {
                        eventsA.push(event);
                });
                const subscriptionB = registry.subscribeForAdmin('viewer-b', (event) => {
                        eventsB.push(event);
                });

                expect(subscriptionA.snapshot).toHaveLength(1);
                expect(subscriptionB.snapshot).toHaveLength(1);

                const queued = registry.queueCommand(registration.agentId, {
                        name: 'ping',
                        payload: { message: 'hello' }
                });

                expect(eventsA.some((event) => event.type === 'command')).toBe(true);
                expect(eventsB.some((event) => event.type === 'command')).toBe(true);

                await registry.syncAgent(registration.agentId, registration.agentKey, {
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

                expect(eventsA.some((event) => event.type === 'agent')).toBe(true);
                expect(eventsB.some((event) => event.type === 'agent')).toBe(true);

                subscriptionA.unsubscribe();
                subscriptionB.unsubscribe();

                const rehydrated = registry.subscribeForAdmin('viewer-a', () => {});
                expect(rehydrated.snapshot).toHaveLength(1);
                const [agentSnapshot] = rehydrated.snapshot;
                expect(agentSnapshot.recentResults).toHaveLength(1);
                expect(agentSnapshot.recentResults[0]?.commandId).toBe(queued.command.id);
        });
});
