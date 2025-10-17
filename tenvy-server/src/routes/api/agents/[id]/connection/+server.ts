import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type {
	AgentConnectionAction,
	AgentConnectionRequest,
	AgentConnectionResponse
} from '../../../../../../../shared/types/agent';

function isConnectionAction(value: unknown): value is AgentConnectionAction {
	return value === 'disconnect' || value === 'reconnect';
}

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: AgentConnectionRequest;
	try {
		payload = (await request.json()) as AgentConnectionRequest;
	} catch {
		throw error(400, 'Invalid connection request payload');
	}

	if (!isConnectionAction(payload?.action)) {
		throw error(400, 'Unsupported connection action');
	}

	try {
		const agent =
			payload.action === 'disconnect' ? registry.disconnectAgent(id) : registry.reconnectAgent(id);

		return json({ agent } satisfies AgentConnectionResponse);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to update agent connection');
	}
};
