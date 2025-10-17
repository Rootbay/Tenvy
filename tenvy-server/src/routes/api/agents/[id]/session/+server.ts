import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { COMMAND_STREAM_SUBPROTOCOL } from '../../../../../../../shared/constants/protocol';

function extractBearerToken(headerValue: string | null): string | null {
	if (!headerValue) {
		return null;
	}
	const match = /^Bearer\s+(?<token>.+)$/i.exec(headerValue.trim());
	return match?.groups?.token?.trim() ?? null;
}

function parseSubprotocolHeader(headerValue: string | null): string[] {
	if (!headerValue) {
		return [];
	}
	return headerValue
		.split(',')
		.map((value) => value.trim())
		.filter((value) => value !== '');
}

export const GET: RequestHandler = ({ request, params, getClientAddress }) => {
	if (request.headers.get('upgrade')?.toLowerCase() !== 'websocket') {
		throw error(400, 'Expected WebSocket upgrade request');
	}

	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const key = extractBearerToken(request.headers.get('authorization'));
	if (!key) {
		throw error(401, 'Missing agent key');
	}

	const requestedProtocols = parseSubprotocolHeader(request.headers.get('sec-websocket-protocol'));
	if (!requestedProtocols.includes(COMMAND_STREAM_SUBPROTOCOL)) {
		throw error(426, 'Unsupported WebSocket protocol');
	}

	const pairFactory = (
		globalThis as {
			WebSocketPair?: new () => { 0: WebSocket; 1: WebSocket };
		}
	).WebSocketPair;

	if (!pairFactory) {
		throw error(503, 'WebSocket upgrade not supported');
	}

	const { 0: client, 1: serverSocket } = new pairFactory();

	try {
		registry.attachSession(id, key, serverSocket, { remoteAddress: getClientAddress() });
	} catch (err) {
		serverSocket.close(1008, 'Session rejected');
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to establish agent session');
	}

	return new Response(null, { status: 101, webSocket: client } as unknown as ResponseInit);
};
