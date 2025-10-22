import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { and, eq } from 'drizzle-orm';
import { AgentRegistry, MAX_PENDING_COMMANDS } from './store';
import { db } from '$lib/server/db';
import {
        agent as agentTable,
        agentCommand as agentCommandTable,
        agentNote as agentNoteTable,
        agentResult as agentResultTable,
        auditEvent as auditEventTable,
        user as userTable,
        voucher as voucherTable,
        plugin as pluginTable,
        pluginInstallation as pluginInstallationTable
} from '$lib/server/db/schema';
import type { AgentRegistryEvent } from '../../../../../shared/types/registry-events';
import remoteDesktopEngineManifestJson from '../../../../../shared/pluginmanifest/remote-desktop-engine.json';
import type { PluginManifest } from '../../../../../shared/types/plugin-manifest';
import {
        remoteDesktopEnginePluginId,
        requiredRemoteDesktopPluginVersion
} from './remote-desktop';

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
        await db.delete(auditEventTable);
        await db.delete(pluginInstallationTable);
        await db.delete(pluginTable);
        await db.delete(agentTable);
        await db.delete(userTable);
        await db.delete(voucherTable);
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

        it('records audit events for queued and executed commands', async () => {
                const registry = new AgentRegistry();
                const registration = registry.registerAgent({ metadata: baseMetadata });

                db.insert(voucherTable)
			.values({ id: 'voucher-audit', codeHash: 'hash', createdAt: new Date() })
			.run();
		db.insert(userTable)
			.values({
				id: 'operator-123',
				voucherId: 'voucher-audit',
				role: 'operator',
				createdAt: new Date()
			})
			.run();

		const queued = registry.queueCommand(
			registration.agentId,
			{
				name: 'ping',
				payload: { message: 'audit' }
			},
			{ operatorId: 'operator-123' }
		);

		const initialAudit = db
			.select()
			.from(auditEventTable)
			.where(eq(auditEventTable.commandId, queued.command.id))
			.get();

		expect(initialAudit).toBeTruthy();
		expect(initialAudit?.operatorId).toBe('operator-123');
		expect(initialAudit?.executedAt).toBeNull();

		await registry.syncAgent(registration.agentId, registration.agentKey, {
			status: 'online',
			timestamp: new Date().toISOString(),
			results: [
				{
					commandId: queued.command.id,
					success: false,
					completedAt: new Date().toISOString(),
					error: 'Command rejected'
				}
			]
		});

		const finalAudit = db
			.select()
			.from(auditEventTable)
			.where(eq(auditEventTable.commandId, queued.command.id))
			.get();

		expect(finalAudit?.executedAt).toBeInstanceOf(Date);
		expect(finalAudit?.result).toBeTruthy();

		const parsed = finalAudit?.result ? JSON.parse(finalAudit.result) : null;
		expect(parsed?.success).toBe(false);
                expect(parsed?.error).toContain('Command rejected');
        });

        it('rejects remote desktop sessions when the engine plugin is missing', async () => {
                const registry = new AgentRegistry();
                const registration = registry.registerAgent({ metadata: baseMetadata });

                await expect(
                        registry.requireAgentPluginVersion(
                                registration.agentId,
                                remoteDesktopEnginePluginId,
                                requiredRemoteDesktopPluginVersion
                        )
                ).rejects.toThrowError('plugin is not installed');
        });

        it('validates the remote desktop engine plugin version', async () => {
                const registry = new AgentRegistry();
                const registration = registry.registerAgent({ metadata: baseMetadata });

                const manifest = remoteDesktopEngineManifestJson as PluginManifest;
                const expectedHash = manifest.package?.hash ?? '';
                const timestamp = new Date().toISOString();

                await registry.syncAgent(registration.agentId, registration.agentKey, {
                        status: 'online',
                        timestamp,
                        plugins: {
                                installations: [
                                        {
                                                pluginId: remoteDesktopEnginePluginId,
                                                version: '0.0.1',
                                                status: 'installed',
                                                hash: expectedHash,
                                                lastCheckedAt: timestamp,
                                                lastDeployedAt: timestamp,
                                                error: null
                                        }
                                ]
                        }
                });

                await db
                        .update(pluginTable)
                        .set({ approvalStatus: 'approved', approvedAt: new Date(), updatedAt: new Date() })
                        .where(eq(pluginTable.id, remoteDesktopEnginePluginId));

                await db
                        .update(pluginInstallationTable)
                        .set({
                                status: 'installed',
                                error: null,
                                enabled: true,
                                updatedAt: new Date(),
                                version: '0.0.1'
                        })
                        .where(
                                and(
                                        eq(pluginInstallationTable.agentId, registration.agentId),
                                        eq(pluginInstallationTable.pluginId, remoteDesktopEnginePluginId)
                                )
                        );

                await expect(
                        registry.requireAgentPluginVersion(
                                registration.agentId,
                                remoteDesktopEnginePluginId,
                                requiredRemoteDesktopPluginVersion
                        )
                ).rejects.toThrowError(`version ${requiredRemoteDesktopPluginVersion} required`);

                await db
                        .update(pluginInstallationTable)
                        .set({
                                status: 'installed',
                                error: null,
                                enabled: true,
                                updatedAt: new Date(),
                                version: requiredRemoteDesktopPluginVersion
                        })
                        .where(
                                and(
                                        eq(pluginInstallationTable.agentId, registration.agentId),
                                        eq(pluginInstallationTable.pluginId, remoteDesktopEnginePluginId)
                                )
                        );

                await expect(
                        registry.requireAgentPluginVersion(
                                registration.agentId,
                                remoteDesktopEnginePluginId,
                                requiredRemoteDesktopPluginVersion
                        )
                ).resolves.toBeUndefined();
        });

        it('rolls back partial persistence when a transactional error occurs', async () => {
                const registry = new AgentRegistry();
                const registration = registry.registerAgent({ metadata: baseMetadata });

		const queued = registry.queueCommand(registration.agentId, {
			name: 'ping',
			payload: { message: 'rollback' }
		});

		registry.syncSharedNotes(registration.agentId, registration.agentKey, [
			{
				id: 'note-rollback',
				visibility: 'shared',
				ciphertext: 'ciphertext',
				nonce: 'nonce',
				digest: 'digest',
				version: 1,
				updatedAt: new Date().toISOString()
			}
		]);

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

		const originalTransaction = db.transaction.bind(db);
		const consoleSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const transactionSpy = vi
			.spyOn(db, 'transaction')
			.mockImplementation((callback: (tx: unknown) => unknown) =>
				originalTransaction((tx) => {
					let failureInjected = false;
					const proxied = new Proxy(tx, {
						get(target, property, receiver) {
							const value = Reflect.get(target, property, receiver);
							if (property === 'insert') {
								return (...args: unknown[]) => {
									const builder = (value as (...args: unknown[]) => unknown).apply(target, args);
									if (!failureInjected && args[0] === agentResultTable) {
										failureInjected = true;
										return new Proxy(builder as Record<string, unknown>, {
											get(bTarget, bProperty, bReceiver) {
												const bValue = Reflect.get(bTarget, bProperty, bReceiver);
												if (bProperty === 'values') {
													return (...valueArgs: unknown[]) => {
														const nextBuilder = (
															bValue as (...valueArgs: unknown[]) => unknown
														).apply(bTarget, valueArgs);
														return new Proxy(nextBuilder as Record<string, unknown>, {
															get(nTarget, nProperty, nReceiver) {
																const nValue = Reflect.get(nTarget, nProperty, nReceiver);
																if (nProperty === 'run') {
																	return () => {
																		throw new Error('Simulated persistence failure');
																	};
																}
																return typeof nValue === 'function'
																	? (nValue as (...args: unknown[]) => unknown).bind(nTarget)
																	: nValue;
															}
														});
													};
												}
												return typeof bValue === 'function'
													? (bValue as (...args: unknown[]) => unknown).bind(bTarget)
													: bValue;
											}
										});
									}
									return builder;
								};
							}
							return typeof value === 'function'
								? (value as (...args: unknown[]) => unknown).bind(target)
								: value;
						}
					});
					return callback(proxied as typeof db);
				})
			);

		let errorCalls = 0;
		try {
			await registry.flush();
		} finally {
			errorCalls = consoleSpy.mock.calls.length;
			transactionSpy.mockRestore();
			consoleSpy.mockRestore();
		}

		const persistedAgents = await db.select({ id: agentTable.id }).from(agentTable);
		const persistedNotes = await db.select({ id: agentNoteTable.noteId }).from(agentNoteTable);
		const persistedCommands = await db.select({ id: agentCommandTable.id }).from(agentCommandTable);
		const persistedResults = await db
			.select({ id: agentResultTable.commandId })
			.from(agentResultTable);

		expect(errorCalls).toBe(1);
		expect(persistedAgents).toHaveLength(0);
		expect(persistedNotes).toHaveLength(0);
		expect(persistedCommands).toHaveLength(0);
		expect(persistedResults).toHaveLength(0);

		(registry as unknown as { schedulePersist: () => void }).schedulePersist();
		await registry.flush();

		const finalAgents = await db.select({ id: agentTable.id }).from(agentTable);
		expect(finalAgents).toHaveLength(1);
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

		expect(
			eventsA.some((event) => event.type === 'command' && event.command.id === queued.command.id)
		).toBe(true);
		expect(
			eventsB.some((event) => event.type === 'command' && event.command.id === queued.command.id)
		).toBe(true);
		expect(eventsA.some((event) => event.type === 'agent')).toBe(true);
		expect(eventsB.some((event) => event.type === 'notes')).toBe(true);

		unsubscribeA();
		unsubscribeB();

		const before = eventsA.length;
		registry.queueCommand(registration.agentId, { name: 'ping', payload: {} });
		expect(eventsA.length).toBe(before);
	});
});
