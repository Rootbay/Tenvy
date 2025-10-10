import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ClipboardManager } from './clipboard';
import type { ClipboardSnapshot, ClipboardTriggerEvent } from '$lib/types/clipboard';

const AGENT_ID = 'agent-123';

function createSnapshot(
	sequence: number,
	overrides: Partial<ClipboardSnapshot> = {}
): ClipboardSnapshot {
	return {
		sequence,
		capturedAt: new Date().toISOString(),
		content: {
			format: 'text',
			text: { value: `sequence-${sequence}` }
		},
		...overrides
	};
}

describe('ClipboardManager', () => {
	let manager: ClipboardManager;

	beforeEach(() => {
		manager = new ClipboardManager();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('resolves pending requests with the freshest snapshot when stale data arrives', async () => {
		manager.ingestState(AGENT_ID, { snapshot: createSnapshot(5) });

		vi.useFakeTimers();
		const { requestId, wait } = manager.createRequest(AGENT_ID, 1000);

		manager.ingestState(AGENT_ID, {
			requestId,
			snapshot: createSnapshot(3)
		});

		await expect(wait).resolves.toMatchObject({ sequence: 5 });
	});

	it('resolves all pending requests when a new snapshot is ingested without a request id', async () => {
		vi.useFakeTimers();
		const pendingA = manager.createRequest(AGENT_ID, 1000).wait;
		const pendingB = manager.createRequest(AGENT_ID, 1000).wait;

		const freshSnapshot = createSnapshot(6);
		const resolved = manager.ingestState(AGENT_ID, { snapshot: freshSnapshot });

		expect(resolved.sequence).toBe(6);
		await expect(pendingA).resolves.toMatchObject({ sequence: 6 });
		await expect(pendingB).resolves.toMatchObject({ sequence: 6 });
	});

	it('deduplicates trigger events and preserves newest entries first', () => {
		const baseEvent: ClipboardTriggerEvent = {
			eventId: 'event-1',
			triggerId: 'trigger-a',
			triggerLabel: 'Trigger A',
			capturedAt: new Date().toISOString(),
			sequence: 1,
			content: {
				format: 'text',
				text: { value: 'alpha' }
			},
			action: { type: 'notify' }
		};

		manager.clearEvents(AGENT_ID);
		const initial = manager.appendEvents(AGENT_ID, { events: [baseEvent] });
		expect(initial).toHaveLength(1);

		const updated = manager.appendEvents(AGENT_ID, {
			events: [
				{
					...baseEvent,
					eventId: 'event-2',
					capturedAt: new Date().toISOString(),
					sequence: 2,
					content: {
						format: 'text',
						text: { value: 'beta' }
					}
				},
				{
					...baseEvent,
					capturedAt: new Date().toISOString(),
					sequence: 1
				}
			]
		});

		expect(updated).toHaveLength(2);
		expect(updated[0].eventId).toBe('event-2');
		expect(updated[1].eventId).toBe('event-1');
	});
});
