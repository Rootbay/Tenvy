import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ClipboardTriggerEvent } from '$lib/types/clipboard';

const queueCommandMock = vi.fn();

vi.mock('./store', () => ({
	registry: {
		queueCommand: queueCommandMock
	},
	RegistryError: class MockRegistryError extends Error {}
}));

const {
	buildCommandPayload,
	executeClipboardTriggerCommandAction,
	normalizeCommandActionConfiguration
} = await import('./clipboard-trigger-actions');

const baseEvent: ClipboardTriggerEvent = {
	eventId: 'evt-1',
	triggerId: 'trigger-1',
	triggerLabel: 'Sensitive clipboard',
	capturedAt: new Date().toISOString(),
	sequence: 42,
	content: {
		format: 'text',
		text: { value: 'secret token' }
	},
	action: { type: 'command', configuration: { command: 'shell', payload: { command: 'whoami' } } }
};

describe('normalizeCommandActionConfiguration', () => {
	it('returns null for non-object input', () => {
		expect(normalizeCommandActionConfiguration(undefined)).toBeNull();
		expect(normalizeCommandActionConfiguration(null)).toBeNull();
		expect(normalizeCommandActionConfiguration('command')).toBeNull();
	});

	it('returns null for unsupported command names', () => {
		expect(
			normalizeCommandActionConfiguration({ command: 'invalid', payload: { command: 'echo test' } })
		).toBeNull();
		expect(
			normalizeCommandActionConfiguration({ command: 'keylogger', payload: { command: 'noop' } })
		).toBeNull();
	});

	it('normalizes valid configuration with defaults', () => {
		const result = normalizeCommandActionConfiguration({
			command: 'shell',
			payload: { command: 'whoami' }
		});
		expect(result).toEqual({
			command: 'shell',
			payload: { command: 'whoami' },
			includeContent: false,
			includeMatches: true,
			includeMetadata: true,
			contextKey: 'context',
			operatorId: undefined
		});
	});

	it('allows new keylogger command variants', () => {
		const startConfig = normalizeCommandActionConfiguration({ command: 'keylogger.start' });
		const stopConfig = normalizeCommandActionConfiguration({ command: 'keylogger.stop' });

		expect(startConfig).toMatchObject({
			command: 'keylogger.start',
			payload: {}
		});
		expect(stopConfig).toMatchObject({
			command: 'keylogger.stop',
			payload: {}
		});
	});
});

describe('buildCommandPayload', () => {
	it('injects context when requested', () => {
		const config = normalizeCommandActionConfiguration({
			command: 'shell',
			payload: { command: 'echo "hello"' },
			includeContent: true,
			includeMatches: true,
			includeMetadata: true,
			contextKey: 'meta'
		});
		expect(config).not.toBeNull();
		const payload = buildCommandPayload(config!, {
			...baseEvent,
			matches: [{ field: 'text', value: 'secret' }]
		});
                expect(payload).toMatchObject({
                        command: 'echo "hello"',
                        meta: {
                                eventId: 'evt-1',
                                triggerId: 'trigger-1',
                                triggerLabel: 'Sensitive clipboard',
                                capturedAt: baseEvent.capturedAt,
                                sequence: 42,
                                matches: [{ field: 'text', value: 'secret' }],
                                content: {
                                        format: 'text',
                                        text: { value: 'secret token' }
				}
			}
		});
		const meta = (payload as Record<string, unknown>).meta as Record<string, unknown>;
		(meta.content as { text: { value: string } }).text.value = 'modified';
		expect(baseEvent.content?.text?.value).toBe('secret token');
	});
});

describe('executeClipboardTriggerCommandAction', () => {
	beforeEach(() => {
		queueCommandMock.mockReset();
	});

	it('queues command when configuration is valid', () => {
                const success = executeClipboardTriggerCommandAction('agent-1', baseEvent, 'test context');
                expect(success).toBe(true);
                expect(queueCommandMock).toHaveBeenCalledWith(
                        'agent-1',
                        {
                                name: 'shell',
                                payload: {
                                        command: 'whoami',
                                        context: expect.objectContaining({
                                                eventId: 'evt-1',
                                                triggerId: 'trigger-1',
                                                triggerLabel: 'Sensitive clipboard',
                                                capturedAt: baseEvent.capturedAt,
                                                sequence: 42
                                        })
                                }
                        },
                        { operatorId: undefined }
                );
        });

	it('returns false when configuration is invalid', () => {
		const event: ClipboardTriggerEvent = {
			...baseEvent,
			action: { type: 'command', configuration: { command: 'invalid-command' } }
		};
		const success = executeClipboardTriggerCommandAction('agent-1', event, 'invalid');
		expect(success).toBe(false);
		expect(queueCommandMock).not.toHaveBeenCalled();
	});

	it('queues keylogger.start command without payload', () => {
                const event: ClipboardTriggerEvent = {
                        ...baseEvent,
                        action: { type: 'command', configuration: { command: 'keylogger.start' } }
                };
                const success = executeClipboardTriggerCommandAction('agent-1', event, 'keylogger start');
                expect(success).toBe(true);
                expect(queueCommandMock).toHaveBeenCalledWith(
                        'agent-1',
                        {
                                name: 'keylogger.start',
                                payload: {
                                        context: expect.objectContaining({
                                                eventId: 'evt-1',
                                                triggerId: 'trigger-1',
                                                triggerLabel: 'Sensitive clipboard',
                                                capturedAt: baseEvent.capturedAt,
                                                sequence: 42
                                        })
                                }
                        },
                        { operatorId: undefined }
                );
        });
});
