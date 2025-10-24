import { beforeEach, describe, expect, it, vi } from 'vitest';

const mockEnv = { env: {} };

vi.mock('$env/dynamic/private', () => mockEnv, { virtual: true });

const queueCommand = vi.fn();

class MockRegistryError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.status = status;
	}
}

vi.mock('../src/lib/server/authorization.js', () => ({
	requireOperator: (user: { id: string }) => user ?? { id: 'operator-test' }
}));

vi.mock('../src/lib/server/rat/store.js', () => ({
	registry: {
		queueCommand
	},
	RegistryError: MockRegistryError
}));

const refreshModule = await import(
	'../src/routes/api/agents/[clientId]/webcam/devices/refresh/+server.js'
);
const devicesModule = await import('../src/routes/api/agents/[clientId]/webcam/devices/+server.js');
const sessionsModule = await import(
	'../src/routes/api/agents/[clientId]/webcam/sessions/+server.js'
);
const sessionModule = await import(
	'../src/routes/api/agents/[clientId]/webcam/sessions/[sessionId]/+server.js'
);
const { webcamControlManager } = await import('../src/lib/server/rat/webcam.js');

const refreshPOST = refreshModule.POST;
const devicesPOST = devicesModule.POST;
const devicesGET = devicesModule.GET;
const sessionsPOST = sessionsModule.POST;
const sessionDELETE = sessionModule.DELETE;

function createEvent<T extends (...args: any) => any>(
	handler: T,
	options: Partial<Parameters<T>[0]>
): Parameters<T>[0] {
	return {
		params: {},
		request: new Request('https://controller.test', { method: 'GET' }),
		locals: { user: { id: 'operator-test' } },
		...options
	} as Parameters<T>[0];
}

describe('webcam API routes', () => {
	beforeEach(() => {
		queueCommand.mockReset();
	});

	it('queues an inventory refresh command', async () => {
		const response = await refreshPOST(
			createEvent(refreshPOST, {
				params: { clientId: 'agent-refresh' }
			})
		);

		expect(queueCommand).toHaveBeenCalledTimes(1);
		const payload = queueCommand.mock.calls[0]?.[1]?.payload as {
			action: string;
			requestId?: string;
		};
		expect(payload?.action).toBe('enumerate');
		expect(payload?.requestId).toBeTruthy();

		const body = (await response.json()) as { requestId: string };
		expect(body.requestId).toBe(payload.requestId);
	});

	it('persists inventory updates and supports session creation lifecycle', async () => {
		const now = new Date().toISOString();
		await devicesPOST(
			createEvent(devicesPOST, {
				params: { clientId: 'agent-inventory' },
				request: new Request('https://controller.test', {
					method: 'POST',
					body: JSON.stringify({
						devices: [{ id: 'cam-1', label: 'Primary Camera' }],
						capturedAt: now
					}),
					headers: { 'Content-Type': 'application/json' }
				})
			})
		);

		const response = await devicesGET(
			createEvent(devicesGET, {
				params: { clientId: 'agent-inventory' }
			})
		);

		const state = (await response.json()) as {
			inventory: { devices: { id: string }[] } | null;
			pending: boolean;
		};
		expect(state.pending).toBe(false);
		expect(state.inventory?.devices[0]?.id).toBe('cam-1');

		queueCommand.mockReset();
		const sessionResponse = await sessionsPOST(
			createEvent(sessionsPOST, {
				params: { clientId: 'agent-inventory' },
				request: new Request('https://controller.test', {
					method: 'POST',
					body: JSON.stringify({ deviceId: 'cam-1' }),
					headers: { 'Content-Type': 'application/json' }
				})
			})
		);

		expect(queueCommand).toHaveBeenCalledTimes(1);
		const sessionPayload = queueCommand.mock.calls[0]?.[1]?.payload as { action: string };
		expect(sessionPayload?.action).toBe('start');

		const sessionBody = (await sessionResponse.json()) as { sessionId: string };
		expect(sessionBody.sessionId).toBeTruthy();

		queueCommand.mockReset();
		await sessionDELETE(
			createEvent(sessionDELETE, {
				params: { clientId: 'agent-inventory', sessionId: sessionBody.sessionId }
			})
		);

		expect(queueCommand).toHaveBeenCalledTimes(1);
		const stopPayload = queueCommand.mock.calls[0]?.[1]?.payload as { action: string };
		expect(stopPayload?.action).toBe('stop');
		expect(webcamControlManager.getSession('agent-inventory', sessionBody.sessionId)).toBeNull();
	});
});
