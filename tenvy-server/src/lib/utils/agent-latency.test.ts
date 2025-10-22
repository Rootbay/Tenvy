import { describe, expect, it } from 'vitest';
import type { AgentSnapshot } from '../../../../shared/types/agent';
import { formatAgentLatency, normalizeAgentLatency, selectAgentLatencyMs } from './agent-latency';

function buildAgentSnapshot(metrics?: AgentSnapshot['metrics']): AgentSnapshot {
	return {
		id: 'agent-123',
		metadata: {
			hostname: 'host',
			username: 'user',
			os: 'linux',
			architecture: 'x64'
		},
		status: 'online',
		connectedAt: new Date(0).toISOString(),
		lastSeen: new Date(0).toISOString(),
		metrics,
		pendingCommands: 0,
		recentResults: []
	} satisfies AgentSnapshot;
}

describe('selectAgentLatencyMs', () => {
	it('prefers latencyMs when available', () => {
		const metrics = { latencyMs: 42, pingMs: 99 } satisfies NonNullable<AgentSnapshot['metrics']>;
		expect(selectAgentLatencyMs(metrics)).toBe(42);
	});

	it('falls back to pingMs when latencyMs is missing', () => {
		const metrics = { pingMs: 87 } satisfies NonNullable<AgentSnapshot['metrics']>;
		expect(selectAgentLatencyMs(metrics)).toBe(87);
	});

	it('returns null when latency metrics are invalid', () => {
		const metrics = { latencyMs: Number.NaN, pingMs: -5 } satisfies NonNullable<
			AgentSnapshot['metrics']
		>;
		expect(selectAgentLatencyMs(metrics)).toBeNull();
	});
});

describe('normalizeAgentLatency', () => {
	it('copies snapshots without metrics', () => {
		const snapshot = buildAgentSnapshot();
		expect(normalizeAgentLatency(snapshot)).toEqual({ ...snapshot });
	});

	it('injects latencyMs when only pingMs exists', () => {
		const snapshot = buildAgentSnapshot({ pingMs: 64 });
		expect(normalizeAgentLatency(snapshot).metrics?.latencyMs).toBe(64);
	});
});

describe('formatAgentLatency', () => {
	it('rounds finite latency values', () => {
		const snapshot = buildAgentSnapshot({ latencyMs: 12.6 });
		expect(formatAgentLatency(snapshot)).toBe('13 ms');
	});

	it('returns N/A when metrics are unavailable', () => {
		const snapshot = buildAgentSnapshot();
		expect(formatAgentLatency(snapshot)).toBe('N/A');
	});
});
