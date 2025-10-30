import type {
	RegistryCommandRequest,
	RegistryCommandResponse,
	RegistryListResult,
	RegistryMutationResult
} from '$lib/types/registry';
import { registryCommandPayloadSchema, registryCommandResponseSchema } from '$lib/types/registry';
import { registry, RegistryError } from './store';

const DEFAULT_TIMEOUT_MS = 15_000;
const MUTATION_TIMEOUT_MS = 30_000;
const MAX_TIMEOUT_MS = 120_000;
const POLL_INTERVAL_MS = 250;

export class RegistryAgentError extends Error {
	status: number;
	code?: string;
	details?: unknown;

	constructor(message: string, status = 500, options: { code?: string; details?: unknown } = {}) {
		super(message);
		this.name = 'RegistryAgentError';
		this.status = status;
		this.code = options.code;
		this.details = options.details;
	}
}

interface DispatchOptions {
	operatorId?: string;
	timeoutMs?: number;
}

type RegistryRequestResult<T extends RegistryCommandRequest> = T extends { operation: 'list' }
	? RegistryListResult
	: RegistryMutationResult;

function normalizeTimeout(operation: RegistryCommandRequest['operation'], requested?: number) {
	const baseline = operation === 'list' ? DEFAULT_TIMEOUT_MS : MUTATION_TIMEOUT_MS;
	if (typeof requested !== 'number' || Number.isNaN(requested) || requested <= 0) {
		return baseline;
	}
	const clamped = Math.min(Math.max(Math.floor(requested), 1_000), MAX_TIMEOUT_MS);
	return Math.max(clamped, baseline);
}

function ensureCommandPayload(request: RegistryCommandRequest) {
	return registryCommandPayloadSchema.parse({ request });
}

async function waitForCommandResult(agentId: string, commandId: string, timeoutMs: number) {
	const start = Date.now();
	let delay = POLL_INTERVAL_MS;

	while (Date.now() - start <= timeoutMs) {
		let snapshot: ReturnType<typeof registry.getAgent>;
		try {
			snapshot = registry.getAgent(agentId);
		} catch (err) {
			if (err instanceof RegistryError) {
				throw new RegistryAgentError(err.message, err.status);
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

	throw new RegistryAgentError('Timed out waiting for agent response', 504);
}

type CommandResultSnapshot = {
	commandId: string;
	success: boolean;
	output?: string;
	error?: string;
	completedAt: string;
};

function extractResult<T extends RegistryCommandRequest>(
	request: T,
	decoded: RegistryCommandResponse
): RegistryRequestResult<T> {
	if (decoded.operation !== request.operation) {
		throw new RegistryAgentError('Agent returned mismatched registry response', 502);
	}

	if (decoded.status === 'error') {
		throw new RegistryAgentError(
			decoded.error || 'Agent reported registry operation failure',
			502,
			{
				code: decoded.code,
				details: decoded.details
			}
		);
	}

	if (decoded.status !== 'ok') {
		throw new RegistryAgentError('Agent returned an unknown registry response', 502);
	}

	if (!('result' in decoded)) {
		throw new RegistryAgentError('Agent response missing registry payload', 502);
	}

	return decoded.result as RegistryRequestResult<T>;
}

export async function dispatchRegistryCommand<T extends RegistryCommandRequest>(
	agentId: string,
	request: T,
	options: DispatchOptions = {}
): Promise<RegistryRequestResult<T>> {
	let queuedCommandId: string;
	try {
		const payload = ensureCommandPayload(request);
		const queued = registry.queueCommand(
			agentId,
			{ name: 'registry', payload },
			{ operatorId: options.operatorId }
		);
		queuedCommandId = queued.command.id;
	} catch (err) {
		if (err instanceof RegistryError) {
			throw new RegistryAgentError(err.message, err.status);
		}
		throw new RegistryAgentError('Failed to queue registry command', 500);
	}

	const timeout = normalizeTimeout(request.operation, options.timeoutMs);

	let result: CommandResultSnapshot;
	try {
		result = await waitForCommandResult(agentId, queuedCommandId, timeout);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw new RegistryAgentError(err.message, err.status);
		}
		throw err;
	}

	if (!result.success) {
		throw new RegistryAgentError(result.error || 'Agent failed to execute registry command', 502);
	}

	if (!result.output) {
		throw new RegistryAgentError('Agent response missing registry payload', 502);
	}

	let decoded: RegistryCommandResponse;
	try {
		const parsed = JSON.parse(result.output) as unknown;
		decoded = registryCommandResponseSchema.parse(parsed);
	} catch (err) {
		throw new RegistryAgentError(
			`Agent response payload malformed: ${(err as Error).message || 'invalid JSON'}`,
			502
		);
	}

	return extractResult(request, decoded);
}
