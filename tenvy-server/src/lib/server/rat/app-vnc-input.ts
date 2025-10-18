import type { AppVncInputEvent, AppVncPointerButton } from '$lib/types/app-vnc';

export type RawAppVncInputEvent = Record<string, unknown>;

const pointerButtons = new Set<AppVncPointerButton>(['left', 'middle', 'right']);

const numberFromUnknown = (value: unknown): number | null => {
	if (typeof value === 'number') {
		return Number.isFinite(value) ? value : null;
	}
	if (typeof value === 'string' && value.trim() !== '') {
		const parsed = Number.parseFloat(value);
		return Number.isFinite(parsed) ? parsed : null;
	}
	return null;
};

const resolveCapturedAt = (value: unknown): number => {
	const parsed = numberFromUnknown(value);
	if (parsed === null) {
		return Date.now();
	}
	const normalized = Math.trunc(parsed);
	return normalized >= 0 ? normalized : Date.now();
};

const toBoolean = (value: unknown, fallback = false) => {
	return typeof value === 'boolean' ? value : fallback;
};

export function sanitizeAppVncInputEvent(raw: RawAppVncInputEvent): AppVncInputEvent | null {
	if (!raw || typeof raw !== 'object') {
		return null;
	}

	const type = typeof raw.type === 'string' ? raw.type : '';
	if (!type) {
		return null;
	}

	switch (type) {
		case 'pointer-move': {
			const x = numberFromUnknown(raw.x);
			const y = numberFromUnknown(raw.y);
			if (x === null || y === null) {
				return null;
			}
			return {
				type: 'pointer-move',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				x,
				y,
				normalized: raw.normalized === true
			};
		}
		case 'pointer-button': {
			const button =
				typeof raw.button === 'string' ? (raw.button.toLowerCase() as AppVncPointerButton) : null;
			if (!button || !pointerButtons.has(button)) {
				return null;
			}
			if (typeof raw.pressed !== 'boolean') {
				return null;
			}
			return {
				type: 'pointer-button',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				button,
				pressed: raw.pressed
			};
		}
		case 'pointer-scroll': {
			const deltaX = numberFromUnknown(raw.deltaX) ?? 0;
			const deltaY = numberFromUnknown(raw.deltaY) ?? 0;
			if (deltaX === 0 && deltaY === 0) {
				return null;
			}
			const event: AppVncInputEvent = {
				type: 'pointer-scroll',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				deltaX,
				deltaY
			};
			const deltaMode = numberFromUnknown(raw.deltaMode);
			if (deltaMode !== null) {
				event.deltaMode = Math.trunc(deltaMode);
			}
			return event;
		}
		case 'key': {
			if (typeof raw.pressed !== 'boolean') {
				return null;
			}
			const event: AppVncInputEvent = {
				type: 'key',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				pressed: raw.pressed,
				repeat: toBoolean(raw.repeat),
				altKey: toBoolean(raw.altKey),
				ctrlKey: toBoolean(raw.ctrlKey),
				shiftKey: toBoolean(raw.shiftKey),
				metaKey: toBoolean(raw.metaKey)
			};
			if (typeof raw.key === 'string') {
				event.key = raw.key;
			}
			if (typeof raw.code === 'string') {
				event.code = raw.code;
			}
			const keyCode = numberFromUnknown(raw.keyCode);
			if (keyCode !== null) {
				event.keyCode = Math.trunc(keyCode);
			}
			return event;
		}
		default:
			return null;
	}
}

export function sanitizeAppVncInputEvents(events: RawAppVncInputEvent[]): AppVncInputEvent[] {
	const sanitized: AppVncInputEvent[] = [];
	if (!Array.isArray(events) || events.length === 0) {
		return sanitized;
	}
	for (const raw of events) {
		const event = sanitizeAppVncInputEvent(raw);
		if (event) {
			sanitized.push(event);
		}
	}
	return sanitized;
}
