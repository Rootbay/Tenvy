import type {
        StartupCommandPayload,
        StartupCommandRequest,
        StartupCommandResponse,
        StartupEntry,
        StartupInventoryResponse,
        StartupListCommandRequest,
        StartupToggleCommandRequest,
        StartupCreateCommandRequest,
        StartupRemoveCommandRequest
} from '$lib/types/startup-manager';
import { registry, RegistryError } from './store';

const DEFAULT_TIMEOUT_MS = 20_000;
const MAX_TIMEOUT_MS = 120_000;
const POLL_INTERVAL_MS = 250;

export class StartupManagerAgentError extends Error {
        status: number;
        code?: string;
        details?: unknown;

        constructor(message: string, status = 500, options: { code?: string; details?: unknown } = {}) {
                super(message);
                this.name = 'StartupManagerAgentError';
                this.status = status;
                this.code = options.code;
                this.details = options.details;
        }
}

interface DispatchOptions {
        operatorId?: string;
        timeoutMs?: number;
}

type StartupCommandResult<T extends StartupCommandRequest> = T extends StartupListCommandRequest
        ? StartupInventoryResponse
        : T extends StartupToggleCommandRequest | StartupCreateCommandRequest
                ? StartupEntry
                : T extends StartupRemoveCommandRequest
                        ? { entryId: string }
                        : never;

function normalizeTimeout(requested?: number): number {
        if (typeof requested !== 'number' || Number.isNaN(requested) || requested <= 0) {
                return DEFAULT_TIMEOUT_MS;
        }
        const clamped = Math.min(Math.max(Math.floor(requested), 1_000), MAX_TIMEOUT_MS);
        return clamped;
}

function ensureCommandPayload(request: StartupCommandRequest): StartupCommandPayload {
        return { request } satisfies StartupCommandPayload;
}

type CommandResultSnapshot = {
        commandId: string;
        success: boolean;
        output?: string;
        error?: string;
        completedAt: string;
};

async function waitForCommandResult(agentId: string, commandId: string, timeoutMs: number): Promise<CommandResultSnapshot> {
        const start = Date.now();
        let delay = POLL_INTERVAL_MS;

        while (Date.now() - start <= timeoutMs) {
                let snapshot: ReturnType<typeof registry.getAgent>;
                try {
                        snapshot = registry.getAgent(agentId);
                } catch (err) {
                        if (err instanceof RegistryError) {
                                throw new StartupManagerAgentError(err.message, err.status);
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

        throw new StartupManagerAgentError('Timed out waiting for agent response', 504);
}

function extractResult<T extends StartupCommandRequest>(
        request: T,
        response: StartupCommandResponse
): StartupCommandResult<T> {
        if (response.operation !== request.operation) {
                throw new StartupManagerAgentError('Agent returned mismatched startup manager response', 502);
        }

        if (response.status === 'error') {
                throw new StartupManagerAgentError(response.error || 'Agent reported startup manager failure', 502, {
                        code: response.code,
                        details: response.details
                });
        }

        if (response.status !== 'ok') {
                throw new StartupManagerAgentError('Agent returned an unknown startup manager response', 502);
        }

        if (!('result' in response)) {
                throw new StartupManagerAgentError('Agent response missing startup manager payload', 502);
        }

        return response.result as StartupCommandResult<T>;
}

export async function dispatchStartupCommand<T extends StartupCommandRequest>(
        agentId: string,
        request: T,
        options: DispatchOptions = {}
): Promise<StartupCommandResult<T>> {
        let queuedCommandId: string;
        try {
                const payload = ensureCommandPayload(request);
                const queued = registry.queueCommand(
                        agentId,
                        { name: 'startup-manager', payload },
                        { operatorId: options.operatorId }
                );
                queuedCommandId = queued.command.id;
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw new StartupManagerAgentError(err.message, err.status);
                }
                throw new StartupManagerAgentError('Failed to queue startup manager command', 500);
        }

        const timeout = normalizeTimeout(options.timeoutMs);

        let result: CommandResultSnapshot;
        try {
                result = await waitForCommandResult(agentId, queuedCommandId, timeout);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw new StartupManagerAgentError(err.message, err.status);
                }
                throw err;
        }

        if (!result.success) {
                throw new StartupManagerAgentError(
                        result.error || 'Agent failed to execute startup manager command',
                        502
                );
        }

        if (!result.output) {
                throw new StartupManagerAgentError('Agent response missing startup manager payload', 502);
        }

        let decoded: StartupCommandResponse;
        try {
                decoded = JSON.parse(result.output) as StartupCommandResponse;
        } catch (err) {
                throw new StartupManagerAgentError(
                        `Agent response payload malformed: ${(err as Error).message || 'invalid JSON'}`,
                        502
                );
        }

        return extractResult(request, decoded);
}
