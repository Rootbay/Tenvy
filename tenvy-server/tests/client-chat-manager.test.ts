import { beforeEach, describe, expect, it } from 'vitest';
import { ClientChatManager, ClientChatError } from '$lib/server/rat/client-chat';

const agentId = 'agent-test';

describe('ClientChatManager', () => {
	let manager: ClientChatManager;

	beforeEach(() => {
		manager = new ClientChatManager();
	});

	it('ensures a session and applies aliases', () => {
		const state = manager.ensureSession(agentId, {
			aliases: { operator: 'Ops', client: 'Remote' }
		});
		expect(state.active).toBe(true);
		expect(state.unstoppable).toBe(true);
		expect(state.operatorAlias).toBe('Ops');
		expect(state.clientAlias).toBe('Remote');
		expect(state.messages).toHaveLength(0);
	});

	it('records operator messages and allows retraction', () => {
		const state = manager.ensureSession(agentId);
		const response = manager.sendOperatorMessage(agentId, {
			sessionId: state.sessionId,
			body: 'Ping from operator'
		});
		expect(response.accepted).toBe(true);
		expect(response.message.body).toBe('Ping from operator');
		const after = manager.getState(agentId);
		expect(after?.messages).toHaveLength(1);
		manager.retractMessage(agentId, response.message.id);
		const snapshot = manager.getState(agentId);
		expect(snapshot?.messages).toHaveLength(0);
	});

	it('activates sessions when client messages arrive', () => {
		const timestamp = new Date().toISOString();
		const response = manager.registerClientMessage(agentId, {
			sessionId: 'client-session',
			message: {
				id: 'm-1',
				body: 'Hello operator',
				timestamp
			}
		});
		expect(response.accepted).toBe(true);
		expect(response.session.active).toBe(true);
		expect(response.session.sessionId).toBe('client-session');
		expect(response.session.messages).toHaveLength(1);
	});

	it('stops a session and clears unstoppable flag', () => {
		const state = manager.ensureSession(agentId);
		const stopped = manager.stopSession(agentId, state.sessionId);
		expect(stopped?.active).toBe(false);
		expect(stopped?.unstoppable).toBe(false);
	});

	it('ignores requests to disable unstoppable while active', () => {
		const state = manager.ensureSession(agentId);
		const updated = manager.configureSession(agentId, {
			sessionId: state.sessionId,
			features: { unstoppable: false }
		});
		expect(updated.unstoppable).toBe(true);
		expect(updated.features.unstoppable).toBe(true);
	});

	it('rejects operator messages when session inactive', () => {
		expect(() =>
			manager.sendOperatorMessage(agentId, {
				sessionId: 'missing',
				body: 'message'
			})
		).toThrow(ClientChatError);
	});
});
