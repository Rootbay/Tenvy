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
	Command,
	CommandInput,
	CommandQueueResponse,
	CommandResult
} from '../../../../../shared/types/messages';

const MAX_TAGS = 16;
const MAX_TAG_LENGTH = 32;
const TAG_PATTERN = /^[\p{L}\p{N}_\-\s]+$/u;

const MAX_RECENT_RESULTS = 25;

class RegistryError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'RegistryError';
		this.status = status;
	}
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
			recentResults: record.recentResults
		} satisfies AgentSnapshot;
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

		record.pendingCommands.push(command);

		return { command };
	}

	disconnectAgent(id: string): AgentSnapshot {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		record.status = 'offline';
		record.lastSeen = new Date();
		record.pendingCommands = [];

		const payload: AgentControlCommandPayload = { action: 'disconnect' };
		const command: Command = {
			id: randomUUID(),
			name: 'agent-control',
			payload,
			createdAt: new Date().toISOString()
		};

		record.pendingCommands.push(command);

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

		record.pendingCommands.unshift(command);

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
