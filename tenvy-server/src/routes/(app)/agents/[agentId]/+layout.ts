import { error } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';
import type { AgentDetailResponse } from '../../../../../../shared/types/agent';

export const load = (async ({ params, fetch }) => {
	const id = params.agentId;
	if (!id) {
		throw error(404, 'Agent not found');
	}

	const response = await fetch(`/api/agents/${id}`);
	if (!response.ok) {
		throw error(response.status, 'Failed to load agent');
	}

	const data = (await response.json()) as AgentDetailResponse;
	return { agent: data.agent };
}) satisfies LayoutLoad;
