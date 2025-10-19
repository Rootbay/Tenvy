import { describe, it, expect, beforeEach, vi } from 'vitest';
import { createHash } from 'crypto';
import { AgentRegistry, MAX_PENDING_COMMANDS, type RegistryBroadcast } from './store';
import { defaultAgentConfig } from '../../../../../shared/types/config';
import type { AgentSnapshot } from '../../../../../shared/types/agent';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';

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

function hashKey(raw: string): string {
	const hash = createHash('sha256');
	hash.update(raw, 'utf-8');
	return hash.digest('hex');
}

const MAX_RECENT_RESULTS = 25;

async function resetRegistryTables() {
	await db.delete(table.agentNote).run();
	await db.delete(table.agentCommand).run();
	await db.delete(table.agentResult).run();
	await db.delete(table.agent).run();
}

describe('AgentRegistry persistence hygiene', () => {
	beforeEach(async () => {
		await resetRegistryTables();
	});

	it('restores persisted agents without clobbering timestamps or configs', async () => {
		const connectedAt = new Date('2024-01-01T00:00:00.000Z');
		const lastSeen = new Date('2024-01-02T12:34:56.000Z');

		await db
			.insert(table.agent)
			.values({
				id: 'agent-1',
				keyHash: hashKey('secret-key'),
				metadata: JSON.stringify(baseMetadata),
				status: 'online',
				connectedAt,
				lastSeen,
				metrics: JSON.stringify({ memoryBytes: 42 }),
				config: JSON.stringify({
					pollIntervalMs: -10,
					maxBackoffMs: 1000,
					jitterRatio: 5
				}),
				fingerprint: fingerprint(baseMetadata),
				createdAt: connectedAt,
				updatedAt: connectedAt
			})
			.run();

		await db
			.insert(table.agentCommand)
			.values({
				id: 'cmd-1',
				agentId: 'agent-1',
				name: 'ping',
				payload: JSON.stringify({}),
				createdAt: connectedAt
			})
			.run();

		await db
			.insert(table.agentResult)
                        .values({
                                agentId: 'agent-1',
                                commandId: 'cmd-1',
                                success: true,
				output: 'ok',
				error: null,
				completedAt: lastSeen,
				createdAt: lastSeen
			})
			.run();

		const registry = new AgentRegistry();

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

		const persistedAgents = db.select().from(table.agent).all();
		expect(persistedAgents).toHaveLength(1);
		expect(persistedAgents[0]?.keyHash).toBe(hashKey('secret-key'));
		const persistedCommands = db.select().from(table.agentCommand).all();
		expect(persistedCommands).toHaveLength(0);
	});

	it('deduplicates recent results and protects registry snapshots from mutation', async () => {
		const registry = new AgentRegistry();
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
		const completedAt = snapshot.recentResults.map((result) => Date.parse(result.completedAt));
		const sortedCompletedAt = [...completedAt].sort((a, b) => b - a);
		expect(completedAt).toEqual(sortedCompletedAt);

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

		const persistedAgents = db.select().from(table.agent).all();
		expect(persistedAgents).toHaveLength(1);
		expect(persistedAgents[0]?.keyHash).toBe(hashKey(registration.agentKey));
	});

	it('caps pending command queues and drops oldest entries when full', () => {
		const registry = new AgentRegistry();
		const registration = registry.registerAgent({ metadata: baseMetadata });

		const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

		try {
			for (let idx = 0; idx < MAX_PENDING_COMMANDS + 5; idx += 1) {
				registry.queueCommand(registration.agentId, {
					name: 'ping',
					payload: { idx }
				});
			}

			const queued = registry.peekCommands(registration.agentId);
			expect(queued).toHaveLength(MAX_PENDING_COMMANDS);
			const firstPayload = queued[0]?.payload as { idx?: number };
			expect(firstPayload?.idx).toBe(5);
			expect(warnSpy).toHaveBeenCalled();
		} finally {
			warnSpy.mockRestore();
		}
	});

	it('requires hashed agent keys for attach and sync flows', async () => {
		const registry = new AgentRegistry();
		const registration = registry.registerAgent({ metadata: baseMetadata });

		const makeSocket = () =>
			({
				readyState: 1,
				addEventListener: vi.fn(),
				send: vi.fn(),
				close: vi.fn()
			}) as unknown as WebSocket;

		const socket = makeSocket();
		registry.attachSession(registration.agentId, registration.agentKey, socket);

		expect(() =>
			registry.attachSession(registration.agentId, 'invalid-key', makeSocket())
		).toThrowError('Invalid agent key');

		expect(() => registry.authorizeAgent(registration.agentId, 'invalid-key')).toThrowError(
			'Invalid agent key'
		);

		expect(() =>
			registry.syncAgent(registration.agentId, 'invalid-key', {
				status: 'online',
				timestamp: new Date().toISOString(),
				results: []
			})
		).toThrowError('Invalid agent key');

		const sync = registry.syncAgent(registration.agentId, registration.agentKey, {
			status: 'online',
			timestamp: new Date().toISOString(),
			results: []
		});
		expect(sync.commands).toHaveLength(0);

		await registry.flush();

		const persistedAgents = db.select().from(table.agent).all();
		expect(persistedAgents[0]?.keyHash).toBe(hashKey(registration.agentKey));
	});

	it('broadcasts registry and activity events to subscribers', () => {
		const registry = new AgentRegistry();
		const events: RegistryBroadcast[] = [];
		const unsubscribe = registry.subscribe((event) => {
			events.push(event);
		});

		try {
			const registration = registry.registerAgent({ metadata: baseMetadata });
			expect(events.at(-1)).toMatchObject({ type: 'agents:snapshot' });

			events.length = 0;
			const command = registry.queueCommand(registration.agentId, { name: 'ping', payload: {} });
			expect(events.some((event) => event.type === 'agent:command-queued')).toBe(true);
			expect(events.some((event) => event.type === 'agents:snapshot')).toBe(true);

			events.length = 0;
			registry.syncSharedNotes(registration.agentId, registration.agentKey, [
				{
					id: 'note-1',
					visibility: 'shared',
					ciphertext: 'cipher',
					nonce: 'nonce',
					digest: 'digest',
					version: 1,
					updatedAt: new Date().toISOString()
				}
			]);
			expect(events).toContainEqual(
				expect.objectContaining({ type: 'agent:notes', agentId: registration.agentId })
			);

			events.length = 0;
			registry.syncAgent(registration.agentId, registration.agentKey, {
				status: 'online',
				timestamp: new Date().toISOString(),
				results: [
					{
						commandId: command.command.id,
						success: true,
						completedAt: new Date().toISOString()
					}
				]
			});

			expect(events.some((event) => event.type === 'agent:command-results')).toBe(true);
			expect(events.some((event) => event.type === 'agents:snapshot')).toBe(true);
		} finally {
			unsubscribe();
		}
	});

	it('persists interleaved mutations without losing state', async () => {
		const registry = new AgentRegistry();
		const registration = registry.registerAgent({ metadata: baseMetadata });

		let queuedCommandId: string | null = null;

		await Promise.all([
			new Promise<void>((resolve) => {
				setTimeout(() => {
					const result = registry.queueCommand(registration.agentId, { name: 'ping', payload: {} });
					queuedCommandId = result.command.id;
					resolve();
				}, 0);
			}),
			new Promise<void>((resolve) => {
				setTimeout(() => {
					registry.updateAgentTags(registration.agentId, ['team']);
					resolve();
				}, 5);
			}),
			new Promise<void>((resolve) => {
				setTimeout(() => {
					registry.syncAgent(registration.agentId, registration.agentKey, {
						status: 'online',
						timestamp: new Date().toISOString(),
						results: queuedCommandId
							? [
									{
										commandId: queuedCommandId,
										success: true,
										completedAt: new Date().toISOString()
									}
								]
							: []
					});
					resolve();
				}, 10);
			})
		]);

		expect(queuedCommandId).not.toBeNull();

		await registry.flush();

		const snapshot = registry.getAgent(registration.agentId);
		expect(snapshot.metadata.tags).toEqual(['team']);
		expect(snapshot.status).toBe('online');
		expect(snapshot.pendingCommands).toBe(0);
		if (queuedCommandId) {
			expect(snapshot.recentResults[0]?.commandId).toBe(queuedCommandId);
		}

		const persistedAgents = db.select().from(table.agent).all();
		expect(persistedAgents).toHaveLength(1);
		const persisted = persistedAgents[0];
		expect(persisted?.status).toBe('online');
		if (persisted?.metadata) {
			const storedMetadata = JSON.parse(persisted.metadata as string) as AgentSnapshot['metadata'];
			expect(storedMetadata.tags).toEqual(['team']);
		}
		const queuedCommands = db.select().from(table.agentCommand).all();
		expect(queuedCommands).toHaveLength(0);
	});
});
