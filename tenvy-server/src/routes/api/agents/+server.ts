import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry } from '$lib/server/rat/store';
import type { AgentListResponse } from '../../../../../shared/types/agent';

export const GET: RequestHandler = () => {
        const agents = registry.listAgents();
        return json({ agents } satisfies AgentListResponse);
};
