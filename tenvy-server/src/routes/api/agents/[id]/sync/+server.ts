import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type { AgentSyncRequest } from '../../../../../../../shared/types/messages';

function getBearerToken(header: string | null): string | undefined {
        if (!header) {
                return undefined;
        }
        const match = header.match(/^Bearer\s+(.+)$/i);
        return match?.[1]?.trim();
}

export const POST: RequestHandler = async ({ params, request }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        let payload: AgentSyncRequest;
        try {
                payload = (await request.json()) as AgentSyncRequest;
        } catch (err) {
                throw error(400, 'Invalid sync payload');
        }

        const token = getBearerToken(request.headers.get('authorization'));
        if (!token) {
                throw error(401, 'Missing agent key');
        }

        try {
                const response = registry.syncAgent(id, token, payload);
                return json(response);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to sync agent');
        }
};
