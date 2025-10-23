import { beforeEach, describe, expect, it } from 'vitest';
import { WebcamControlManager } from './webcam';

describe('WebcamControlManager', () => {
	let manager: WebcamControlManager;

	beforeEach(() => {
		manager = new WebcamControlManager();
	});

	it('tracks pending inventory requests until fulfilled', () => {
		const stateEmpty = manager.getInventoryState('agent-1');
		expect(stateEmpty.pending).toBe(false);
		expect(stateEmpty.inventory).toBeNull();

		manager.markInventoryRequest('agent-1', 'req-1');
		let state = manager.getInventoryState('agent-1');
		expect(state.pending).toBe(true);

		manager.updateInventory('agent-1', {
			devices: [],
			capturedAt: new Date().toISOString(),
			requestId: 'req-1'
		});

		state = manager.getInventoryState('agent-1');
		expect(state.pending).toBe(false);
		expect(state.inventory).not.toBeNull();
	});

	it('creates, updates, and removes sessions', () => {
		const session = manager.createSession('agent-1', {
			deviceId: 'camera-1',
			settings: { width: 1280, height: 720 }
		});

		expect(session.sessionId).toBeTruthy();
		expect(session.status).toBe('pending');

		const updated = manager.updateSession('agent-1', session.sessionId, {
			status: 'active',
			negotiation: {
				offer: { transport: 'webrtc', offer: 'abc' }
			}
		});

		expect(updated.status).toBe('active');
		expect(updated.negotiation?.offer?.offer).toBe('abc');

		const fetched = manager.getSession('agent-1', session.sessionId);
		expect(fetched?.status).toBe('active');

		manager.deleteSession('agent-1', session.sessionId);
		expect(manager.getSession('agent-1', session.sessionId)).toBeNull();
	});
});
