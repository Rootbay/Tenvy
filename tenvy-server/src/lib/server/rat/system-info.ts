import type { SystemInfoCommandPayload, SystemInfoReport, SystemInfoSnapshot } from '$lib/types/system-info';
import { registry, RegistryError } from './store';

const DEFAULT_TIMEOUT_MS = 15_000;
const MAX_TIMEOUT_MS = 120_000;
const POLL_INTERVAL_MS = 250;

export class SystemInfoAgentError extends Error {
        status: number;

        constructor(message: string, status = 500) {
                super(message);
                this.name = 'SystemInfoAgentError';
                this.status = status;
        }
}

type CommandResultSnapshot = {
        commandId: string;
        success: boolean;
        output?: string;
        error?: string;
        completedAt: string;
};

function normalizeTimeout(requested?: number): number {
        if (typeof requested !== 'number' || Number.isNaN(requested) || requested <= 0) {
                return DEFAULT_TIMEOUT_MS;
        }
        const clamped = Math.min(Math.max(Math.floor(requested), POLL_INTERVAL_MS), MAX_TIMEOUT_MS);
        return Math.max(clamped, DEFAULT_TIMEOUT_MS);
}

async function waitForCommandResult(
        agentId: string,
        commandId: string,
        timeoutMs: number
): Promise<CommandResultSnapshot> {
        const started = Date.now();
        let delay = POLL_INTERVAL_MS;

        while (Date.now() - started <= timeoutMs) {
                let snapshot: ReturnType<typeof registry.getAgent>;
                try {
                        snapshot = registry.getAgent(agentId);
                } catch (err) {
                        if (err instanceof RegistryError) {
                                throw new SystemInfoAgentError(err.message, err.status);
                        }
                        throw err;
                }

                const match = snapshot.recentResults.find((result) => result.commandId === commandId);
                if (match) {
                        return match satisfies CommandResultSnapshot;
                }

                const elapsed = Date.now() - started;
                const remaining = timeoutMs - elapsed;
                if (remaining <= 0) {
                        break;
                }

                await new Promise((resolve) => setTimeout(resolve, Math.min(delay, remaining)));
                delay = Math.min(delay * 2, 1_000);
        }

        throw new SystemInfoAgentError('Timed out waiting for agent response', 504);
}

function createPayload(refresh?: boolean): SystemInfoCommandPayload {
        if (refresh) {
                return { refresh: true } satisfies SystemInfoCommandPayload;
        }
        return {} satisfies SystemInfoCommandPayload;
}

function decodeReport(raw: string): SystemInfoReport {
        try {
                const decoded = JSON.parse(raw) as SystemInfoReport;
                if (!decoded || typeof decoded !== 'object') {
                        throw new Error('missing system info payload');
                }
                if (typeof decoded.collectedAt !== 'string' || !decoded.collectedAt) {
                        throw new Error('missing collectedAt timestamp');
                }
                if (!decoded.hardware || typeof decoded.hardware.architecture !== 'string') {
                        throw new Error('missing hardware architecture');
                }
                if (!decoded.runtime || typeof decoded.runtime.goVersion !== 'string') {
                        throw new Error('missing runtime metadata');
                }
                return decoded;
        } catch (error) {
                const message = error instanceof Error ? error.message : 'invalid payload';
                throw new SystemInfoAgentError(`Agent response payload malformed: ${message}`, 502);
        }
}

export async function requestSystemInfoSnapshot(
        agentId: string,
        options: { refresh?: boolean; timeoutMs?: number; operatorId?: string } = {}
): Promise<SystemInfoSnapshot> {
        let commandId: string;
        try {
                const payload = createPayload(options.refresh);
                const queued = registry.queueCommand(
                        agentId,
                        { name: 'system-info', payload },
                        { operatorId: options.operatorId }
                );
                commandId = queued.command.id;
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw new SystemInfoAgentError(err.message, err.status);
                }
                throw new SystemInfoAgentError('Failed to queue system info command');
        }

        const timeout = normalizeTimeout(options.timeoutMs);

        const result = await waitForCommandResult(agentId, commandId, timeout);
        if (!result.success) {
                throw new SystemInfoAgentError(
                        result.error || 'Agent failed to execute system info command',
                        502
                );
        }
        if (!result.output) {
                throw new SystemInfoAgentError('Agent response missing system info payload', 502);
        }

        const report = decodeReport(result.output);

        return {
                agentId,
                requestId: commandId,
                receivedAt: result.completedAt,
                report,
        } satisfies SystemInfoSnapshot;
}
