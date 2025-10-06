import { randomBytes, randomUUID } from 'crypto';
import { defaultAgentConfig, type AgentConfig } from '../../../../../shared/types/config';
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
	AgentSyncRequest,
	AgentSyncResponse,
	Command,
	CommandInput,
	CommandQueueResponse,
	CommandResult
} from '../../../../../shared/types/messages';

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
}

function ensureMetadata(metadata: AgentMetadata, fallbackAddress?: string): AgentMetadata {
	if (metadata.ipAddress || !fallbackAddress) {
		return metadata;
	}
	return { ...metadata, ipAddress: fallbackAddress };
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
			recentResults: []
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

	syncAgent(id: string, key: string | undefined, payload: AgentSyncRequest): AgentSyncResponse {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}

		if (!key || key !== record.key) {
			throw new RegistryError('Invalid agent key', 401);
		}

		record.lastSeen = new Date();
		record.status = payload.status;
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

	peekCommands(id: string): Command[] {
		const record = this.agents.get(id);
		if (!record) {
			throw new RegistryError('Agent not found', 404);
		}
		return [...record.pendingCommands];
	}
}

export const registry = new AgentRegistry();
export { RegistryError };
