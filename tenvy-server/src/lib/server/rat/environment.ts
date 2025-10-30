import type {
	EnvironmentCommandRequest,
	EnvironmentCommandResponse,
	EnvironmentMutationResult,
	EnvironmentSnapshot
} from '$lib/types/environment';
import {
	environmentCommandRequestSchema,
	environmentCommandResponseSchema
} from '$lib/types/environment';
import { registry, RegistryError } from './store';

const DEFAULT_TIMEOUT_MS = 10_000;
const MUTATION_TIMEOUT_MS = 25_000;
const MAX_TIMEOUT_MS = 60_000;
const POLL_INTERVAL_MS = 250;

export class EnvironmentAgentError extends Error {
	status: number;
	code?: string;

	constructor(message: string, status = 500, options: { code?: string } = {}) {
		super(message);
		this.name = 'EnvironmentAgentError';
		this.status = status;
		this.code = options.code;
	}
}

interface DispatchOptions {
	operatorId?: string;
	timeoutMs?: number;
}

type EnvironmentRequestResult<T extends EnvironmentCommandRequest> = T extends {
	action: 'list';
}
	? EnvironmentSnapshot
	: EnvironmentMutationResult;

type CommandResultSnapshot = {
	commandId: string;
	success: boolean;
	output?: string;
	error?: string;
	completedAt: string;
};

function normalizeTimeout(action: EnvironmentCommandRequest['action'], requested?: number) {
	const baseline = action === 'list' ? DEFAULT_TIMEOUT_MS : MUTATION_TIMEOUT_MS;
	if (typeof requested !== 'number' || Number.isNaN(requested) || requested <= 0) {
		return baseline;
	}
	const clamped = Math.min(Math.max(Math.floor(requested), 1_000), MAX_TIMEOUT_MS);
	return Math.max(clamped, baseline);
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
				throw new EnvironmentAgentError(err.message, err.status);
			}
			throw err;
		}

		const match = snapshot.recentResults.find((entry) => entry.commandId === commandId);
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

	throw new EnvironmentAgentError('Timed out waiting for environment response', 504);
}

function extractResult<T extends EnvironmentCommandRequest>(
	request: T,
	decoded: EnvironmentCommandResponse
): EnvironmentRequestResult<T> {
	if (decoded.action !== request.action) {
		throw new EnvironmentAgentError('Agent returned mismatched environment action', 502);
	}

	if (decoded.status === 'error') {
		throw new EnvironmentAgentError(
			decoded.error || 'Agent reported environment command failure',
			502,
			{ code: decoded.code }
		);
	}

	if (decoded.status !== 'ok') {
		throw new EnvironmentAgentError('Agent returned unknown environment response', 502);
	}

	if (!('result' in decoded)) {
		throw new EnvironmentAgentError('Agent response missing environment payload', 502);
	}

	return decoded.result as EnvironmentRequestResult<T>;
}

export async function dispatchEnvironmentCommand<T extends EnvironmentCommandRequest>(
	agentId: string,
	request: T,
	options: DispatchOptions = {}
): Promise<EnvironmentRequestResult<T>> {
	const payload = environmentCommandRequestSchema.parse(request);

	let queuedCommandId: string;
	try {
		const queued = registry.queueCommand(
			agentId,
			{ name: 'environment-variables', payload },
			{ operatorId: options.operatorId }
		);
		queuedCommandId = queued.command.id;
	} catch (err) {
		if (err instanceof RegistryError) {
			throw new EnvironmentAgentError(err.message, err.status);
		}
		throw new EnvironmentAgentError('Failed to queue environment command', 500);
	}

	const timeout = normalizeTimeout(request.action, options.timeoutMs);
	let result: CommandResultSnapshot;
	try {
		result = await waitForCommandResult(agentId, queuedCommandId, timeout);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw new EnvironmentAgentError(err.message, err.status);
		}
		throw err;
	}

	if (!result.success) {
		throw new EnvironmentAgentError(
			result.error || 'Agent failed to execute environment command',
			502
		);
	}

	if (!result.output) {
		throw new EnvironmentAgentError('Agent response missing environment payload', 502);
	}

	let decoded: EnvironmentCommandResponse;
	try {
		const parsed = JSON.parse(result.output) as unknown;
		decoded = environmentCommandResponseSchema.parse(parsed);
	} catch (err) {
		throw new EnvironmentAgentError(
			`Environment response payload malformed: ${(err as Error).message || 'invalid JSON'}`,
			502
		);
	}

	return extractResult(request, decoded);
}
