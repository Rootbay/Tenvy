import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator } from '$lib/server/authorization';
import { dispatchTaskManagerCommand, TaskManagerAgentError } from '$lib/server/rat/task-manager';
import type { ProcessAction, ProcessActionRequest } from '$lib/types/task-manager';

const ALLOWED_ACTIONS: readonly ProcessAction[] = [
	'stop',
	'force-stop',
	'suspend',
	'resume',
	'restart'
];

function parsePid(raw: string | undefined): number {
	const value = Number.parseInt(raw ?? '', 10);
	if (!Number.isInteger(value) || value <= 0) {
		throw error(400, 'Invalid process identifier');
	}
	return value;
}

function normalizeAction(input: ProcessActionRequest['action']): ProcessAction {
	if (!ALLOWED_ACTIONS.includes(input)) {
		throw error(400, 'Unsupported process action');
	}
	return input;
}

export const POST: RequestHandler = async ({ params, request, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const user = requireOperator(locals.user);
	const pid = parsePid(params.pid);

	let payload: ProcessActionRequest;
	try {
		payload = (await request.json()) as ProcessActionRequest;
	} catch {
		throw error(400, 'Invalid process action payload');
	}

	const action = normalizeAction(payload.action);

	try {
		const result = await dispatchTaskManagerCommand(
			id,
			{ operation: 'action', pid, action },
			{ operatorId: user.id }
		);
		return json(result);
	} catch (err) {
		if (err instanceof TaskManagerAgentError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to execute process action');
	}
};
