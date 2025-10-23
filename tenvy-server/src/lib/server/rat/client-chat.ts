import { randomUUID } from 'crypto';
import type {
	ClientChatAliasConfiguration,
	ClientChatFeatureFlags,
	ClientChatMessage,
	ClientChatMessageEnvelope,
	ClientChatMessageResponse,
	ClientChatSessionState
} from '$lib/types/client-chat';

const MAX_HISTORY = 200;
const DEFAULT_OPERATOR_ALIAS = 'Operator';
const DEFAULT_CLIENT_ALIAS = 'Client';

function cloneMessage(message: ChatMessageRecord): ClientChatMessage {
	return {
		id: message.id,
		sessionId: message.sessionId,
		sender: message.sender,
		alias: message.alias,
		body: message.body,
		timestamp: message.timestamp.toISOString()
	} satisfies ClientChatMessage;
}

function cloneFeatures(record: ChatSessionRecord): ClientChatFeatureFlags {
	const features: ClientChatFeatureFlags = {
		unstoppable: record.unstoppable
	};
	if (record.features.allowNotifications !== undefined) {
		features.allowNotifications = record.features.allowNotifications;
	}
	if (record.features.allowFileTransfers !== undefined) {
		features.allowFileTransfers = record.features.allowFileTransfers;
	}
	return features;
}

class ChatMessageRecord {
	id!: string;
	sessionId!: string;
	sender!: 'operator' | 'client';
	alias?: string;
	body!: string;
	timestamp!: Date;
}

interface ChatSessionRecord {
	id: string;
	agentId: string;
	active: boolean;
	unstoppable: boolean;
	startedAt: Date;
	stoppedAt?: Date;
	operatorAlias: string;
	clientAlias: string;
	features: {
		unstoppable: boolean;
		allowNotifications?: boolean;
		allowFileTransfers?: boolean;
	};
	messages: ChatMessageRecord[];
}

function sanitizeAlias(alias: string | undefined, fallback: string): string {
	const trimmed = alias?.trim();
	return trimmed && trimmed.length > 0 ? trimmed : fallback;
}

export class ClientChatError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'ClientChatError';
		this.status = status;
	}
}

function ensureTimestamp(value: string | undefined): Date {
	if (!value) {
		return new Date();
	}
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) {
		return new Date();
	}
	return parsed;
}

function createMessageRecord(
	sessionId: string,
	sender: 'operator' | 'client',
	body: string,
	options: {
		id?: string;
		alias?: string;
		timestamp?: string;
	}
): ChatMessageRecord {
	const trimmed = body.trim();
	if (!trimmed) {
		throw new ClientChatError('Message body is required', 400);
	}
	const record = new ChatMessageRecord();
	record.id = (options.id?.trim() || randomUUID()).toString();
	record.sessionId = sessionId;
	record.sender = sender;
	record.alias = options.alias?.trim();
	record.body = trimmed;
	record.timestamp = ensureTimestamp(options.timestamp);
	return record;
}

function cloneState(record: ChatSessionRecord): ClientChatSessionState {
	return {
		sessionId: record.id,
		active: record.active,
		unstoppable: record.unstoppable,
		operatorAlias: record.operatorAlias,
		clientAlias: record.clientAlias,
		startedAt: record.startedAt.toISOString(),
		stoppedAt: record.stoppedAt?.toISOString(),
		features: cloneFeatures(record),
		messages: record.messages.map((message) => cloneMessage(message))
	} satisfies ClientChatSessionState;
}

function defaultFeatures(): ChatSessionRecord['features'] {
	return { unstoppable: false };
}

export class ClientChatManager {
	private sessions = new Map<string, ChatSessionRecord>();

	getState(agentId: string): ClientChatSessionState | null {
		const record = this.sessions.get(agentId);
		if (!record) {
			return null;
		}
		return cloneState(record);
	}

	ensureSession(
		agentId: string,
		options: {
			sessionId?: string;
			aliases?: ClientChatAliasConfiguration;
			features?: Partial<ClientChatFeatureFlags>;
		} = {}
	): ClientChatSessionState {
		const record = this.getOrCreateRecord(agentId);
		const requestedId = options.sessionId?.trim();
		if (requestedId && requestedId !== record.id) {
			record.id = requestedId;
			record.messages = [];
		}
		if (!record.active) {
			record.startedAt = new Date();
		}
		record.active = true;
		record.unstoppable = true;
		record.features.unstoppable = true;
		record.stoppedAt = undefined;
		this.applyAliases(record, options.aliases);
		this.applyFeatures(record, options.features);
		return cloneState(record);
	}

	stopSession(agentId: string, sessionId?: string): ClientChatSessionState | null {
		const record = this.sessions.get(agentId);
		if (!record) {
			return null;
		}
		if (sessionId?.trim() && sessionId.trim() !== record.id) {
			throw new ClientChatError('Chat session mismatch', 409);
		}
		if (!record.active) {
			return cloneState(record);
		}
		record.active = false;
		record.unstoppable = false;
		record.features.unstoppable = false;
		record.stoppedAt = new Date();
		return cloneState(record);
	}

	configureSession(
		agentId: string,
		options: {
			sessionId?: string;
			aliases?: ClientChatAliasConfiguration;
			features?: Partial<ClientChatFeatureFlags>;
		}
	): ClientChatSessionState {
		const record = this.getOrCreateRecord(agentId);
                const requestedId = options.sessionId?.trim();
                if (requestedId && requestedId !== record.id) {
                        record.id = requestedId;
                        record.messages = [];
                }
                this.applyAliases(record, options.aliases);
                this.applyFeatures(record, options.features);
                return cloneState(record);
        }

	sendOperatorMessage(
		agentId: string,
		input: {
			sessionId: string;
			id?: string;
			body: string;
			timestamp?: string;
			alias?: string;
		}
	): ClientChatMessageResponse {
		const record = this.sessions.get(agentId);
		if (!record || !record.active) {
			throw new ClientChatError('Chat session is not active', 409);
		}
		if (input.sessionId.trim() !== record.id) {
			throw new ClientChatError('Chat session mismatch', 409);
		}
		const message = createMessageRecord(record.id, 'operator', input.body, {
			id: input.id,
			alias: input.alias ?? record.operatorAlias,
			timestamp: input.timestamp
		});
		this.appendMessage(record, message);
		return {
			accepted: true,
			session: cloneState(record),
			message: cloneMessage(message)
		} satisfies ClientChatMessageResponse;
	}

	registerClientMessage(
		agentId: string,
		envelope: ClientChatMessageEnvelope
	): ClientChatMessageResponse {
		const record = this.getOrCreateRecord(agentId);
		const incomingSessionId = envelope.sessionId?.trim();
		if (incomingSessionId && incomingSessionId !== record.id) {
			record.id = incomingSessionId;
			record.messages = [];
		}
		if (!record.active) {
			record.active = true;
			record.unstoppable = true;
			record.features.unstoppable = true;
			record.startedAt = new Date();
			record.stoppedAt = undefined;
		}
		const payload = envelope.message;
		const message = createMessageRecord(record.id, 'client', payload.body, {
			id: payload.id,
			alias: payload.alias ?? record.clientAlias,
			timestamp: payload.timestamp
		});
		this.appendMessage(record, message);
		return {
			accepted: true,
			session: cloneState(record),
			message: cloneMessage(message)
		} satisfies ClientChatMessageResponse;
	}

	retractMessage(agentId: string, messageId: string): void {
		const record = this.sessions.get(agentId);
		if (!record) {
			return;
		}
		const index = record.messages.findIndex((message) => message.id === messageId);
		if (index >= 0) {
			record.messages.splice(index, 1);
		}
	}

	private appendMessage(record: ChatSessionRecord, message: ChatMessageRecord) {
		record.messages.push(message);
		if (record.messages.length > MAX_HISTORY) {
			record.messages = record.messages.slice(record.messages.length - MAX_HISTORY);
		}
	}

	private getOrCreateRecord(agentId: string): ChatSessionRecord {
		let record = this.sessions.get(agentId);
		if (record) {
			return record;
		}
		record = {
			id: randomUUID(),
			agentId,
			active: false,
			unstoppable: false,
			startedAt: new Date(),
			operatorAlias: DEFAULT_OPERATOR_ALIAS,
			clientAlias: DEFAULT_CLIENT_ALIAS,
			features: defaultFeatures(),
			messages: []
		} satisfies ChatSessionRecord;
		this.sessions.set(agentId, record);
		return record;
	}

	private applyAliases(record: ChatSessionRecord, aliases?: ClientChatAliasConfiguration) {
		if (!aliases) {
			return;
		}
		if (aliases.operator !== undefined) {
			record.operatorAlias = sanitizeAlias(aliases.operator, DEFAULT_OPERATOR_ALIAS);
		}
		if (aliases.client !== undefined) {
			record.clientAlias = sanitizeAlias(aliases.client, DEFAULT_CLIENT_ALIAS);
		}
	}

	private applyFeatures(record: ChatSessionRecord, features?: Partial<ClientChatFeatureFlags>) {
		if (!features) {
			return;
		}
		if (features.unstoppable !== undefined) {
			record.unstoppable = features.unstoppable;
			record.features.unstoppable = features.unstoppable;
		}
		if (features.allowNotifications !== undefined) {
			record.features.allowNotifications = features.allowNotifications;
		}
		if (features.allowFileTransfers !== undefined) {
			record.features.allowFileTransfers = features.allowFileTransfers;
		}
	}
}

export const clientChatManager = new ClientChatManager();
