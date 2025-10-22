import type { AgentMetrics, AgentSnapshot } from '../../../../shared/types/agent';

function sanitizeLatency(value: unknown): number | null {
        if (typeof value !== 'number') {
                return null;
        }
        if (!Number.isFinite(value)) {
                return null;
        }
        if (value < 0) {
                return null;
        }
        return value;
}

export function selectAgentLatencyMs(metrics: AgentMetrics | undefined): number | null {
        if (!metrics) {
                return null;
        }

        const latency = sanitizeLatency(metrics.latencyMs);
        if (latency !== null) {
                return latency;
        }

        return sanitizeLatency(metrics.pingMs);
}

export function normalizeAgentLatency(agent: AgentSnapshot): AgentSnapshot {
        const metrics = agent.metrics ? { ...agent.metrics } : undefined;
        if (!metrics) {
                return { ...agent };
        }

        const latency = selectAgentLatencyMs(metrics);

        if (latency !== null) {
                metrics.latencyMs = latency;
        } else if ('latencyMs' in metrics) {
                metrics.latencyMs = undefined;
        }

        return {
                ...agent,
                metrics
        };
}

export function formatAgentLatency(agent: AgentSnapshot): string {
        const latency = selectAgentLatencyMs(agent.metrics);
        if (latency === null) {
                return 'N/A';
        }

        return `${Math.round(latency)} ms`;
}
