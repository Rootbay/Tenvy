import { browser } from '$app/environment';
import { get, writable, type Readable } from 'svelte/store';
import type {
	AppVncInputEvent,
	AppVncSessionSettings,
	AppVncSessionState
} from '$lib/types/app-vnc';

type Encoding = 'png' | 'jpeg';

type CreateControllerOptions = {
	clientId: string;
	initialSession?: AppVncSessionState | null;
};

type SessionRequest = AppVncSessionSettings;

type SessionResponse = { session?: AppVncSessionState | null };

const IMAGE_PREFIX: Record<Encoding, string> = {
	png: 'data:image/png;base64,',
	jpeg: 'data:image/jpeg;base64,'
};

function parseResponseMessage(response: Response): Promise<string> {
	return response
		.text()
		.then((text) => text.trim())
		.catch(() => '');
}

export type AppVncSessionController = ReturnType<typeof createAppVncSessionController>;

export function createAppVncSessionController({
	clientId,
	initialSession = null
}: CreateControllerOptions) {
	const session = writable<AppVncSessionState | null>(initialSession);
	const frameUrl = writable<string | null>(null);
	const frameWidth = writable<number | null>(null);
	const frameHeight = writable<number | null>(null);
	const lastHeartbeat = writable<string | null>(null);
	const isStarting = writable(false);
	const isStopping = writable(false);
	const isUpdating = writable(false);
	const errorMessage = writable<string | null>(null);
	const infoMessage = writable<string | null>(null);

	let eventSource: EventSource | null = null;
	let streamSessionId: string | null = null;
	let flushHandle: number | null = null;
	const pendingEvents: AppVncInputEvent[] = [];

	function resetStatusMessages() {
		errorMessage.set(null);
		infoMessage.set(null);
	}

	function disconnectStream() {
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}
		streamSessionId = null;
		pendingEvents.length = 0;
		frameUrl.set(null);
		frameWidth.set(null);
		frameHeight.set(null);
	}

	function connectStream(id?: string | null) {
		if (!browser) {
			return;
		}
		const targetId = id ?? null;
		if (eventSource && streamSessionId === targetId) {
			return;
		}
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}

		const base = new URL(`/api/agents/${clientId}/app-vnc/stream`, window.location.origin);
		if (targetId) {
			base.searchParams.set('sessionId', targetId);
		}

		const source = new EventSource(base.toString());
		streamSessionId = targetId;
		eventSource = source;

		source.addEventListener('session', (evt) => {
			try {
				const payload = JSON.parse((evt as MessageEvent).data ?? '{}') as SessionResponse;
				session.set(payload.session ?? null);
				if (!payload.session?.active) {
					frameUrl.set(null);
					frameWidth.set(null);
					frameHeight.set(null);
				}
			} catch (err) {
				console.warn('Failed to parse app VNC session payload', err);
			}
		});

		source.addEventListener('frame', (evt) => {
			try {
				const payload = JSON.parse((evt as MessageEvent).data ?? '{}') as {
					frame?: {
						image?: string;
						encoding?: Encoding;
						width?: number;
						height?: number;
					};
				};
				const frame = payload.frame;
				if (frame && frame.image && frame.encoding && IMAGE_PREFIX[frame.encoding]) {
					frameUrl.set(`${IMAGE_PREFIX[frame.encoding]}${frame.image}`);
					frameWidth.set(frame.width ?? null);
					frameHeight.set(frame.height ?? null);
				}
			} catch (err) {
				console.warn('Failed to parse app VNC frame payload', err);
			}
		});

		source.addEventListener('heartbeat', (evt) => {
			try {
				const payload = JSON.parse((evt as MessageEvent).data ?? '{}') as { timestamp?: string };
				lastHeartbeat.set(payload.timestamp ?? new Date().toISOString());
			} catch {
				lastHeartbeat.set(new Date().toISOString());
			}
		});

		source.addEventListener('end', () => {
			infoMessage.set('Session closed');
			frameUrl.set(null);
			frameWidth.set(null);
			frameHeight.set(null);
		});

		source.onerror = (err) => {
			console.warn('App VNC event source error', err);
		};
	}

	async function startSession(settings: SessionRequest): Promise<AppVncSessionState | null> {
		if (!browser) {
			errorMessage.set('Sessions are unavailable in this environment');
			return get(session);
		}
		if (get(isStarting)) {
			return get(session);
		}

		resetStatusMessages();
		isStarting.set(true);
		try {
			const response = await fetch(`/api/agents/${clientId}/app-vnc/session`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(settings)
			});
			if (!response.ok) {
				const message = await parseResponseMessage(response);
				errorMessage.set(message || 'Failed to start session');
				return get(session);
			}
			const payload = (await response.json()) as SessionResponse;
			const nextSession = payload.session ?? null;
			session.set(nextSession);
			if (nextSession?.active) {
				infoMessage.set('Session started');
			}
			return nextSession;
		} catch (err) {
			console.error('Failed to start app VNC session', err);
			errorMessage.set('Failed to start session');
			return get(session);
		} finally {
			isStarting.set(false);
		}
	}

	async function updateSession(settings: SessionRequest): Promise<AppVncSessionState | null> {
		const current = get(session);
		if (!browser || !current?.active) {
			return current;
		}
		if (get(isUpdating)) {
			return get(session);
		}

		resetStatusMessages();
		isUpdating.set(true);
		try {
			const response = await fetch(`/api/agents/${clientId}/app-vnc/session`, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ sessionId: current.sessionId, ...settings })
			});
			if (!response.ok) {
				const message = await parseResponseMessage(response);
				errorMessage.set(message || 'Failed to update session');
				return get(session);
			}
			const payload = (await response.json()) as SessionResponse;
			const nextSession = payload.session ?? null;
			session.set(nextSession);
			if (nextSession?.active) {
				infoMessage.set('Session updated');
			}
			return nextSession;
		} catch (err) {
			console.error('Failed to update app VNC session', err);
			errorMessage.set('Failed to update session');
			return get(session);
		} finally {
			isUpdating.set(false);
		}
	}

	async function stopSession(): Promise<AppVncSessionState | null> {
		const current = get(session);
		if (!browser || !current?.active) {
			return current;
		}
		if (get(isStopping)) {
			return get(session);
		}

		resetStatusMessages();
		isStopping.set(true);
		try {
			const response = await fetch(`/api/agents/${clientId}/app-vnc/session`, {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ sessionId: current.sessionId })
			});
			if (!response.ok) {
				const message = await parseResponseMessage(response);
				errorMessage.set(message || 'Failed to stop session');
				return get(session);
			}
			const payload = (await response.json()) as SessionResponse;
			const nextSession = payload.session ?? null;
			session.set(nextSession);
			frameUrl.set(null);
			frameWidth.set(null);
			frameHeight.set(null);
			pendingEvents.length = 0;
			infoMessage.set('Session stopped');
			return nextSession;
		} catch (err) {
			console.error('Failed to stop app VNC session', err);
			errorMessage.set('Failed to stop session');
			return get(session);
		} finally {
			isStopping.set(false);
		}
	}

	async function refreshSession(): Promise<AppVncSessionState | null> {
		if (!browser) {
			return get(session);
		}
		try {
			const response = await fetch(`/api/agents/${clientId}/app-vnc/session`);
			if (!response.ok) {
				return get(session);
			}
			const payload = (await response.json()) as SessionResponse;
			const nextSession = payload.session ?? null;
			session.set(nextSession);
			return nextSession;
		} catch {
			return get(session);
		}
	}

	async function flushEvents() {
		if (!browser) {
			pendingEvents.length = 0;
			return;
		}
		const current = get(session);
		if (!current?.active || pendingEvents.length === 0) {
			pendingEvents.length = 0;
			return;
		}
		const events = pendingEvents.splice(0, pendingEvents.length);
		try {
			await fetch(`/api/agents/${clientId}/app-vnc/input`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ sessionId: current.sessionId, events }),
				keepalive: true
			});
		} catch (err) {
			console.warn('Failed to deliver app VNC input events', err);
		}
	}

	function enqueueEvent(event: AppVncInputEvent) {
		pendingEvents.push(event);
		if (!browser) {
			return;
		}
		if (flushHandle !== null) {
			return;
		}
		flushHandle = requestAnimationFrame(() => {
			flushHandle = null;
			void flushEvents();
		});
	}

	const unsubscribe = session.subscribe((current) => {
		if (!browser) {
			return;
		}
		if (current && current.active) {
			connectStream(current.sessionId);
		} else {
			disconnectStream();
		}
	});

	function dispose() {
		if (flushHandle !== null) {
			cancelAnimationFrame(flushHandle);
			flushHandle = null;
		}
		unsubscribe();
		disconnectStream();
	}

	const readableStores: Record<string, Readable<unknown>> = {
		session,
		frameUrl,
		frameWidth,
		frameHeight,
		lastHeartbeat,
		isStarting,
		isStopping,
		isUpdating,
		errorMessage,
		infoMessage
	};

	return {
		...readableStores,
		startSession,
		updateSession,
		stopSession,
		refreshSession,
		enqueueEvent,
		resetStatusMessages,
		dispose
	} as {
		session: typeof session;
		frameUrl: typeof frameUrl;
		frameWidth: typeof frameWidth;
		frameHeight: typeof frameHeight;
		lastHeartbeat: typeof lastHeartbeat;
		isStarting: typeof isStarting;
		isStopping: typeof isStopping;
		isUpdating: typeof isUpdating;
		errorMessage: typeof errorMessage;
		infoMessage: typeof infoMessage;
		startSession: typeof startSession;
		updateSession: typeof updateSession;
		stopSession: typeof stopSession;
		refreshSession: typeof refreshSession;
		enqueueEvent: typeof enqueueEvent;
		resetStatusMessages: typeof resetStatusMessages;
		dispose: typeof dispose;
	};
}
