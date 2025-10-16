import { randomBytes, randomUUID } from 'crypto';
import { defaultAgentConfig, type AgentConfig } from '../../../../../shared/types/config';
import type { NoteEnvelope } from '../../../../../shared/types/notes';
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
	Command,
	CommandDeliveryMode,
	CommandInput,
	CommandQueueResponse,
	CommandResult
} from '../../../../../shared/types/messages';

const MAX_TAGS = 16;
const MAX_TAG_LENGTH = 32;
const TAG_PATTERN = /^[\p{L}\p{N}_\-\s]+$/u;

const MAX_RECENT_RESULTS = 25;

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
	session?: AgentSessionRecord;
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

export class AgentRegistry {
	private agents = new Map<string, AgentRecord>();

	private toSnapshot(record: AgentRecord): AgentSnapshot {
		return {
			id: record.id,
			metadata: record.metadata,
			status: record.status,
			connectedAt: record.connectedAt.toISOString(),
			lastSeen: record.lastSeen.toISOString(),
			metrics: record.metrics,
			pendingCommands: record.pendingCommands.length,
			recentResults: record.recentResults,
			liveSession: Boolean(record.session)
		} satisfies AgentSnapshot;
	}

	private detachSession(
		record: AgentRecord,
		sessionId: symbol,
		options: { close?: boolean; code?: number; reason?: string } = {}
	) {
		const session = record.session;
		if (!session || session.id !== sessionId) {
			return;
		}

		record.session = undefined;

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

	registerAgent(
		payload: AgentRegistrationRequest,
		options: { remoteAddress?: string } = {}
	): AgentRegistrationResponse {
		validateToken(payload.token);
		const now = new Date();
		const id = randomUUID();
		const key = randomBytes(32).toString('hex');

		const record: AgentRecord = {
			id,
			key,
			metadata: ensureMetadata(payload.metadata, options.remoteAddress),
			status: 'online',
			connectedAt: now,
			lastSeen: now,
			config: { ...defaultAgentConfig },
			pendingCommands: [],
			recentResults: [],
			sharedNotes: new Map()
		};

		this.agents.set(id, record);

		return {
			agentId: id,
			agentKey: key,
			config: record.config,
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
				reason: 'Session replaced'
			});
		}

		record.lastSeen = new Date();
		record.status = 'online';

		const acceptingSocket = socket as unknown as { accept?: () => void };
		if (typeof acceptingSocket.accept === 'function') {
			try {
				acceptingSocket.accept();
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
					break;
				}
			}
		}
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
			record.metrics = payload.metrics;
		}
		if (payload.results && payload.results.length > 0) {
			record.recentResults = [...payload.results, ...record.recentResults].slice(
				0,
				MAX_RECENT_RESULTS
			);
		}

		const commands = record.pendingCommands;
		record.pendingCommands = [];

		return {
			agentId: id,
			commands,
			config: record.config,
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
		}

		const delivery: CommandDeliveryMode = delivered ? 'session' : 'queued';
		return { command, delivery };
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
		}

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
		}

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
			}
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
