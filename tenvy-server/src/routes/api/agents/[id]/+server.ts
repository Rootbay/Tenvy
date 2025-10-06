import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type { AgentDetailResponse } from '../../../../../../shared/types/agent';

export const GET: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	try {
		const agent = registry.getAgent(id);
		return json({ agent } satisfies AgentDetailResponse);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to load agent');
	}
};
