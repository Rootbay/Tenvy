import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import { dispatchStartupCommand, StartupManagerAgentError } from '$lib/server/rat/startup-manager';
import type { StartupEntryDefinition } from '$lib/types/startup-manager';

const SCOPES = new Set(['machine', 'user', 'scheduled-task']);
const SOURCES = new Set(['registry', 'startup-folder', 'scheduled-task', 'service', 'other']);

function ensureDefinition(input: unknown): StartupEntryDefinition {
        if (!input || typeof input !== 'object') {
                throw error(400, 'Startup entry definition is required');
        }

        const candidate = input as Partial<StartupEntryDefinition> & { enabled?: unknown };
        const name = typeof candidate.name === 'string' ? candidate.name.trim() : '';
        if (!name) {
                throw error(400, 'Startup entry name is required');
        }

        const path = typeof candidate.path === 'string' ? candidate.path.trim() : '';
        if (!path) {
                throw error(400, 'Startup entry path is required');
        }

        const location = typeof candidate.location === 'string' ? candidate.location.trim() : '';
        if (!location) {
                throw error(400, 'Startup entry location is required');
        }

        const scope = typeof candidate.scope === 'string' ? candidate.scope.trim().toLowerCase() : '';
        if (!SCOPES.has(scope)) {
                throw error(400, 'Startup entry scope is invalid');
        }

        const source = typeof candidate.source === 'string' ? candidate.source.trim().toLowerCase() : '';
        if (!SOURCES.has(source)) {
                throw error(400, 'Startup entry source is invalid');
        }

        const enabled = typeof candidate.enabled === 'boolean' ? candidate.enabled : true;
        const publisher = typeof candidate.publisher === 'string' ? candidate.publisher.trim() || undefined : undefined;
        const description =
                typeof candidate.description === 'string' ? candidate.description.trim() || undefined : undefined;
        const args = typeof candidate.arguments === 'string' ? candidate.arguments.trim() || undefined : undefined;

        return {
                name,
                path,
                location,
                scope: scope as StartupEntryDefinition['scope'],
                source: source as StartupEntryDefinition['source'],
                enabled,
                publisher,
                description,
                arguments: args
        } satisfies StartupEntryDefinition;
}

export const GET: RequestHandler = async ({ params, locals }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        requireViewer(locals.user);

        try {
                const result = await dispatchStartupCommand(id, { operation: 'list' });
                return json(result);
        } catch (err) {
                if (err instanceof StartupManagerAgentError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to retrieve startup inventory');
        }
};

export const POST: RequestHandler = async ({ params, request, locals }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        const user = requireOperator(locals.user);

        let payload: unknown;
        try {
                payload = await request.json();
        } catch {
                throw error(400, 'Invalid startup entry payload');
        }

        const definition = ensureDefinition(payload);

        try {
                const created = await dispatchStartupCommand(
                        id,
                        { operation: 'create', definition },
                        { operatorId: user.id }
                );
                return json(created, { status: 201 });
        } catch (err) {
                if (err instanceof StartupManagerAgentError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to create startup entry');
        }
};
