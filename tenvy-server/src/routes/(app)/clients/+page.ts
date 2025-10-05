import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { AgentListResponse } from '../../../../../shared/types/agent';

export const load: PageLoad = async ({ fetch }) => {
        const response = await fetch('/api/agents');
        if (!response.ok) {
                throw error(response.status, 'Failed to load agents');
        }

        const data = (await response.json()) as AgentListResponse;
        return { agents: data.agents };
};
