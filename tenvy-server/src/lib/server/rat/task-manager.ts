import type {
        ProcessActionResponse,
        ProcessDetail,
        ProcessListResponse,
        StartProcessResponse,
        TaskManagerCommandRequest,
        TaskManagerCommandResponse,
        TaskManagerCommandPayload,
        TaskManagerOperation
} from '$lib/types/task-manager';
import { registry, RegistryError } from './store';

const DEFAULT_TIMEOUT_MS = 15_000;
const START_TIMEOUT_MS = 30_000;
const MAX_TIMEOUT_MS = 120_000;
const POLL_INTERVAL_MS = 250;

export class TaskManagerAgentError extends Error {
        status: number;
        code?: string;
        details?: unknown;

        constructor(message: string, status = 500, options: { code?: string; details?: unknown } = {}) {
                super(message);
                this.name = 'TaskManagerAgentError';
                this.status = status;
                this.code = options.code;
                this.details = options.details;
        }
}

interface DispatchOptions {
        operatorId?: string;
        timeoutMs?: number;
}

type TaskManagerRequestResult<T extends TaskManagerCommandRequest> = T extends {
        operation: 'list';
}
        ? ProcessListResponse
        : T extends { operation: 'detail' }
                ? ProcessDetail
                : T extends { operation: 'start' }
                        ? StartProcessResponse
                        : ProcessActionResponse;

function normalizeTimeout(operation: TaskManagerOperation, requested?: number): number {
        const baseline = operation === 'start' ? START_TIMEOUT_MS : DEFAULT_TIMEOUT_MS;
        if (typeof requested !== 'number' || Number.isNaN(requested) || requested <= 0) {
                return baseline;
        }
        const clamped = Math.min(Math.max(Math.floor(requested), 1_000), MAX_TIMEOUT_MS);
        return Math.max(clamped, baseline);
}

function ensureCommandPayload(request: TaskManagerCommandRequest): TaskManagerCommandPayload {
        return { request } satisfies TaskManagerCommandPayload;
}

async function waitForCommandResult(
        agentId: string,
        commandId: string,
        timeoutMs: number
): Promise<CommandResultSnapshot> {
        const start = Date.now();
        let delay = POLL_INTERVAL_MS;

        while (Date.now() - start <= timeoutMs) {
                let snapshot: ReturnType<typeof registry.getAgent>;
                try {
                        snapshot = registry.getAgent(agentId);
                } catch (err) {
                        if (err instanceof RegistryError) {
                                throw new TaskManagerAgentError(err.message, err.status);
                        }
                        throw err;
                }
                const match = snapshot.recentResults.find((result) => result.commandId === commandId);
                if (match) {
                        return match;
                }
                const elapsed = Date.now() - start;
                const remaining = timeoutMs - elapsed;
                if (remaining <= 0) {
                        break;
                }
                await new Promise((resolve) => setTimeout(resolve, Math.min(delay, remaining)));
                delay = Math.min(delay * 2, 1_000);
        }

        throw new TaskManagerAgentError('Timed out waiting for agent response', 504);
}

type CommandResultSnapshot = {
        commandId: string;
        success: boolean;
        output?: string;
        error?: string;
        completedAt: string;
};

function extractResult<T extends TaskManagerCommandRequest>(
        request: T,
        response: TaskManagerCommandResponse
): TaskManagerRequestResult<T> {
        if (response.operation !== request.operation) {
                throw new TaskManagerAgentError('Agent returned mismatched task manager response', 502);
        }

        if (response.status === 'error') {
                throw new TaskManagerAgentError(response.error || 'Agent reported task manager failure', 502, {
                        code: response.code,
                        details: response.details
                });
        }

        if (response.status !== 'ok') {
                throw new TaskManagerAgentError('Agent returned an unknown task manager response', 502);
        }

        if (!('result' in response)) {
                throw new TaskManagerAgentError('Agent response missing task manager result payload', 502);
        }

        return response.result as TaskManagerRequestResult<T>;
}

export async function dispatchTaskManagerCommand<T extends TaskManagerCommandRequest>(
        agentId: string,
        request: T,
        options: DispatchOptions = {}
): Promise<TaskManagerRequestResult<T>> {
        let queuedCommandId: string;
        try {
                const payload = ensureCommandPayload(request);
                const queued = registry.queueCommand(
                        agentId,
                        { name: 'task-manager', payload },
                        { operatorId: options.operatorId }
                );
                queuedCommandId = queued.command.id;
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw new TaskManagerAgentError(err.message, err.status);
                }
                throw new TaskManagerAgentError('Failed to queue task manager command', 500);
        }

        const timeout = normalizeTimeout(request.operation, options.timeoutMs);

        let result: CommandResultSnapshot;
        try {
                result = await waitForCommandResult(agentId, queuedCommandId, timeout);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw new TaskManagerAgentError(err.message, err.status);
                }
                throw err;
        }

        if (!result.success) {
                throw new TaskManagerAgentError(result.error || 'Agent failed to execute task manager command', 502);
        }

        if (!result.output) {
                throw new TaskManagerAgentError('Agent response missing task manager payload', 502);
        }

        let decoded: TaskManagerCommandResponse;
        try {
                decoded = JSON.parse(result.output) as TaskManagerCommandResponse;
        } catch (err) {
                throw new TaskManagerAgentError(
                        `Agent response payload malformed: ${(err as Error).message || 'invalid JSON'}`,
                        502
                );
        }

        return extractResult(request, decoded);
}
