import { createHash, randomBytes, randomUUID, timingSafeEqual } from 'crypto';
import { defaultAgentConfig, type AgentConfig } from '../../../../../shared/types/config';
import type { NoteEnvelope } from '../../../../../shared/types/notes';
import { COMMAND_STREAM_SUBPROTOCOL } from '../../../../../shared/constants/protocol';
import type {
	AgentMetadata,
	AgentMetrics,
	AgentSnapshot,
	AgentStatus
} from '../../../../../shared/types/agent';
import type {
	AgentRegistrationRequest,
	AgentRegistrationResponse
} from '../../../../../shared/types/auth';
import type {
	AgentControlCommandPayload,
	AgentSyncRequest,
	AgentSyncResponse,
	AgentCommandEnvelope,
	AgentRemoteDesktopInputEnvelope,
	AgentAppVncInputEnvelope,
	Command,
	CommandDeliveryMode,
	CommandInput,
	CommandQueueResponse,
	CommandResult
} from '../../../../../shared/types/messages';
import type { RemoteDesktopInputBurst } from '../../../../../shared/types/remote-desktop';
import type { AppVncInputBurst } from '../../../../../shared/types/app-vnc';
import { db, type DatabaseClient } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { asc, desc } from 'drizzle-orm';

const MAX_TAGS = 16;
const MAX_TAG_LENGTH = 32;
const TAG_PATTERN = /^[\p{L}\p{N}_\-\s]+$/u;

const MAX_RECENT_RESULTS = 25;
export const MAX_PENDING_COMMANDS = 200;
const PENDING_COMMAND_DROP_WARN_INTERVAL_MS = 30_000;
const PERSIST_DEBOUNCE_MS = 2_000;

interface AgentRegistryOptions {
	db?: DatabaseClient;
}

const SOCKET_OPEN_STATE = (() => {
	const globalSocket = (globalThis as { WebSocket?: { OPEN?: number } }).WebSocket;
	if (globalSocket && typeof globalSocket.OPEN === 'number') {
		return globalSocket.OPEN;
	}
	return 1;
})();

class RegistryError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'RegistryError';
		this.status = status;
	}
}

interface AgentSessionRecord {
	id: symbol;
	socket: WebSocket;
}

interface AgentRecord {
	id: string;
	keyHash: string;
	metadata: AgentMetadata;
	status: AgentStatus;
	connectedAt: Date;
	lastSeen: Date;
	metrics?: AgentMetrics;
	config: AgentConfig;
	pendingCommands: Command[];
	recentResults: CommandResult[];
	sharedNotes: Map<string, SharedNoteRecord>;
	fingerprint: string;
	session?: AgentSessionRecord;
	lastQueueDropWarning?: number;
}

interface SharedNoteRecord {
	id: string;
	ciphertext: string;
	nonce: string;
	digest: string;
	version: number;
	updatedAt: Date;
}

function ensureMetadata(metadata: AgentMetadata, remoteAddress?: string): AgentMetadata {
	if (!remoteAddress) {
		return metadata;
	}

	const next: AgentMetadata = { ...metadata };

	if (!next.ipAddress) {
		next.ipAddress = remoteAddress;
	}

	if (!next.publicIpAddress || next.publicIpAddress.trim() === '') {
		next.publicIpAddress = remoteAddress;
	}

	return next;
}

function validateToken(requestToken: string | undefined) {
	const expected = process.env.TENVY_SHARED_SECRET;
	if (expected && expected !== requestToken) {
		throw new RegistryError('Invalid registration token', 401);
	}
}

function computeFingerprint(metadata: AgentMetadata): string {
	const normalize = (value: string | undefined) => value?.trim().toLowerCase() ?? '';
	const hash = createHash('sha256');
	hash.update(normalize(metadata.hostname));
	hash.update('|');
	hash.update(normalize(metadata.username));
	hash.update('|');
	hash.update(normalize(metadata.os));
	hash.update('|');
	hash.update(normalize(metadata.architecture));
	hash.update('|');
	hash.update(normalize(metadata.group));
	return hash.digest('hex');
}

function hashAgentKey(rawKey: string): string {
	const hash = createHash('sha256');
	hash.update(rawKey, 'utf-8');
	return hash.digest('hex');
}

function timingSafeEqualHex(expected: string, candidate: string): boolean {
	if (expected.length !== candidate.length) {
		return false;
	}

	try {
		const expectedBuffer = Buffer.from(expected, 'hex');
		const candidateBuffer = Buffer.from(candidate, 'hex');
		return timingSafeEqual(expectedBuffer, candidateBuffer);
	} catch {
		return false;
	}
}

function generateAgentKey(): { token: string; hash: string } {
	const token = randomBytes(32).toString('hex');
	return { token, hash: hashAgentKey(token) };
}

function parseNumeric(value: unknown): number | null {
	if (typeof value === 'number') {
		return Number.isFinite(value) ? value : null;
	}
	if (typeof value === 'string' && value.trim() !== '') {
		const parsed = Number(value);
		return Number.isFinite(parsed) ? parsed : null;
	}
	return null;
}

function parseJson<T>(value: unknown): T | null {
	if (value === null || value === undefined) {
		return null;
	}
	if (typeof value === 'string') {
		if (value.trim() === '') {
			return null;
		}
		try {
			return JSON.parse(value) as T;
		} catch {
			return null;
		}
	}
	if (typeof value === 'object') {
		return value as T;
	}
	return null;
}

function normalizeConfig(config?: Partial<AgentConfig> | null): AgentConfig {
	const normalized: AgentConfig = {
		...defaultAgentConfig
	};

	if (!config) {
		return normalized;
	}

	const pollInterval = parseNumeric(config.pollIntervalMs);
	if (pollInterval !== null && pollInterval > 0) {
		normalized.pollIntervalMs = Math.max(1, Math.round(pollInterval));
	}

	const maxBackoff = parseNumeric(config.maxBackoffMs);
	if (maxBackoff !== null && maxBackoff > 0) {
		normalized.maxBackoffMs = Math.max(normalized.pollIntervalMs, Math.round(maxBackoff));
	}

	const jitter = parseNumeric(config.jitterRatio);
	if (jitter !== null && jitter >= 0 && jitter <= 1) {
		normalized.jitterRatio = jitter;
	}

	return normalized;
}

function cloneMetadata(metadata: AgentMetadata): AgentMetadata {
	const clone: AgentMetadata = { ...metadata };
	if (Array.isArray(metadata.tags)) {
		clone.tags = [...metadata.tags];
	}
	if (metadata.location) {
		clone.location = { ...metadata.location };
	}
	return clone;
}

function cloneMetrics(metrics: AgentMetrics | undefined): AgentMetrics | undefined {
	return metrics ? { ...metrics } : undefined;
}

function parseCompletedAt(result: CommandResult | undefined): number {
	if (!result) {
		return 0;
	}
	if (typeof result.completedAt === 'string') {
		const parsed = Date.parse(result.completedAt);
		if (Number.isFinite(parsed)) {
			return parsed;
		}
	}
	return 0;
}

function mergeRecentResults(existing: CommandResult[], incoming: CommandResult[]): CommandResult[] {
	if (existing.length === 0 && incoming.length === 0) {
		return [];
	}

	const merged = new Map<string, { result: CommandResult; timestamp: number }>();

	const upsert = (candidate: CommandResult | null | undefined) => {
		if (!candidate?.commandId) {
			return;
		}
		const timestamp = parseCompletedAt(candidate);
		const current = merged.get(candidate.commandId);
		if (!current || timestamp >= current.timestamp) {
			merged.set(candidate.commandId, {
				result: { ...candidate },
				timestamp
			});
		}
	};

	for (const result of existing) {
		upsert(result);
	}

	for (const result of incoming) {
		upsert(result);
	}

	return Array.from(merged.values())
		.sort((a, b) => {
			if (b.timestamp !== a.timestamp) {
				return b.timestamp - a.timestamp;
			}
			return b.result.commandId.localeCompare(a.result.commandId);
		})
		.slice(0, MAX_RECENT_RESULTS)
		.map((entry) => entry.result);
}

type RegistryBroadcastEvent =
	| { type: 'agents:snapshot'; agents: AgentSnapshot[] }
	| { type: 'agent:notes'; agentId: string; notes: NoteEnvelope[] }
	| {
			type: 'agent:command-queued';
			agentId: string;
			command: Command;
			delivery: CommandDeliveryMode;
	  }
	| { type: 'agent:command-results'; agentId: string; results: CommandResult[] };

type RegistryListener = (event: RegistryBroadcastEvent) => void;

export class AgentRegistry {
	private readonly db: DatabaseClient;
	private readonly agents = new Map<string, AgentRecord>();
	private readonly fingerprints = new Map<string, string>();
	private readonly listeners = new Set<RegistryListener>();
	private persistTimer: ReturnType<typeof setTimeout> | null = null;
	private persistPromise: Promise<void> | null = null;
	private needsPersist = false;

	constructor(options: AgentRegistryOptions = {}) {
		this.db = options.db ?? db;
		this.loadFromDatabase();
	}

	subscribe(listener: RegistryListener): () => void {
		this.listeners.add(listener);
		return () => {
			this.listeners.delete(listener);
		};
	}

	private broadcast(event: RegistryBroadcastEvent): void {
		for (const listener of this.listeners) {
			try {
				listener(event);
			} catch (error) {
				console.error('Registry listener failed', error);
			}
		}
	}

	private broadcastAgentsSnapshot(): void {
		this.broadcast({ type: 'agents:snapshot', agents: this.listAgents() });
	}

	private verifyAgentKey(record: AgentRecord, key: string | undefined): boolean {
		if (!key) {
			return false;
		}

		const incomingHash = hashAgentKey(key);
		return timingSafeEqualHex(record.keyHash, incomingHash);
	}

	private loadFromDatabase(): void {
		const agentRows = this.db.select().from(table.agent).orderBy(asc(table.agent.createdAt)).all();
		const commandRows = this.db
			.select()
			.from(table.agentCommand)
			.orderBy(asc(table.agentCommand.createdAt))
			.all();
		const noteRows = this.db
			.select()
			.from(table.agentNote)
			.orderBy(desc(table.agentNote.updatedAt))
			.all();
		const resultRows = this.db
			.select()
			.from(table.agentResult)
			.orderBy(desc(table.agentResult.completedAt))
			.all();

		this.agents.clear();
		this.fingerprints.clear();

		const commandsByAgent = new Map<string, Command[]>();
		for (const row of commandRows) {
			const payload = parseJson<Command['payload']>(row.payload) ?? {};
			const createdAt = row.createdAt ?? new Date();
			const command: Command = {
				id: row.id,
				name: row.name as Command['name'],
				payload,
				createdAt: createdAt.toISOString()
			};
			const queue = commandsByAgent.get(row.agentId);
			if (queue) {
				queue.push(command);
			} else {
				commandsByAgent.set(row.agentId, [command]);
			}
		}

		const notesByAgent = new Map<string, Map<string, SharedNoteRecord>>();
		for (const row of noteRows) {
			const updatedAt = row.updatedAt ?? new Date();
			let bucket = notesByAgent.get(row.agentId);
			if (!bucket) {
				bucket = new Map();
				notesByAgent.set(row.agentId, bucket);
			}
			bucket.set(row.id, {
				id: row.id,
				ciphertext: row.ciphertext,
				nonce: row.nonce,
				digest: row.digest,
				version: row.version ?? 1,
				updatedAt
			});
		}

		const resultsByAgent = new Map<string, CommandResult[]>();
		for (const row of resultRows) {
			const result: CommandResult = {
				commandId: row.commandId,
				success: Boolean(row.success),
				output: row.output ?? undefined,
				error: row.error ?? undefined,
				completedAt: (row.completedAt ?? new Date()).toISOString()
			};
			const list = resultsByAgent.get(row.agentId) ?? [];
			if (list.length < MAX_RECENT_RESULTS) {
				list.push(result);
				resultsByAgent.set(row.agentId, list);
			}
		}

		for (const row of agentRows) {
			const metadata = parseJson<AgentMetadata>(row.metadata) ?? ({} as AgentMetadata);
			const config = normalizeConfig(parseJson<AgentConfig>(row.config) ?? null);
			const metrics = parseJson<AgentMetrics>(row.metrics) ?? undefined;
			const connectedAt = row.connectedAt ?? new Date();
			let lastSeen = row.lastSeen ?? connectedAt;
			let status = (row.status as AgentStatus) ?? 'offline';
			if (status === 'online') {
				status = 'offline';
				if (lastSeen.getTime() < connectedAt.getTime()) {
					lastSeen = connectedAt;
				}
			}
			const normalizedMetadata: AgentMetadata = {
				...metadata,
				tags: Array.isArray(metadata.tags) ? this.normalizeTags(metadata.tags) : metadata.tags
			};

			const record: AgentRecord = {
				id: row.id,
				keyHash: row.keyHash,
				metadata: normalizedMetadata,
				status,
				connectedAt,
				lastSeen,
				metrics,
				config,
				pendingCommands: [...(commandsByAgent.get(row.id) ?? [])],
				recentResults: [...(resultsByAgent.get(row.id) ?? [])],
				sharedNotes: notesByAgent.get(row.id) ?? new Map(),
				fingerprint: row.fingerprint
			};

			if (record.pendingCommands.length > MAX_PENDING_COMMANDS) {
				record.pendingCommands = record.pendingCommands.slice(-MAX_PENDING_COMMANDS);
			}

			this.agents.set(record.id, record);
			this.fingerprints.set(record.fingerprint, record.id);
		}
	}

	private schedulePersist(): void {
		this.needsPersist = true;
		if (this.persistPromise) {
			return;
		}
		if (this.persistTimer) {
			return;
		}
		this.persistTimer = setTimeout(() => {
			this.persistTimer = null;
			this.persistPromise = this.flushPersistLoop();
		}, PERSIST_DEBOUNCE_MS);
	}

	private async flushPersistLoop(): Promise<void> {
		try {
			while (this.needsPersist) {
				this.needsPersist = false;
				try {
					await this.persistToDatabase();
				} catch (error) {
					console.error('Failed to persist agent registry', error);
				}
			}
		} finally {
			this.persistPromise = null;
		}
	}

	private async persistToDatabase(): Promise<void> {
		const agents = Array.from(this.agents.values());
		const now = new Date();

		await this.db.transaction((tx) => {
			tx.delete(table.agentNote).run();
			tx.delete(table.agentCommand).run();
			tx.delete(table.agentResult).run();
			tx.delete(table.agent).run();

			if (agents.length === 0) {
				return;
			}

			tx.insert(table.agent)
				.values(
					agents.map((record) => ({
						id: record.id,
						keyHash: record.keyHash,
						metadata: JSON.stringify(record.metadata),
						status: record.status,
						connectedAt: record.connectedAt,
						lastSeen: record.lastSeen,
						metrics: record.metrics ? JSON.stringify(record.metrics) : null,
						config: JSON.stringify(record.config),
						fingerprint: record.fingerprint,
						createdAt: record.connectedAt,
						updatedAt: now
					}))
				)
				.run();

			const notes: Array<typeof table.agentNote.$inferInsert> = [];
			const commands: Array<typeof table.agentCommand.$inferInsert> = [];
			const results: Array<typeof table.agentResult.$inferInsert> = [];

			for (const record of agents) {
				for (const note of record.sharedNotes.values()) {
					notes.push({
						id: note.id,
						agentId: record.id,
						ciphertext: note.ciphertext,
						nonce: note.nonce,
						digest: note.digest,
						version: note.version,
						updatedAt: note.updatedAt
					});
				}

				for (const command of record.pendingCommands) {
					commands.push({
						id: command.id,
						agentId: record.id,
						name: command.name,
						payload: JSON.stringify(command.payload),
						createdAt: new Date(command.createdAt)
					});
				}

				for (const result of record.recentResults) {
					results.push({
						agentId: record.id,
						commandId: result.commandId,
						success: result.success,
						output: result.output ?? null,
						error: result.error ?? null,
						completedAt: new Date(result.completedAt),
						createdAt: new Date(result.completedAt)
					});
				}
			}

			if (notes.length > 0) {
				tx.insert(table.agentNote).values(notes).run();
			}

			if (commands.length > 0) {
				tx.insert(table.agentCommand).values(commands).run();
			}

			if (results.length > 0) {
				tx.insert(table.agentResult).values(results).run();
			}
		});
	}

	async flush(): Promise<void> {
		if (this.persistTimer) {
			clearTimeout(this.persistTimer);
			this.persistTimer = null;
		}

		if (!this.persistPromise && this.needsPersist) {
			this.persistPromise = this.flushPersistLoop();
		}

		if (this.persistPromise) {
			await this.persistPromise;
		}
	}

	private toSnapshot(record: AgentRecord): AgentSnapshot {
		return {
			id: record.id,
			metadata: cloneMetadata(record.metadata),
			status: record.status,
			connectedAt: record.connectedAt.toISOString(),
			lastSeen: record.lastSeen.toISOString(),
			metrics: cloneMetrics(record.metrics),
			pendingCommands: record.pendingCommands.length,
			recentResults: record.recentResults.map((result) => ({ ...result })),
			liveSession: Boolean(record.session)
		} satisfies AgentSnapshot;
	}

	private detachSession(
		record: AgentRecord,
		sessionId: symbol,
		options: { close?: boolean; code?: number; reason?: string; markOffline?: boolean } = {}
	) {
		const session = record.session;
		if (!session || session.id !== sessionId) {
			return;
		}

		record.session = undefined;

		if (options.markOffline !== false) {
			record.status = 'offline';
			record.lastSeen = new Date();
			this.schedulePersist();
			this.broadcastAgentsSnapshot();
		}

		if (options.close === false) {
			return;
		}

		try {
			session.socket.close(options.code ?? 1000, options.reason);
		} catch {
			// Ignore close failures.
		}
	}

	private deliverViaSession(record: AgentRecord, command: Command): boolean {
		const session = record.session;
		if (!session) {
			return false;
		}

		const socket = session.socket;
		if (!socket || (socket.readyState ?? 0) !== SOCKET_OPEN_STATE) {
			this.detachSession(record, session.id, { close: false });
			return false;
		}

		try {
			const envelope: AgentCommandEnvelope = { type: 'command', command };
			socket.send(JSON.stringify(envelope));
			return true;
		} catch {
			this.detachSession(record, session.id, { close: false });
			return false;
		}
	}

	private clampPendingCommands(record: AgentRecord, dropFrom: 'front' | 'back' = 'front'): void {
		const overflow = record.pendingCommands.length - MAX_PENDING_COMMANDS;
		if (overflow <= 0) {
			return;
		}

		if (dropFrom === 'back') {
			record.pendingCommands.splice(record.pendingCommands.length - overflow, overflow);
			this.warnPendingCommandDrop(record, overflow, dropFrom);
			return;
		}

		record.pendingCommands.splice(0, overflow);
		this.warnPendingCommandDrop(record, overflow, dropFrom);
	}

	private warnPendingCommandDrop(
		record: AgentRecord,
		dropped: number,
		dropFrom: 'front' | 'back'
	): void {
		if (dropped <= 0) {
			return;
		}

		const now = Date.now();
		if (
			record.lastQueueDropWarning &&
			now - record.lastQueueDropWarning < PENDING_COMMAND_DROP_WARN_INTERVAL_MS
		) {
			return;
		}

		record.lastQueueDropWarning = now;
		const direction = dropFrom === 'front' ? 'oldest' : 'newest';
		const plural = dropped === 1 ? '' : 's';
		console.warn(
			`Pending command queue for agent ${record.id} reached capacity (${MAX_PENDING_COMMANDS}); dropped ${dropped} ${direction} command${plural}.`
		);
	}

	registerAgent(
		payload: AgentRegistrationRequest,
		options: { remoteAddress?: string } = {}
	): AgentRegistrationResponse {
		validateToken(payload.token);
		const now = new Date();
		const normalizedTags = this.normalizeTags(payload.metadata.tags ?? []);
		const incomingMetadata = ensureMetadata(
			{ ...payload.metadata, tags: normalizedTags.length > 0 ? normalizedTags : undefined },
			options.remoteAddress
		);
		const fingerprint = computeFingerprint(incomingMetadata);

		const existingId = this.fingerprints.get(fingerprint);
		if (existingId) {
			const existingRecord = this.agents.get(existingId);
			if (existingRecord) {
				if (existingRecord.session) {
					this.detachSession(existingRecord, existingRecord.session.id, {
						code: 1012,
						reason: 'Registration superseded active session',
						markOffline: false
					});
				}

				const hasExplicitTags = Array.isArray(existingRecord.metadata.tags);
				const nextMetadata: AgentMetadata = ensureMetadata(
					{
						...existingRecord.metadata,
						...incomingMetadata,
						tags: hasExplicitTags ? existingRecord.metadata.tags : incomingMetadata.tags
					},
					options.remoteAddress
				);

				const previousFingerprint = existingRecord.fingerprint;
				existingRecord.metadata = nextMetadata;
				existingRecord.status = 'online';
				existingRecord.connectedAt = now;
				existingRecord.lastSeen = now;
				existingRecord.metrics = undefined;
				const nextKey = generateAgentKey();
				existingRecord.keyHash = nextKey.hash;
				existingRecord.config = normalizeConfig(existingRecord.config);
				existingRecord.fingerprint = computeFingerprint(nextMetadata);

				if (previousFingerprint !== existingRecord.fingerprint) {
					this.fingerprints.delete(previousFingerprint);
				}
				this.fingerprints.set(existingRecord.fingerprint, existingRecord.id);
				this.agents.set(existingRecord.id, existingRecord);
				this.schedulePersist();
				this.broadcastAgentsSnapshot();

				return {
					agentId: existingRecord.id,
					agentKey: nextKey.token,
					config: { ...existingRecord.config },
					commands: [],
					serverTime: now.toISOString()
				};
			}

			this.fingerprints.delete(fingerprint);
		}

		const id = randomUUID();
		const nextKey = generateAgentKey();
		const record: AgentRecord = {
			id,
			keyHash: nextKey.hash,
			metadata: incomingMetadata,
			status: 'online',
			connectedAt: now,
			lastSeen: now,
			metrics: undefined,
			config: normalizeConfig(null),
			pendingCommands: [],
			recentResults: [],
			sharedNotes: new Map(),
			fingerprint
		};

		this.agents.set(id, record);
		this.fingerprints.set(fingerprint, id);
		this.schedulePersist();
		this.broadcastAgentsSnapshot();

		return {
			agentId: id,
			agentKey: nextKey.token,
			config: { ...record.config },
			commands: [],
			serverTime: now.toISOString()
		};
	}

	attachSession(
		id: string,
		key: string | undefined,
		socket: WebSocket,
		options: { remoteAddress?: string } = {}
	): void {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		if (!this.verifyAgentKey(record, key)) {
			throw new RegistryError('Invalid agent key', 401);
		}

		const sessionId = Symbol(`agent:${id}`);

		if (record.session) {
			this.detachSession(record, record.session.id, {
				code: 1012,
				reason: 'Session replaced',
				markOffline: false
			});
		}

		record.lastSeen = new Date();
		record.status = 'online';

		const acceptingSocket = socket as unknown as {
			accept?: (options?: { protocol?: string }) => void;
		};
		if (typeof acceptingSocket.accept === 'function') {
			try {
				acceptingSocket.accept({ protocol: COMMAND_STREAM_SUBPROTOCOL });
			} catch {
				// Ignore accept failures; send will surface errors later.
			}
		}

		const closeListener = () => {
			this.detachSession(record, sessionId, { close: false });
		};

		if (typeof socket.addEventListener === 'function') {
			socket.addEventListener('close', closeListener);
			socket.addEventListener('error', closeListener);
		} else {
			// Bun exposes onclose/onerror; fall back to direct assignment when listeners are unavailable.
			(socket as unknown as { onclose?: () => void }).onclose = closeListener;
			(socket as unknown as { onerror?: () => void }).onerror = closeListener;
		}

		record.session = { id: sessionId, socket };

		if (options.remoteAddress) {
			record.metadata = ensureMetadata(record.metadata, options.remoteAddress);
		}

		if (record.pendingCommands.length > 0) {
			const queued = record.pendingCommands;
			record.pendingCommands = [];
			for (let idx = 0; idx < queued.length; idx += 1) {
				const command = queued[idx];
				if (!this.deliverViaSession(record, command)) {
					record.pendingCommands = queued.slice(idx);
					this.clampPendingCommands(record, 'front');
					break;
				}
			}
		}

		this.schedulePersist();
		this.broadcastAgentsSnapshot();
	}

	syncAgent(
		id: string,
		key: string | undefined,
		payload: AgentSyncRequest,
		options: { remoteAddress?: string } = {}
	): AgentSyncResponse {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		if (!this.verifyAgentKey(record, key)) {
			throw new RegistryError('Invalid agent key', 401);
		}

		record.lastSeen = new Date();
		record.status = payload.status;

		if (options.remoteAddress) {
			record.metadata = ensureMetadata(record.metadata, options.remoteAddress);
		}
		if (payload.metrics) {
			record.metrics = { ...payload.metrics };
		}
		if (payload.results && payload.results.length > 0) {
			record.recentResults = mergeRecentResults(record.recentResults, payload.results);
		}

		const commands = record.pendingCommands.map((command) => ({ ...command }));
		record.pendingCommands = [];

		this.schedulePersist();
		this.broadcastAgentsSnapshot();
		if (payload.results && payload.results.length > 0) {
			this.broadcast({
				type: 'agent:command-results',
				agentId: id,
				results: payload.results.map((result) => ({ ...result }))
			});
		}

		return {
			agentId: id,
			commands,
			config: { ...record.config },
			serverTime: new Date().toISOString()
		};
	}

	queueCommand(id: string, input: CommandInput): CommandQueueResponse {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		const command: Command = {
			id: randomUUID(),
			name: input.name,
			payload: input.payload,
			createdAt: new Date().toISOString()
		};

		const delivered = this.deliverViaSession(record, command);
		if (!delivered) {
			record.pendingCommands.push(command);
			this.clampPendingCommands(record, 'front');
			this.schedulePersist();
			this.broadcastAgentsSnapshot();
		}

		const delivery: CommandDeliveryMode = delivered ? 'session' : 'queued';
		this.broadcast({
			type: 'agent:command-queued',
			agentId: id,
			command: { ...command },
			delivery
		});
		return { command, delivery };
	}

	sendRemoteDesktopInput(id: string, burst: RemoteDesktopInputBurst): boolean {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		const session = record.session;
		if (!session) {
			return false;
		}

		const socket = session.socket;
		if (!socket || (socket.readyState ?? 0) !== SOCKET_OPEN_STATE) {
			this.detachSession(record, session.id, { close: false });
			return false;
		}

		const envelope: AgentRemoteDesktopInputEnvelope = {
			type: 'remote-desktop-input',
			input: {
				sessionId: burst.sessionId,
				events: burst.events,
				sequence: burst.sequence
			}
		};

		try {
			socket.send(JSON.stringify(envelope));
			return true;
		} catch (err) {
			this.detachSession(record, session.id, { close: false });
			console.error('Failed to transmit remote desktop input burst', err);
			return false;
		}
	}

	sendAppVncInput(id: string, burst: AppVncInputBurst): boolean {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		const session = record.session;
		if (!session) {
			return false;
		}

		const socket = session.socket;
		if (!socket || (socket.readyState ?? 0) !== SOCKET_OPEN_STATE) {
			this.detachSession(record, session.id, { close: false });
			return false;
		}

		const envelope: AgentAppVncInputEnvelope = {
			type: 'app-vnc-input',
			input: {
				sessionId: burst.sessionId,
				events: burst.events,
				sequence: burst.sequence
			}
		};

		try {
			socket.send(JSON.stringify(envelope));
			return true;
		} catch (err) {
			this.detachSession(record, session.id, { close: false });
			console.error('Failed to transmit app VNC input burst', err);
			return false;
		}
	}

	disconnectAgent(id: string): AgentSnapshot {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		record.status = 'offline';
		record.lastSeen = new Date();
		const payload: AgentControlCommandPayload = { action: 'disconnect' };
		const command: Command = {
			id: randomUUID(),
			name: 'agent-control',
			payload,
			createdAt: new Date().toISOString()
		};

		record.pendingCommands = [];
		const delivered = this.deliverViaSession(record, command);
		if (!delivered) {
			record.pendingCommands.push(command);
			this.clampPendingCommands(record, 'front');
		}

		this.schedulePersist();
		this.broadcastAgentsSnapshot();
		const delivery: CommandDeliveryMode = delivered ? 'session' : 'queued';
		this.broadcast({
			type: 'agent:command-queued',
			agentId: id,
			command: { ...command },
			delivery
		});

		return this.toSnapshot(record);
	}

	private normalizeTags(tags: string[]): string[] {
		const seen = new Set<string>();
		const result: string[] = [];

		for (const entry of tags) {
			if (typeof entry !== 'string') {
				continue;
			}

			const trimmed = entry.trim();
			if (trimmed.length === 0 || trimmed.length > MAX_TAG_LENGTH) {
				continue;
			}

			if (!TAG_PATTERN.test(trimmed)) {
				continue;
			}

			const key = trimmed.toLowerCase();
			if (seen.has(key)) {
				continue;
			}

			seen.add(key);
			result.push(trimmed);

			if (result.length >= MAX_TAGS) {
				break;
			}
		}

		return result;
	}

	reconnectAgent(id: string): AgentSnapshot {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		const now = new Date();
		record.status = 'online';
		record.connectedAt = now;
		record.lastSeen = now;

		const payload: AgentControlCommandPayload = { action: 'reconnect' };
		const command: Command = {
			id: randomUUID(),
			name: 'agent-control',
			payload,
			createdAt: now.toISOString()
		};

		const delivered = this.deliverViaSession(record, command);
		if (!delivered) {
			record.pendingCommands.unshift(command);
			this.clampPendingCommands(record, 'back');
		}

		this.schedulePersist();
		this.broadcastAgentsSnapshot();
		const delivery: CommandDeliveryMode = delivered ? 'session' : 'queued';
		this.broadcast({
			type: 'agent:command-queued',
			agentId: id,
			command: { ...command },
			delivery
		});

		return this.toSnapshot(record);
	}

	updateAgentTags(id: string, tags: string[]): AgentSnapshot {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		record.metadata = {
			...record.metadata,
			tags: this.normalizeTags(Array.isArray(tags) ? tags : [])
		};

		this.schedulePersist();
		this.broadcastAgentsSnapshot();

		return this.toSnapshot(record);
	}

	listAgents(): AgentSnapshot[] {
		return Array.from(this.agents.values()).map((record) => this.toSnapshot(record));
	}

	getAgent(id: string): AgentSnapshot {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}
		return this.toSnapshot(record);
	}

	authorizeAgent(id: string, key: string | undefined): void {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}
		if (!this.verifyAgentKey(record, key)) {
			throw new RegistryError('Invalid agent key', 401);
		}
		record.lastSeen = new Date();
	}

	peekCommands(id: string): Command[] {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}
		return [...record.pendingCommands];
	}

	syncSharedNotes(id: string, key: string | undefined, payload: NoteEnvelope[]): NoteEnvelope[] {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		if (!this.verifyAgentKey(record, key)) {
			throw new RegistryError('Invalid agent key', 401);
		}

		const now = new Date();
		let changed = false;
		for (const envelope of payload) {
			if (!envelope?.id) {
				continue;
			}
			const incomingUpdated = new Date(envelope.updatedAt ?? now.toISOString());
			const existing = record.sharedNotes.get(envelope.id);

			if (!existing) {
				record.sharedNotes.set(envelope.id, {
					id: envelope.id,
					ciphertext: envelope.ciphertext,
					nonce: envelope.nonce,
					digest: envelope.digest,
					version: envelope.version,
					updatedAt: incomingUpdated
				});
				changed = true;
				continue;
			}

			const shouldReplace =
				incomingUpdated.getTime() > existing.updatedAt.getTime() ||
				envelope.version > existing.version;

			if (shouldReplace) {
				existing.ciphertext = envelope.ciphertext;
				existing.nonce = envelope.nonce;
				existing.digest = envelope.digest;
				existing.version = envelope.version;
				existing.updatedAt = incomingUpdated;
				changed = true;
			}
		}

		const notes = Array.from(record.sharedNotes.values()).map(
			(note) =>
				({
					id: note.id,
					visibility: 'shared',
					ciphertext: note.ciphertext,
					nonce: note.nonce,
					digest: note.digest,
					version: note.version,
					updatedAt: note.updatedAt.toISOString()
				}) satisfies NoteEnvelope
		);

		if (changed) {
			this.schedulePersist();
			this.broadcast({
				type: 'agent:notes',
				agentId: id,
				notes: notes.map((note) => ({ ...note }))
			});
		}

		return notes;
	}
}

export const registry = new AgentRegistry();
export { RegistryError };
export type { RegistryBroadcastEvent as RegistryBroadcast };
