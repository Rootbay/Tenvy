import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireViewer } from '$lib/server/authorization';
import { dispatchTaskManagerCommand, TaskManagerAgentError } from '$lib/server/rat/task-manager';

function parsePid(raw: string | undefined): number {
        const value = Number.parseInt(raw ?? '', 10);
        if (!Number.isInteger(value) || value <= 0) {
                throw error(400, 'Invalid process identifier');
        }
        return value;
}

export const GET: RequestHandler = async ({ params, locals }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        requireViewer(locals.user);

        const pid = parsePid(params.pid);

        try {
                const result = await dispatchTaskManagerCommand(id, { operation: 'detail', pid });
                return json(result);
        } catch (err) {
                if (err instanceof TaskManagerAgentError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to load process detail');
        }
};
