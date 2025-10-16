import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { remoteDesktopManager, RemoteDesktopError } from '$lib/server/rat/remote-desktop';
import type { RemoteDesktopSessionNegotiationRequest } from '$lib/types/remote-desktop';

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: RemoteDesktopSessionNegotiationRequest;
	try {
		payload = (await request.json()) as RemoteDesktopSessionNegotiationRequest;
	} catch (err) {
		throw error(400, 'Invalid negotiation payload');
	}

	if (!payload || typeof payload.sessionId !== 'string' || payload.sessionId.length === 0) {
		throw error(400, 'Negotiation session identifier is required');
	}

	try {
		const response = await remoteDesktopManager.negotiateTransport(id, payload);
		return json(response);
	} catch (err) {
		if (err instanceof RemoteDesktopError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to negotiate remote desktop transport');
	}
};
