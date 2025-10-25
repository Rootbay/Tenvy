import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store.js';
import { telemetryStore, getBearerToken } from '../_shared.js';

export const GET: RequestHandler = async ({ params, request }) => {
        const id = params.id;
        const pluginId = params.pluginId;
        if (!id || !pluginId) {
                throw error(400, 'Missing identifiers');
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

        const approved = await telemetryStore.getApprovedManifest(pluginId);
        if (!approved) {
                throw error(404, 'Plugin manifest not found');
        }

        const etag = `"${approved.descriptor.manifestDigest}"`;

        return new Response(approved.record.raw, {
                headers: {
                        'Content-Type': 'application/json',
                        'Cache-Control': 'no-store',
                        ETag: etag
                }
        });
};
