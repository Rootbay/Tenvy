import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import path from 'path';
import { AgentRegistry } from './store';
import type { AgentRegistrationResponse } from '../../../../../shared/types/auth';
import type { CommandQueueResponse } from '../../../../../shared/types/messages';

const baseMetadata = {
	hostname: 'test-host',
	username: 'tester',
	os: 'linux',
	architecture: 'x64'
};

class MockSocket {
	readyState = 1;
	accepted = false;
	closed = false;
	sent: string[] = [];
	private listeners = new Map<string, Set<() => void>>();

	accept() {
		this.accepted = true;
	}

	send(data: unknown) {
		if (this.closed) {
			throw new Error('socket closed');
		}
		this.sent.push(typeof data === 'string' ? data : String(data));
	}

	close() {
		if (this.closed) {
			return;
		}
		this.closed = true;
		this.readyState = 3;
		this.emit('close');
	}

	addEventListener(type: string, handler: () => void) {
		if (!this.listeners.has(type)) {
			this.listeners.set(type, new Set());
		}
		this.listeners.get(type)!.add(handler);
	}

	emit(type: string) {
		const handlers = this.listeners.get(type);
		if (!handlers) {
			return;
		}
		for (const handler of handlers) {
			handler();
		}
	}
}

describe('AgentRegistry live sessions', () => {
        let registry: AgentRegistry;
        let registration: AgentRegistrationResponse;
        let tempDir: string;

        beforeEach(() => {
                tempDir = mkdtempSync(path.join(tmpdir(), 'agent-registry-test-'));
                const storagePath = path.join(tempDir, 'registry.json');
                registry = new AgentRegistry({ storagePath });
                registration = registry.registerAgent({ metadata: baseMetadata });
        });

        afterEach(async () => {
                await registry.flush();
                rmSync(tempDir, { recursive: true, force: true });
        });

	function attach(socket: MockSocket) {
		registry.attachSession(
			registration.agentId,
			registration.agentKey,
			socket as unknown as WebSocket
		);
	}

	it('delivers new commands through an active session', () => {
		const socket = new MockSocket();
		attach(socket);

		const response = registry.queueCommand(registration.agentId, {
			name: 'ping',
			payload: { message: 'hello' }
		});

		expect(response.delivery).toBe('session');
		expect(socket.sent).toHaveLength(1);

		const envelope = JSON.parse(socket.sent[0]!);
		expect(envelope.type).toBe('command');
		expect(envelope.command?.name).toBe('ping');

		const snapshot = registry.getAgent(registration.agentId);
		expect(snapshot.liveSession).toBe(true);
		expect(registry.peekCommands(registration.agentId)).toHaveLength(0);
	});

	it('queues commands when no session is available', () => {
		const response = registry.queueCommand(registration.agentId, {
			name: 'ping',
			payload: {}
		});

		expect(response.delivery).toBe('queued');
		expect(registry.peekCommands(registration.agentId)).toHaveLength(1);
	});

	it('flushes queued commands when a session attaches', () => {
		const queued = registry.queueCommand(registration.agentId, {
			name: 'ping',
			payload: { message: 'queued' }
		}) satisfies CommandQueueResponse;
		expect(queued.delivery).toBe('queued');

		const socket = new MockSocket();
		attach(socket);

		expect(socket.sent).toHaveLength(1);
		const envelope = JSON.parse(socket.sent[0]!);
		expect(envelope.command?.id).toBe(queued.command.id);
		expect(registry.peekCommands(registration.agentId)).toHaveLength(0);
	});

	it('falls back to queuing when the session closes', () => {
		const socket = new MockSocket();
		attach(socket);
		socket.close();

		const response = registry.queueCommand(registration.agentId, {
			name: 'ping',
			payload: {}
		});

		expect(response.delivery).toBe('queued');
		const snapshot = registry.getAgent(registration.agentId);
		expect(snapshot.liveSession).toBe(false);
	});
});
