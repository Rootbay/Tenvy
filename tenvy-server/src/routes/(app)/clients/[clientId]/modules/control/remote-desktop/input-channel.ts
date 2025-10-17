import { browser } from '$app/environment';
import type { RemoteDesktopInputEvent } from '$lib/types/remote-desktop';

export interface InputChannelOptions {
	dispatch: (events: RemoteDesktopInputEvent[]) => Promise<boolean>;
	historyWindowMs?: number;
	onDispatchFailure?: (events: RemoteDesktopInputEvent[]) => void;
	onDispatchError?: (error: unknown, events: RemoteDesktopInputEvent[]) => void;
	raf?: typeof requestAnimationFrame;
	cancelRaf?: typeof cancelAnimationFrame;
}

export interface InputChannel {
	captureTimestamp(): number;
	normalize(event: RemoteDesktopInputEvent): void;
	enqueue(event: RemoteDesktopInputEvent): void;
	enqueueBatch(events: RemoteDesktopInputEvent[]): void;
	clear(): void;
	resolveLatency(presentationMs: number): number | null;
	computeLatency(timestamp?: string | null): number | null;
}

const DEFAULT_HISTORY_WINDOW_MS = 4_000;

export function createInputChannel(options: InputChannelOptions): InputChannel {
	const queue: RemoteDesktopInputEvent[] = [];
	const history: number[] = [];
	const historyWindowMs = options.historyWindowMs ?? DEFAULT_HISTORY_WINDOW_MS;

	let flushHandle: number | null = null;
	let sending = false;

	const raf =
		options.raf ??
		(browser && typeof requestAnimationFrame === 'function' ? requestAnimationFrame : undefined);
	const cancelRaf =
		options.cancelRaf ??
		(browser && typeof cancelAnimationFrame === 'function' ? cancelAnimationFrame : undefined);

	const captureTimestamp = () => {
		if (typeof performance !== 'undefined' && typeof performance.now === 'function') {
			return Math.round(performance.timeOrigin + performance.now());
		}
		return Date.now();
	};

	const trimHistory = (cutoff: number) => {
		if (history.length === 0) {
			return;
		}
		let removeCount = 0;
		for (const value of history) {
			if (value < cutoff) {
				removeCount += 1;
				continue;
			}
			break;
		}
		if (removeCount > 0) {
			history.splice(0, removeCount);
		}
	};

	const recordTimestamp = (timestamp: number) => {
		if (!Number.isFinite(timestamp)) {
			return;
		}
		history.push(timestamp);
		trimHistory(timestamp - historyWindowMs);
	};

	const normalize = (event: RemoteDesktopInputEvent) => {
		if (typeof event.capturedAt !== 'number' || !Number.isFinite(event.capturedAt)) {
			event.capturedAt = captureTimestamp();
			return;
		}
		event.capturedAt = Math.trunc(event.capturedAt);
	};

	const resolveLatency = (presentationMs: number) => {
		if (!Number.isFinite(presentationMs)) {
			return null;
		}
		let anchor: number | null = null;
		for (let index = history.length - 1; index >= 0; index -= 1) {
			const value = history[index];
			if (value <= presentationMs) {
				anchor = value;
				break;
			}
		}
		trimHistory(presentationMs - historyWindowMs);
		if (anchor === null) {
			const delta = Date.now() - presentationMs;
			return delta < 0 ? 0 : delta;
		}
		const delta = presentationMs - anchor;
		return delta < 0 ? 0 : delta;
	};

	const computeLatency = (timestamp?: string | null) => {
		if (!timestamp) {
			return null;
		}
		const parsed = Date.parse(timestamp);
		if (Number.isNaN(parsed)) {
			return null;
		}
		return resolveLatency(parsed);
	};

	const scheduleFlush = () => {
		if (!browser || queue.length === 0 || typeof raf !== 'function') {
			return;
		}
		if (flushHandle !== null) {
			return;
		}
		flushHandle = raf(() => {
			flushHandle = null;
			void flush();
		});
	};

	const flush = async () => {
		if (sending || queue.length === 0) {
			return;
		}
		sending = true;
		const events = queue.splice(0, queue.length);
		try {
			const success = await options.dispatch(events);
			if (!success) {
				queue.unshift(...events);
				options.onDispatchFailure?.(events);
			}
		} catch (error) {
			queue.unshift(...events);
			options.onDispatchError?.(error, events);
		} finally {
			sending = false;
			if (queue.length > 0) {
				scheduleFlush();
			}
		}
	};

	const enqueue = (event: RemoteDesktopInputEvent) => {
		if (!browser) {
			return;
		}
		normalize(event);
		if (typeof event.capturedAt === 'number') {
			recordTimestamp(event.capturedAt);
		}
		queue.push(event);
		scheduleFlush();
	};

	const enqueueBatch = (events: RemoteDesktopInputEvent[]) => {
		if (!browser || events.length === 0) {
			return;
		}
		for (const event of events) {
			normalize(event);
			if (typeof event.capturedAt === 'number') {
				recordTimestamp(event.capturedAt);
			}
			queue.push(event);
		}
		scheduleFlush();
	};

	const clear = () => {
		queue.length = 0;
		history.length = 0;
		if (browser && flushHandle !== null && typeof cancelRaf === 'function') {
			cancelRaf(flushHandle);
			flushHandle = null;
		}
	};

	return {
		captureTimestamp,
		normalize,
		enqueue,
		enqueueBatch,
		clear,
		resolveLatency,
		computeLatency
	};
}
