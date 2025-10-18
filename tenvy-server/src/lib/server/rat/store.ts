import { createHash, randomBytes, randomUUID } from 'crypto';
import { readFileSync } from 'fs';
import { mkdir, rename, rm, writeFile } from 'fs/promises';
import path from 'path';
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

const MAX_TAGS = 16;
const MAX_TAG_LENGTH = 32;
const TAG_PATTERN = /^[\p{L}\p{N}_\-\s]+$/u;

const MAX_RECENT_RESULTS = 25;
export const MAX_PENDING_COMMANDS = 200;
const PENDING_COMMAND_DROP_WARN_INTERVAL_MS = 30_000;
const PERSIST_DEBOUNCE_MS = 2_000;
const PERSIST_FILE_VERSION = 1;

const DEFAULT_STORAGE_PATH = process.env.TENVY_AGENT_REGISTRY_PATH
	? path.resolve(process.env.TENVY_AGENT_REGISTRY_PATH)
	: path.join(process.cwd(), 'var', 'registry', 'clients.json');

interface PersistedSharedNoteRecord {
	id: string;
	ciphertext: string;
	nonce: string;
	digest: string;
	version: number;
	updatedAt: string;
}

interface PersistedAgentRecord {
	id: string;
	key: string;
	metadata: AgentMetadata;
	status: AgentStatus;
	connectedAt: string;
	lastSeen: string;
	metrics?: AgentMetrics;
	config: AgentConfig;
	pendingCommands: Command[];
	recentResults: CommandResult[];
	sharedNotes: PersistedSharedNoteRecord[];
	fingerprint: string;
}

interface PersistedRegistryFile {
	version: number;
	agents: PersistedAgentRecord[];
}

interface AgentRegistryOptions {
	storagePath?: string;
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
	key: string;
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

function resolveStoragePath(storagePath?: string): string {
	if (storagePath && storagePath.trim() !== '') {
		return path.resolve(storagePath);
	}
	return DEFAULT_STORAGE_PATH;
}

async function ensureParentDirectory(filePath: string): Promise<void> {
	const directory = path.dirname(filePath);
	await mkdir(directory, { recursive: true });
}

async function writeFileAtomic(destination: string, data: string): Promise<void> {
	const tempPath = `${destination}.${randomUUID()}.tmp`;
	try {
		await writeFile(tempPath, data, 'utf-8');
		await rename(tempPath, destination);
	} catch (error) {
		try {
			await rm(tempPath, { force: true });
		} catch {
			// no-op cleanup failure
		}
		throw error;
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

function parsePersistedDate(value: unknown, fallback: Date): Date {
	if (typeof value === 'string') {
		const parsed = new Date(value);
		if (!Number.isNaN(parsed.getTime())) {
			return parsed;
		}
	}
	return fallback;
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

export class AgentRegistry {
	private readonly storagePath: string;
	private readonly agents = new Map<string, AgentRecord>();
	private readonly fingerprints = new Map<string, string>();
	private persistTimer: ReturnType<typeof setTimeout> | null = null;
	private persistPromise: Promise<void> | null = null;
	private needsPersist = false;

	constructor(options: AgentRegistryOptions = {}) {
		this.storagePath = resolveStoragePath(options.storagePath);
		this.loadFromDisk();
	}

	private loadFromDisk(): void {
		let source: string;
		try {
			source = readFileSync(this.storagePath, 'utf-8');
		} catch (error) {
			const err = error as NodeJS.ErrnoException;
			if (err.code !== 'ENOENT') {
				console.error('Failed to read agent registry from disk', err);
			}
			return;
		}

		if (!source || source.trim() === '') {
			return;
		}

		let parsed: unknown;
		try {
			parsed = JSON.parse(source);
		} catch (error) {
			console.error('Agent registry file is not valid JSON', error);
			return;
		}

		const file = parsed as Partial<PersistedRegistryFile> | null;
		if (!file || typeof file !== 'object' || !Array.isArray(file.agents)) {
			return;
		}

		this.agents.clear();
		this.fingerprints.clear();

		for (const entry of file.agents) {
			if (!entry || typeof entry !== 'object') {
				continue;
			}

			const id = typeof entry.id === 'string' && entry.id.trim() !== '' ? entry.id : null;
			const key = typeof entry.key === 'string' && entry.key.trim() !== '' ? entry.key : null;
			const metadata = entry.metadata ?? null;
			const status = entry.status;
			if (!id || !key || !metadata) {
				continue;
			}
			if (status !== 'online' && status !== 'offline' && status !== 'error') {
				continue;
			}

			const connectedAt = parsePersistedDate(entry.connectedAt, new Date());
			let lastSeen = parsePersistedDate(entry.lastSeen, connectedAt);
			let normalizedStatus = status as AgentStatus;
			if (normalizedStatus === 'online') {
				normalizedStatus = 'offline';
				if (lastSeen.getTime() < connectedAt.getTime()) {
					lastSeen = connectedAt;
				}
			}

			const sharedNotes = new Map<string, SharedNoteRecord>();
			if (Array.isArray(entry.sharedNotes)) {
				for (const note of entry.sharedNotes) {
					if (!note || typeof note !== 'object') {
						continue;
					}
					if (typeof note.id !== 'string' || note.id.trim() === '') {
						continue;
					}
					const updatedAt = parsePersistedDate(note.updatedAt, lastSeen);
					sharedNotes.set(note.id, {
						id: note.id,
						ciphertext: note.ciphertext ?? '',
						nonce: note.nonce ?? '',
						digest: note.digest ?? '',
						version: typeof note.version === 'number' ? note.version : 1,
						updatedAt
					});
				}
			}

			let pendingCommands = Array.isArray(entry.pendingCommands)
				? entry.pendingCommands.map((command) => ({ ...command }))
				: [];
			if (pendingCommands.length > MAX_PENDING_COMMANDS) {
				console.warn(
					`Pending command snapshot for agent ${id} exceeded capacity (${pendingCommands.length}); trimming to latest ${MAX_PENDING_COMMANDS}.`
				);
				pendingCommands = pendingCommands.slice(-MAX_PENDING_COMMANDS);
			}

			const recentResults = Array.isArray(entry.recentResults)
				? entry.recentResults.map((result) => ({ ...result }))
				: [];

			const normalizedMetadata: AgentMetadata = {
				...(metadata as AgentMetadata),
				tags: Array.isArray((metadata as AgentMetadata).tags)
					? this.normalizeTags((metadata as AgentMetadata).tags!)
					: (metadata as AgentMetadata).tags
			};

			const fingerprint = entry.fingerprint
				? entry.fingerprint
				: computeFingerprint(normalizedMetadata);

			const record: AgentRecord = {
				id,
				key,
				metadata: normalizedMetadata,
				status: normalizedStatus,
				connectedAt,
				lastSeen,
				metrics: entry.metrics ? { ...entry.metrics } : undefined,
				config: normalizeConfig(entry.config),
				pendingCommands,
				recentResults,
				sharedNotes,
				fingerprint
			};

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
					await this.persistToDisk();
				} catch (error) {
					console.error('Failed to persist agent registry', error);
				}
			}
		} finally {
			this.persistPromise = null;
		}
	}

	private async persistToDisk(): Promise<void> {
		const agents = Array.from(this.agents.values()).map<PersistedAgentRecord>((record) => ({
			id: record.id,
			key: record.key,
			metadata: cloneMetadata(record.metadata),
			status: record.status,
			connectedAt: record.connectedAt.toISOString(),
			lastSeen: record.lastSeen.toISOString(),
			metrics: cloneMetrics(record.metrics),
			config: { ...record.config },
			pendingCommands: record.pendingCommands.map((command) => ({ ...command })),
			recentResults: record.recentResults.map((result) => ({ ...result })),
			sharedNotes: Array.from(record.sharedNotes.values()).map((note) => ({
				id: note.id,
				ciphertext: note.ciphertext,
				nonce: note.nonce,
				digest: note.digest,
				version: note.version,
				updatedAt: note.updatedAt.toISOString()
			})),
			fingerprint: record.fingerprint
		}));

		const payload: PersistedRegistryFile = {
			version: PERSIST_FILE_VERSION,
			agents
		};

		const data = JSON.stringify(payload);
		await ensureParentDirectory(this.storagePath);
		await writeFileAtomic(this.storagePath, data);
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
				existingRecord.key = randomBytes(32).toString('hex');
				existingRecord.config = normalizeConfig(existingRecord.config);
				existingRecord.fingerprint = computeFingerprint(nextMetadata);

				if (previousFingerprint !== existingRecord.fingerprint) {
					this.fingerprints.delete(previousFingerprint);
				}
				this.fingerprints.set(existingRecord.fingerprint, existingRecord.id);
				this.agents.set(existingRecord.id, existingRecord);
				this.schedulePersist();

				return {
					agentId: existingRecord.id,
					agentKey: existingRecord.key,
					config: { ...existingRecord.config },
					commands: [],
					serverTime: now.toISOString()
				};
			}

			this.fingerprints.delete(fingerprint);
		}

		const id = randomUUID();
		const key = randomBytes(32).toString('hex');
		const record: AgentRecord = {
			id,
			key,
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

		return {
			agentId: id,
			agentKey: key,
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

		if (!key || key !== record.key) {
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

		if (!key || key !== record.key) {
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
		}

		const delivery: CommandDeliveryMode = delivered ? 'session' : 'queued';
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
		if (!this.deliverViaSession(record, command)) {
			record.pendingCommands.push(command);
			this.clampPendingCommands(record, 'front');
		}

		this.schedulePersist();

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

		if (!this.deliverViaSession(record, command)) {
			record.pendingCommands.unshift(command);
			this.clampPendingCommands(record, 'back');
		}

		this.schedulePersist();

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
		if (!key || key !== record.key) {
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

		if (!key || key !== record.key) {
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

		if (changed) {
			this.schedulePersist();
		}

		return Array.from(record.sharedNotes.values()).map(
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
	}
}

export const registry = new AgentRegistry();
export { RegistryError };
