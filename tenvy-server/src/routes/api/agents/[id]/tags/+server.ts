import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type {
	AgentTagUpdateRequest,
	AgentTagUpdateResponse
} from '../../../../../../../shared/types/agent';

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: AgentTagUpdateRequest;
	try {
		payload = (await request.json()) as AgentTagUpdateRequest;
	} catch {
		throw error(400, 'Invalid tag update payload');
	}

	if (!payload || !Array.isArray(payload.tags)) {
		throw error(400, 'Tag payload must provide an array of tags');
	}

	try {
		const agent = registry.updateAgentTags(id, payload.tags);
		return json({ agent } satisfies AgentTagUpdateResponse);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to update agent tags');
	}
};
