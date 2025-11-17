import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry } from '$lib/server/rat/store';
import { requireViewer } from '$lib/server/authorization';
import type { AgentListResponse } from '../../../../../shared/types/agent';

export const GET: RequestHandler = ({ locals }) => {
	requireViewer(locals.user);

	const agents = registry.listAgents();
	return json({ agents } satisfies AgentListResponse);
};
