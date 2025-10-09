import { error, json } from '@sveltejs/kit';
import { registry } from '$lib/server/rat/store';
import { remoteDesktopManager } from '$lib/server/rat/remote-desktop';
import type {
	RemoteDesktopCommandPayload,
	RemoteDesktopInputEvent,
	RemoteDesktopMouseButton
} from '$lib/types/remote-desktop';
import type { RequestHandler } from './$types';

type RawInputEvent = Record<string, unknown>;

const mouseButtons = new Set<RemoteDesktopMouseButton>(['left', 'middle', 'right']);

const clampMonitorIndex = (value: unknown) => {
	if (typeof value !== 'number') return null;
	if (!Number.isFinite(value)) return null;
	const parsed = Math.trunc(value);
	return parsed >= 0 ? parsed : null;
};

const toFiniteNumber = (value: unknown) => {
	if (typeof value === 'number') {
		return Number.isFinite(value) ? value : null;
	}
	if (typeof value === 'string' && value.trim() !== '') {
		const parsed = Number.parseFloat(value);
		return Number.isFinite(parsed) ? parsed : null;
	}
	return null;
};

const toBoolean = (value: unknown, fallback = false) => {
	return typeof value === 'boolean' ? value : fallback;
};

function sanitizeInputEvent(
	raw: RawInputEvent,
	allowMouse: boolean,
	allowKeyboard: boolean
): RemoteDesktopInputEvent | null {
	const type = typeof raw.type === 'string' ? raw.type : '';
	if (!type) {
		return null;
	}

	if (type === 'mouse-move' || type === 'mouse-button' || type === 'mouse-scroll') {
		if (!allowMouse) {
			return null;
		}
	}
	if (type === 'key' && !allowKeyboard) {
		return null;
	}

	switch (type) {
		case 'mouse-move': {
			const x = toFiniteNumber(raw.x);
			const y = toFiniteNumber(raw.y);
			if (x === null || y === null) {
				return null;
			}
			const monitor = clampMonitorIndex(raw.monitor);
			const event: RemoteDesktopInputEvent = {
				type: 'mouse-move',
				x,
				y,
				normalized: raw.normalized === true
			};
			if (monitor !== null) {
				event.monitor = monitor;
			}
			return event;
		}
		case 'mouse-button': {
			const button =
				typeof raw.button === 'string' ? (raw.button as RemoteDesktopMouseButton) : null;
			if (!button || !mouseButtons.has(button)) {
				return null;
			}
			if (typeof raw.pressed !== 'boolean') {
				return null;
			}
			const monitor = clampMonitorIndex(raw.monitor);
			const event: RemoteDesktopInputEvent = {
				type: 'mouse-button',
				button,
				pressed: raw.pressed
			};
			if (monitor !== null) {
				event.monitor = monitor;
			}
			return event;
		}
		case 'mouse-scroll': {
			const deltaX = toFiniteNumber(raw.deltaX) ?? 0;
			const deltaY = toFiniteNumber(raw.deltaY) ?? 0;
			if (deltaX === 0 && deltaY === 0) {
				return null;
			}
			const monitor = clampMonitorIndex(raw.monitor);
			const event: RemoteDesktopInputEvent = {
				type: 'mouse-scroll',
				deltaX,
				deltaY
			};
			const deltaMode = toFiniteNumber(raw.deltaMode);
			if (deltaMode !== null) {
				event.deltaMode = Math.trunc(deltaMode);
			}
			if (monitor !== null) {
				event.monitor = monitor;
			}
			return event;
		}
		case 'key': {
			if (typeof raw.pressed !== 'boolean') {
				return null;
			}
			const event: RemoteDesktopInputEvent = {
				type: 'key',
				pressed: raw.pressed,
				repeat: toBoolean(raw.repeat, false),
				altKey: toBoolean(raw.altKey, false),
				ctrlKey: toBoolean(raw.ctrlKey, false),
				shiftKey: toBoolean(raw.shiftKey, false),
				metaKey: toBoolean(raw.metaKey, false)
			};
			if (typeof raw.key === 'string') {
				event.key = raw.key;
			}
			if (typeof raw.code === 'string') {
				event.code = raw.code;
			}
			const keyCode = toFiniteNumber(raw.keyCode);
			if (keyCode !== null) {
				event.keyCode = Math.trunc(keyCode);
			}
			return event;
		}
		default:
			return null;
	}
}

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

	const eventsRaw = Array.isArray(payload.events) ? (payload.events as RawInputEvent[]) : [];
	if (eventsRaw.length === 0) {
		throw error(400, 'No input events provided');
	}

	const session = remoteDesktopManager.getSessionState(id);
	if (!session || !session.active || session.sessionId !== sessionId) {
		throw error(404, 'No active remote desktop session');
	}

	const allowMouse = session.settings.mouse === true;
	const allowKeyboard = session.settings.keyboard === true;

	const sanitized: RemoteDesktopInputEvent[] = [];
	for (const raw of eventsRaw) {
		if (!raw || typeof raw !== 'object') {
			continue;
		}
		const event = sanitizeInputEvent(raw, allowMouse, allowKeyboard);
		if (event) {
			sanitized.push(event);
		}
	}

	if (sanitized.length === 0) {
		return json({ accepted: false, reason: 'filtered' });
	}

	const command: RemoteDesktopCommandPayload = {
		action: 'input',
		sessionId: session.sessionId,
		events: sanitized
	};

	try {
		registry.queueCommand(id, { name: 'remote-desktop', payload: command });
	} catch (err) {
		throw error(500, 'Failed to queue remote desktop input command');
	}

	return json({ accepted: true, count: sanitized.length });
};
