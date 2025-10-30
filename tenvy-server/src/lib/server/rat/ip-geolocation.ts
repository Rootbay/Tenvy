import type {
	GeoCommandRequest,
	GeoCommandResponse,
	GeoLookupResult,
	GeoStatus
} from '$lib/types/ip-geolocation';
import { geoCommandRequestSchema, geoCommandResponseSchema } from '$lib/types/ip-geolocation';
import { registry, RegistryError } from './store';

const STATUS_TIMEOUT_MS = 6_000;
const LOOKUP_TIMEOUT_MS = 20_000;
const MAX_TIMEOUT_MS = 90_000;
const POLL_INTERVAL_MS = 200;

export class GeoLookupAgentError extends Error {
	status: number;
	code?: string;

	constructor(message: string, status = 500, options: { code?: string } = {}) {
		super(message);
		this.name = 'GeoLookupAgentError';
		this.status = status;
		this.code = options.code;
	}
}

interface DispatchOptions {
	operatorId?: string;
	timeoutMs?: number;
}

type GeoRequestResult<T extends GeoCommandRequest> = T extends { action: 'status' }
	? GeoStatus
	: GeoLookupResult;

type CommandResultSnapshot = {
	commandId: string;
	success: boolean;
	output?: string;
	error?: string;
	completedAt: string;
};

function normalizeTimeout(action: GeoCommandRequest['action'], requested?: number) {
	const baseline = action === 'status' ? STATUS_TIMEOUT_MS : LOOKUP_TIMEOUT_MS;
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
				throw new GeoLookupAgentError(err.message, err.status);
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

	throw new GeoLookupAgentError('Timed out waiting for geolocation response', 504);
}

function extractResult<T extends GeoCommandRequest>(
	request: T,
	decoded: GeoCommandResponse
): GeoRequestResult<T> {
	if (decoded.action !== request.action) {
		throw new GeoLookupAgentError('Agent returned mismatched geolocation action', 502);
	}

	if (decoded.status === 'error') {
		throw new GeoLookupAgentError(decoded.error || 'Agent reported geolocation failure', 502, {
			code: decoded.code
		});
	}

	if (decoded.status !== 'ok') {
		throw new GeoLookupAgentError('Agent returned unknown geolocation response', 502);
	}

	if (!('result' in decoded)) {
		throw new GeoLookupAgentError('Agent response missing geolocation payload', 502);
	}

	return decoded.result as GeoRequestResult<T>;
}

export async function dispatchGeoCommand<T extends GeoCommandRequest>(
	agentId: string,
	request: T,
	options: DispatchOptions = {}
): Promise<GeoRequestResult<T>> {
	const payload = geoCommandRequestSchema.parse(request);

	let queuedCommandId: string;
	try {
		const queued = registry.queueCommand(
			agentId,
			{ name: 'ip-geolocation', payload },
			{ operatorId: options.operatorId }
		);
		queuedCommandId = queued.command.id;
	} catch (err) {
		if (err instanceof RegistryError) {
			throw new GeoLookupAgentError(err.message, err.status);
		}
		throw new GeoLookupAgentError('Failed to queue geolocation command', 500);
	}

	const timeout = normalizeTimeout(request.action, options.timeoutMs);
	let result: CommandResultSnapshot;
	try {
		result = await waitForCommandResult(agentId, queuedCommandId, timeout);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw new GeoLookupAgentError(err.message, err.status);
		}
		throw err;
	}

	if (!result.success) {
		throw new GeoLookupAgentError(
			result.error || 'Agent failed to execute geolocation command',
			502
		);
	}

	if (!result.output) {
		throw new GeoLookupAgentError('Agent response missing geolocation payload', 502);
	}

	let decoded: GeoCommandResponse;
	try {
		const parsed = JSON.parse(result.output) as unknown;
		decoded = geoCommandResponseSchema.parse(parsed);
	} catch (err) {
		throw new GeoLookupAgentError(
			`Geolocation response payload malformed: ${(err as Error).message || 'invalid JSON'}`,
			502
		);
	}

	return extractResult(request, decoded);
}
