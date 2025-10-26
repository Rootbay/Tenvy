import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator } from '$lib/server/authorization';
import { dispatchStartupCommand, StartupManagerAgentError } from '$lib/server/rat/startup-manager';

export const PATCH: RequestHandler = async ({ params, request, locals }) => {
        const id = params.id;
        const entryId = params.entryId;
        if (!id || !entryId) {
                throw error(400, 'Missing agent or entry identifier');
        }

        const user = requireOperator(locals.user);

        let payload: unknown;
        try {
                payload = await request.json();
        } catch {
                throw error(400, 'Invalid startup toggle payload');
        }

        if (!payload || typeof payload !== 'object' || typeof (payload as { enabled?: unknown }).enabled !== 'boolean') {
                throw error(400, 'Startup toggle requires an enabled flag');
        }

        const enabled = (payload as { enabled: boolean }).enabled;

        try {
                const updated = await dispatchStartupCommand(
                        id,
                        { operation: 'toggle', entryId, enabled },
                        { operatorId: user.id }
                );
                return json(updated);
        } catch (err) {
                if (err instanceof StartupManagerAgentError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to update startup entry');
        }
};

export const DELETE: RequestHandler = async ({ params, locals }) => {
        const id = params.id;
        const entryId = params.entryId;
        if (!id || !entryId) {
                throw error(400, 'Missing agent or entry identifier');
        }

        const user = requireOperator(locals.user);

        try {
                const result = await dispatchStartupCommand(
                        id,
                        { operation: 'remove', entryId },
                        { operatorId: user.id }
                );
                return json(result);
        } catch (err) {
                if (err instanceof StartupManagerAgentError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to remove startup entry');
        }
};
