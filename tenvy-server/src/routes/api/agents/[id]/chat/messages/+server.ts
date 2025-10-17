import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { clientChatManager, ClientChatError } from '$lib/server/rat/client-chat';
import type { ClientChatMessageEnvelope, ClientChatMessageResponse } from '$lib/types/client-chat';

export const POST: RequestHandler = async ({ params, request }) => {
	const agentId = params.id;
	if (!agentId) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: ClientChatMessageEnvelope;
	try {
		payload = (await request.json()) as ClientChatMessageEnvelope;
	} catch {
		throw error(400, 'Invalid chat message payload');
	}

	try {
		const result = clientChatManager.registerClientMessage(agentId, payload);
		const response: ClientChatMessageResponse = result;
		return json(response);
	} catch (err) {
		if (err instanceof ClientChatError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to ingest chat message');
	}
};
