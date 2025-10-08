<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import type { Client } from '$lib/data/clients';
        import type {
                RemoteDesktopFramePacket,
                RemoteDesktopInputEvent,
                RemoteDesktopMonitor,
                RemoteDesktopMouseButton,
                RemoteDesktopSessionState,
                RemoteDesktopSettings
        } from '$lib/types/remote-desktop';

	const fallbackMonitors = [
		{ id: 0, label: 'Primary', width: 1280, height: 720 }
	] satisfies RemoteDesktopMonitor[];

	const qualityOptions = [
		{ value: 'auto', label: 'Auto' },
		{ value: 'high', label: 'High' },
		{ value: 'medium', label: 'Medium' },
		{ value: 'low', label: 'Low' }
	] satisfies { value: RemoteDesktopSettings['quality']; label: string }[];

	const modeOptions = [
		{ value: 'video', label: 'Video clips (100–300 ms)' },
		{ value: 'images', label: 'Still frames' }
	] satisfies { value: RemoteDesktopSettings['mode']; label: string }[];

	const MAX_FRAME_QUEUE = 24;
	const supportsImageBitmap = browser && typeof createImageBitmap === 'function';
	const IMAGE_BASE64_PREFIX = {
		png: 'data:image/png;base64,',
		jpeg: 'data:image/jpeg;base64,'
	} as const;

	let { data } = $props<{ data: { session: RemoteDesktopSessionState | null; client: Client } }>();

	const client = $derived(data.client);
	let session = $state<RemoteDesktopSessionState | null>(data.session ?? null);
	let activeTab = $state<'stream' | 'controls'>('stream');
	let quality = $state<RemoteDesktopSettings['quality']>('auto');
	let mode = $state<RemoteDesktopSettings['mode']>('video');
	let monitor = $state(0);
	let mouseEnabled = $state(false);
	let keyboardEnabled = $state(false);
	let fps = $state<number | null>(null);
	let gpu = $state<number | null>(null);
	let cpu = $state<number | null>(null);
	let bandwidth = $state<number | null>(null);
	let clipQuality = $state<number | null>(null);
	let streamWidth = $state<number | null>(null);
	let streamHeight = $state<number | null>(null);
	let latencyMs = $state<number | null>(null);
	let droppedFrames = $state(0);
let isStarting = $state(false);
	let isStopping = $state(false);
	let isUpdating = $state(false);
	let errorMessage = $state<string | null>(null);
	let infoMessage = $state<string | null>(null);
	let monitors = $state<RemoteDesktopMonitor[]>(fallbackMonitors);
        let sessionActive = $state(false);
        let sessionId = $state('');

        let viewportEl: HTMLDivElement | null = null;
        let viewportFocused = false;
        let pointerCaptured = false;
        let activePointerId: number | null = null;
        let inputQueue: RemoteDesktopInputEvent[] = [];
        let inputFlushHandle: number | null = null;
        let inputSending = false;
        const pressedKeys = new Set<number>();
        const pressedKeyMeta = new Map<number, { key?: string; code?: string }>();

        const pointerButtonMap: Record<number, RemoteDesktopMouseButton> = {
                0: 'left',
                1: 'middle',
                2: 'right'
        };

	let canvasEl: HTMLCanvasElement | null = null;
	let canvasContext: CanvasRenderingContext2D | null = null;
	let eventSource: EventSource | null = null;
	let streamSessionId: string | null = null;
	let frameQueue: RemoteDesktopFramePacket[] = [];
	let processing = false;
	let stopRequested = false;
	let imageBitmapFallbackLogged = false;

	let skipMouseSync = true;
	let skipKeyboardSync = true;

	const qualityLabel = (value: string) => {
		const found = qualityOptions.find((item) => item.value === value);
		return found ? found.label : value;
	};

	const monitorLabel = (id: number) => {
		const list = monitors;
		const found = list.find((item: RemoteDesktopMonitor) => item.id === id);
		if (!found) {
			return `Monitor ${id + 1}`;
		}
		return `${found.label} · ${found.width}×${found.height}`;
	};

        const modeLabel = (value: string) => {
                const found = modeOptions.find((item) => item.value === value);
                return found ? found.label : value;
        };

        const clamp = (value: number, min: number, max: number) => {
                if (Number.isNaN(value)) return min;
                if (value < min) return min;
                if (value > max) return max;
                return value;
        };

	function resetMetrics() {
		fps = null;
		gpu = null;
		cpu = null;
		bandwidth = null;
		clipQuality = null;
		streamWidth = null;
		streamHeight = null;
		latencyMs = null;
		droppedFrames = 0;
	}

	function disconnectStream() {
		if (eventSource) {
			eventSource.close();
			eventSource = null;
		}
		streamSessionId = null;
		stopRequested = true;
		frameQueue = [];
		processing = false;
		imageBitmapFallbackLogged = false;
	}

	function connectStream(id?: string) {
		if (!browser) return;
		const targetId = id ?? null;
		if (eventSource && streamSessionId === targetId) {
			return;
		}

		disconnectStream();
		stopRequested = false;

		const base = new URL(`/api/agents/${client.id}/remote-desktop/stream`, window.location.origin);
		if (targetId) {
			base.searchParams.set('sessionId', targetId);
		}

		eventSource = new EventSource(base.toString());
		streamSessionId = targetId;

		eventSource.addEventListener('session', (event) => {
			const parsed = parseSessionEvent(event as MessageEvent);
			if (parsed) {
				session = parsed;
				if (!parsed.active) {
					disconnectStream();
				}
			}
		});

		eventSource.addEventListener('frame', (event) => {
			const frame = parseFrameEvent(event as MessageEvent);
			if (frame) {
				enqueueFrame(frame);
			}
		});

		eventSource.addEventListener('end', (event) => {
			const reason = parseEndEvent(event as MessageEvent);
			if (session) {
				session = { ...session, active: false };
			}
			infoMessage = reason ?? 'Remote desktop session ended.';
			disconnectStream();
		});

		eventSource.onerror = () => {
			if (!sessionActive) {
				disconnectStream();
			}
		};
	}

	function parseSessionEvent(event: MessageEvent): RemoteDesktopSessionState | null {
		try {
			const data = JSON.parse(event.data) as { session?: RemoteDesktopSessionState };
			return data?.session ?? null;
		} catch (err) {
			console.error('Failed to parse session event', err);
			return null;
		}
	}

	function parseFrameEvent(event: MessageEvent): RemoteDesktopFramePacket | null {
		try {
			const data = JSON.parse(event.data) as { frame?: RemoteDesktopFramePacket };
			return data?.frame ?? null;
		} catch (err) {
			console.error('Failed to parse frame event', err);
			return null;
		}
	}

	function parseEndEvent(event: MessageEvent): string | null {
		try {
			const data = JSON.parse(event.data) as { reason?: string };
			return data?.reason ?? null;
		} catch {
			return null;
		}
	}

	function ensureContext(): CanvasRenderingContext2D | null {
		if (!canvasEl) {
			return null;
		}
		if (!canvasContext) {
			canvasContext = canvasEl.getContext('2d');
		}
		return canvasContext;
	}

	function enqueueFrame(frame: RemoteDesktopFramePacket) {
		if (frame.keyFrame) {
			frameQueue = [];
		}
		frameQueue.push(frame);

		if (frameQueue.length > MAX_FRAME_QUEUE) {
			let removed = 0;
			while (frameQueue.length > MAX_FRAME_QUEUE) {
				if (frameQueue[0]?.keyFrame && frameQueue.length > 1) {
					frameQueue.splice(1, 1);
				} else {
					frameQueue.shift();
				}
				removed += 1;
			}
			if (removed > 0) {
				droppedFrames += removed;
			}
		}

		if (!processing) {
			void processQueue();
		}
	}

	async function processQueue() {
		if (processing) return;
		processing = true;
		try {
			while (frameQueue.length > 0) {
				if (stopRequested) {
					frameQueue = [];
					break;
				}
				const next = frameQueue.shift();
				if (!next) {
					break;
				}
				try {
					await applyFrame(next);
					if (next.metrics) {
						const metrics = next.metrics;
						fps = typeof metrics.fps === 'number' ? metrics.fps : fps;
						bandwidth =
							typeof metrics.bandwidthKbps === 'number' ? metrics.bandwidthKbps : bandwidth;
						cpu = typeof metrics.cpuPercent === 'number' ? metrics.cpuPercent : cpu;
						gpu = typeof metrics.gpuPercent === 'number' ? metrics.gpuPercent : gpu;
						clipQuality =
							typeof metrics.clipQuality === 'number' ? metrics.clipQuality : clipQuality;
					}
					streamWidth = typeof next.width === 'number' ? next.width : streamWidth;
					streamHeight = typeof next.height === 'number' ? next.height : streamHeight;
					latencyMs = computeLatency(next.timestamp);
					if (next.monitors && next.monitors.length > 0) {
						monitors = next.monitors;
					}
					if (session) {
						session = {
							...session,
							lastSequence: next.sequence,
							lastUpdatedAt: next.timestamp,
							metrics: next.metrics ?? session.metrics,
							monitors: next.monitors && next.monitors.length > 0 ? next.monitors : session.monitors
						};
					}
				} catch (err) {
					console.error('Failed to apply frame', err);
					errorMessage = err instanceof Error ? err.message : 'Failed to render frame';
				}
			}
		} finally {
			processing = false;
		}
	}

	async function applyFrame(frame: RemoteDesktopFramePacket) {
		const context = ensureContext();
		if (!canvasEl || !context) {
			return;
		}

		if (canvasEl.width !== frame.width || canvasEl.height !== frame.height) {
			canvasEl.width = frame.width;
			canvasEl.height = frame.height;
		}

		if (frame.encoding === 'clip') {
			await applyClipFrame(context, frame);
			return;
		}

                if (frame.keyFrame) {
                        if (!frame.image) {
                                throw new Error('Missing key frame image data');
                        }
                        const mime = frame.encoding === 'jpeg' ? 'image/jpeg' : 'image/png';
                        if (supportsImageBitmap) {
                                try {
                                        const bitmap = await decodeBitmap(frame.image, mime);
                                        try {
                                                context.drawImage(bitmap, 0, 0, frame.width, frame.height);
                                        } finally {
                                                bitmap.close();
                                        }
                                        return;
                                } catch (err) {
                                        logBitmapFallback(err);
                                }
                        }
                        await drawWithImageElement(
                                context,
                                frame.image,
                                0,
                                0,
                                frame.width,
                                frame.height,
                                frame.encoding === 'jpeg' ? 'jpeg' : 'png'
                        );
                        return;
                }

		if (frame.deltas && frame.deltas.length > 0) {
			if (supportsImageBitmap) {
				try {
					const bitmaps = await Promise.all(
						frame.deltas.map(async (rect) => ({
							rect,
							bitmap: await decodeBitmap(
								rect.data,
								rect.encoding === 'jpeg' ? 'image/jpeg' : 'image/png'
							)
						}))
					);
					try {
						for (const { rect, bitmap } of bitmaps) {
							context.drawImage(bitmap, rect.x, rect.y, rect.width, rect.height);
						}
					} finally {
						for (const { bitmap } of bitmaps) {
							bitmap.close();
						}
					}
					return;
				} catch (err) {
					logBitmapFallback(err);
				}
			}

			for (const rect of frame.deltas) {
				await drawWithImageElement(
					context,
					rect.data,
					rect.x,
					rect.y,
					rect.width,
					rect.height,
					rect.encoding === 'jpeg' ? 'jpeg' : 'png'
				);
			}
		}
	}

	async function applyClipFrame(
		context: CanvasRenderingContext2D,
		frame: RemoteDesktopFramePacket
	) {
		const clip = frame.clip;
		if (!clip || !clip.frames || clip.frames.length === 0) {
			throw new Error('Missing clip frame payload');
		}

		const start = performance.now();
		for (const segment of clip.frames) {
			const target = Math.max(0, segment.offsetMs);
			const elapsed = performance.now() - start;
			const delay = target - elapsed;
			if (delay > 1) {
				await new Promise<void>((resolve) => setTimeout(resolve, delay));
			}

			const mime = segment.encoding === 'jpeg' ? 'image/jpeg' : 'image/png';
			if (supportsImageBitmap) {
				try {
					const bitmap = await decodeBitmap(segment.data, mime);
					try {
						context.drawImage(bitmap, 0, 0, frame.width, frame.height);
					} finally {
						bitmap.close();
					}
					continue;
				} catch (err) {
					logBitmapFallback(err);
				}
			}

			await drawWithImageElement(
				context,
				segment.data,
				0,
				0,
				frame.width,
				frame.height,
				segment.encoding === 'jpeg' ? 'jpeg' : 'png'
			);
		}
	}

	async function decodeBitmap(
		data: string,
		mimeType: 'image/png' | 'image/jpeg'
	): Promise<ImageBitmap> {
		const binary = atob(data);
		const length = binary.length;
		const bytes = new Uint8Array(length);
		for (let i = 0; i < length; i += 1) {
			bytes[i] = binary.charCodeAt(i);
		}
		const blob = new Blob([bytes], { type: mimeType });
		return await createImageBitmap(blob);
	}

	function drawWithImageElement(
		context: CanvasRenderingContext2D,
		data: string,
		x: number,
		y: number,
		width: number,
		height: number,
		encoding: 'png' | 'jpeg'
	): Promise<void> {
		return new Promise((resolve, reject) => {
			const image = new Image();
			image.decoding = 'async';
			image.onload = () => {
				try {
					context.drawImage(image, x, y, width, height);
					resolve();
				} catch (err) {
					reject(err);
				}
			};
			image.onerror = () => reject(new Error('Failed to decode frame image segment'));
			const prefix = encoding === 'jpeg' ? IMAGE_BASE64_PREFIX.jpeg : IMAGE_BASE64_PREFIX.png;
			image.src = `${prefix}${data}`;
		});
	}

	function logBitmapFallback(err: unknown) {
		if (imageBitmapFallbackLogged) {
			return;
		}
		imageBitmapFallbackLogged = true;
		console.warn('ImageBitmap decode failed, falling back to <img> rendering', err);
	}

	function computeLatency(timestamp?: string | null) {
		if (!timestamp) {
			return null;
		}
		const parsed = Date.parse(timestamp);
		if (Number.isNaN(parsed)) {
			return null;
		}
		const delta = Date.now() - parsed;
		return delta < 0 ? 0 : delta;
	}

	async function startSession() {
		if (!client || isStarting) return;
		errorMessage = null;
		infoMessage = null;
		isStarting = true;
		try {
			const payload = {
				quality,
				monitor,
				mode,
				mouse: mouseEnabled,
				keyboard: keyboardEnabled
			} satisfies Partial<RemoteDesktopSettings> & { mouse: boolean; keyboard: boolean };
			const response = await fetch(`/api/agents/${client.id}/remote-desktop/session`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			if (!response.ok) {
				const message = (await response.text()) || 'Unable to start remote desktop session';
				throw new Error(message);
			}
			const data = (await response.json()) as { session: RemoteDesktopSessionState | null };
			session = data.session ?? null;
			resetMetrics();
			infoMessage = 'Remote desktop session started.';
			if (session?.sessionId) {
				connectStream(session.sessionId);
			}
		} catch (err) {
			errorMessage = err instanceof Error ? err.message : 'Failed to start remote desktop session';
		} finally {
			isStarting = false;
		}
	}

	async function stopSession() {
		if (!client || isStopping || !session?.sessionId) return;
		errorMessage = null;
		infoMessage = null;
		isStopping = true;
		try {
			const response = await fetch(`/api/agents/${client.id}/remote-desktop/session`, {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ sessionId: session.sessionId })
			});
			if (!response.ok) {
				const message = (await response.text()) || 'Unable to stop remote desktop session';
				throw new Error(message);
			}
			const data = (await response.json()) as { session: RemoteDesktopSessionState | null };
			session = data.session ?? session;
			infoMessage = 'Remote desktop session stopped.';
			disconnectStream();
		} catch (err) {
			errorMessage = err instanceof Error ? err.message : 'Failed to stop remote desktop session';
		} finally {
			isStopping = false;
		}
	}

	async function updateSession(partial: Partial<RemoteDesktopSettings>) {
		if (!client || !session?.sessionId) return;
		if (Object.keys(partial).length === 0) {
			return;
		}
		isUpdating = true;
		try {
			const response = await fetch(`/api/agents/${client.id}/remote-desktop/session`, {
				method: 'PATCH',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ sessionId: session.sessionId, ...partial })
			});
			if (!response.ok) {
				const message = (await response.text()) || 'Failed to update session settings';
				throw new Error(message);
			}
			const data = (await response.json()) as { session: RemoteDesktopSessionState | null };
			session = data.session ?? session;
		} catch (err) {
			errorMessage =
				err instanceof Error ? err.message : 'Failed to update remote desktop settings';
		} finally {
			isUpdating = false;
		}
	}

	function formatMetric(value: number | null, suffix: string, digits = 1) {
		if (value === null || Number.isNaN(value)) {
			return `-- ${suffix}`;
		}
		return `${value.toFixed(digits)} ${suffix}`;
	}

	function formatPercent(value: number | null) {
		if (value === null || Number.isNaN(value)) {
			return '-- %';
		}
		return `${Math.round(value)}%`;
	}

	function formatResolution(width: number | null, height: number | null) {
		if (width === null || height === null || Number.isNaN(width) || Number.isNaN(height)) {
			return '--';
		}
		return `${width}×${height}`;
	}

	function formatLatency(value: number | null) {
		if (value === null || Number.isNaN(value)) {
			return '-- ms';
		}
		if (value >= 1000) {
			return `${(value / 1000).toFixed(1)} s`;
		}
		return `${Math.round(value)} ms`;
	}

        function formatTimestamp(value: string | null | undefined) {
                if (!value) return '—';
                const parsed = new Date(value);
                if (Number.isNaN(parsed.getTime())) {
                        return value;
                }
                return parsed.toLocaleTimeString();
        }

        function queueInput(event: RemoteDesktopInputEvent) {
                if (!browser || !sessionActive || !sessionId || !client) {
                        return;
                }
                inputQueue.push(event);
                scheduleInputFlush();
        }

        function queueInputBatch(events: RemoteDesktopInputEvent[]) {
                if (!browser || !sessionActive || !sessionId || !client) {
                        return;
                }
                if (events.length === 0) {
                        return;
                }
                inputQueue.push(...events);
                scheduleInputFlush();
        }

        function scheduleInputFlush() {
                if (!browser || inputQueue.length === 0) {
                        return;
                }
                if (inputFlushHandle !== null) {
                        return;
                }
                inputFlushHandle = requestAnimationFrame(() => {
                        inputFlushHandle = null;
                        void flushInputQueue();
                });
        }

        async function flushInputQueue() {
                if (!browser || !client || !sessionId || !sessionActive) {
                        inputSending = false;
                        return;
                }
                if (inputSending || inputQueue.length === 0) {
                        return;
                }
                inputSending = true;
                const events = inputQueue.splice(0, inputQueue.length);
                try {
                        const response = await fetch(`/api/agents/${client.id}/remote-desktop/input`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({ sessionId, events }),
                                keepalive: true
                        });
                        if (!response.ok) {
                                inputQueue = [...events, ...inputQueue];
                                const message = await response.text();
                                console.warn('Remote desktop input dispatch failed', message);
                        }
                } catch (err) {
                        inputQueue = [...events, ...inputQueue];
                        console.error('Failed to send remote desktop input events', err);
                } finally {
                        inputSending = false;
                        if (inputQueue.length > 0) {
                                scheduleInputFlush();
                        }
                }
        }

        function clearInputQueue() {
                inputQueue = [];
                if (browser && inputFlushHandle !== null) {
                        cancelAnimationFrame(inputFlushHandle);
                        inputFlushHandle = null;
                }
        }

        function releasePointerCapture() {
                if (!viewportEl) {
                        pointerCaptured = false;
                        activePointerId = null;
                        return;
                }
                if (!pointerCaptured || activePointerId === null) {
                        pointerCaptured = false;
                        activePointerId = null;
                        return;
                }
                try {
                        viewportEl.releasePointerCapture(activePointerId);
                } catch {
                        // ignore
                }
                pointerCaptured = false;
                activePointerId = null;
        }

        function pointerButtonFromEvent(button: number): RemoteDesktopMouseButton | null {
                if (button === 0) return 'left';
                if (button === 1) return 'middle';
                if (button === 2) return 'right';
                const mapped = pointerButtonMap[button];
                return mapped ?? null;
        }

        function handlePointerMove(event: PointerEvent) {
                if (!browser || event.pointerType !== 'mouse') {
                        return;
                }
                if (!mouseEnabled || !sessionActive) {
                        return;
                }
                if (!canvasEl) {
                        return;
                }
                const rect = canvasEl.getBoundingClientRect();
                if (rect.width <= 0 || rect.height <= 0) {
                        return;
                }
                const x = clamp((event.clientX - rect.left) / rect.width, 0, 1);
                const y = clamp((event.clientY - rect.top) / rect.height, 0, 1);
                queueInput({ type: 'mouse-move', x, y, normalized: true, monitor });
        }

        function handlePointerDown(event: PointerEvent) {
                if (!browser || event.pointerType !== 'mouse') {
                        return;
                }
                if (!mouseEnabled || !sessionActive) {
                        return;
                }
                event.preventDefault();
                viewportEl?.focus();
                handlePointerMove(event);
                const button = pointerButtonFromEvent(event.button);
                if (button) {
                        queueInput({ type: 'mouse-button', button, pressed: true, monitor });
                }
                const target = event.currentTarget as HTMLDivElement | null;
                if (target) {
                        try {
                                target.setPointerCapture(event.pointerId);
                                pointerCaptured = true;
                                activePointerId = event.pointerId;
                        } catch {
                                pointerCaptured = false;
                                activePointerId = null;
                        }
                }
        }

        function handlePointerUp(event: PointerEvent) {
                if (!browser || event.pointerType !== 'mouse') {
                        return;
                }
                if (!mouseEnabled || !sessionActive) {
                        releasePointerCapture();
                        return;
                }
                event.preventDefault();
                const button = pointerButtonFromEvent(event.button);
                if (button) {
                        queueInput({ type: 'mouse-button', button, pressed: false, monitor });
                }
                if (pointerCaptured && activePointerId === event.pointerId) {
                        releasePointerCapture();
                }
        }

        function handlePointerLeave() {
                if (!pointerCaptured) {
                        return;
                }
                releasePointerCapture();
        }

        function handleWheel(event: WheelEvent) {
                if (!mouseEnabled || !sessionActive) {
                        return;
                }
                queueInput({
                        type: 'mouse-scroll',
                        deltaX: event.deltaX,
                        deltaY: event.deltaY,
                        deltaMode: event.deltaMode,
                        monitor
                });
        }

        function handleViewportFocus() {
                viewportFocused = true;
        }

        function handleViewportBlur() {
                viewportFocused = false;
                releasePointerCapture();
                releaseAllPressedKeys();
        }

        function keyCodeFromEvent(event: KeyboardEvent) {
                const raw = (event as KeyboardEvent & { which?: number }).keyCode ?? event.which;
                if (typeof raw !== 'number' || Number.isNaN(raw)) {
                        return 0;
                }
                return Math.trunc(raw);
        }

        function createKeyEvent(
                pressed: boolean,
                keyCode: number,
                event: KeyboardEvent,
                meta?: { key?: string; code?: string }
        ): RemoteDesktopInputEvent {
                return {
                        type: 'key',
                        pressed,
                        keyCode,
                        key: event.key ?? meta?.key,
                        code: event.code ?? meta?.code,
                        repeat: pressed ? event.repeat : false,
                        altKey: event.altKey,
                        ctrlKey: event.ctrlKey,
                        shiftKey: event.shiftKey,
                        metaKey: event.metaKey
                };
        }

        function handleKeyDown(event: KeyboardEvent) {
                if (!keyboardEnabled || !sessionActive || !viewportFocused) {
                        return;
                }
                const keyCode = keyCodeFromEvent(event);
                if (keyCode <= 0) {
                        return;
                }
                event.preventDefault();
                if (!event.repeat && !pressedKeys.has(keyCode)) {
                        pressedKeys.add(keyCode);
                        pressedKeyMeta.set(keyCode, { key: event.key, code: event.code });
                }
                const meta = pressedKeyMeta.get(keyCode);
                queueInput(createKeyEvent(true, keyCode, event, meta));
        }

        function handleKeyUp(event: KeyboardEvent) {
                const keyCode = keyCodeFromEvent(event);
                if (keyCode <= 0) {
                        return;
                }
                const meta = pressedKeyMeta.get(keyCode);
                pressedKeys.delete(keyCode);
                pressedKeyMeta.delete(keyCode);
                if (!keyboardEnabled || !sessionActive) {
                        return;
                }
                event.preventDefault();
                queueInput(createKeyEvent(false, keyCode, event, meta));
        }

        function releaseAllPressedKeys() {
                if (pressedKeys.size === 0) {
                        pressedKeyMeta.clear();
                        return;
                }
                const events: RemoteDesktopInputEvent[] = [];
                for (const code of pressedKeys) {
                        const meta = pressedKeyMeta.get(code);
                        events.push({
                                type: 'key',
                                pressed: false,
                                keyCode: code,
                                key: meta?.key,
                                code: meta?.code,
                                altKey: false,
                                ctrlKey: false,
                                shiftKey: false,
                                metaKey: false
                        });
                }
                pressedKeys.clear();
                pressedKeyMeta.clear();
                queueInputBatch(events);
        }

        $effect(() => {
                mouseEnabled;
                if (!mouseEnabled) {
                        releasePointerCapture();
                }
        });

        $effect(() => {
                keyboardEnabled;
                if (!keyboardEnabled) {
                        releaseAllPressedKeys();
                }
        });

        $effect(() => {
                sessionActive;
                if (!sessionActive) {
                        releasePointerCapture();
                        releaseAllPressedKeys();
                        clearInputQueue();
                }
        });

        $effect(() => {
                const current = session;
                if (!current) {
                        quality = 'auto';
                        mode = 'video';
                        monitor = 0;
			mouseEnabled = true;
			keyboardEnabled = true;
			sessionActive = false;
			sessionId = '';
			monitors = fallbackMonitors;
			resetMetrics();
			return;
		}
		quality = current.settings.quality;
		mode = current.settings.mode;
		monitor = current.settings.monitor;
		mouseEnabled = current.settings.mouse;
		keyboardEnabled = current.settings.keyboard;
		sessionActive = current.active === true;
		sessionId = current.sessionId ?? '';
		monitors =
			current.monitors && current.monitors.length > 0 ? current.monitors : fallbackMonitors;
		if (current.metrics) {
			fps = typeof current.metrics.fps === 'number' ? current.metrics.fps : fps;
			gpu = typeof current.metrics.gpuPercent === 'number' ? current.metrics.gpuPercent : gpu;
			cpu = typeof current.metrics.cpuPercent === 'number' ? current.metrics.cpuPercent : cpu;
			bandwidth =
				typeof current.metrics.bandwidthKbps === 'number'
					? current.metrics.bandwidthKbps
					: bandwidth;
			clipQuality =
				typeof current.metrics.clipQuality === 'number' ? current.metrics.clipQuality : clipQuality;
		}
	});

	$effect(() => {
		if (!sessionActive) {
			skipMouseSync = true;
			skipKeyboardSync = true;
		}
	});

	$effect(() => {
		if (!sessionActive) {
			return;
		}
		const current = session;
		mouseEnabled;
		if (!current) {
			return;
		}
		if (current.settings.mouse === mouseEnabled) {
			skipMouseSync = false;
			return;
		}
		if (skipMouseSync) {
			skipMouseSync = false;
			return;
		}
		void updateSession({ mouse: mouseEnabled });
	});

	$effect(() => {
		if (!sessionActive) {
			return;
		}
		const current = session;
		keyboardEnabled;
		if (!current) {
			return;
		}
		if (current.settings.keyboard === keyboardEnabled) {
			skipKeyboardSync = false;
			return;
		}
		if (skipKeyboardSync) {
			skipKeyboardSync = false;
			return;
		}
		void updateSession({ keyboard: keyboardEnabled });
	});

	$effect(() => {
		if (!sessionActive || !sessionId) {
			disconnectStream();
			return;
		}
		connectStream(sessionId);
	});

	onMount(() => {
		if (browser && sessionActive && sessionId) {
			connectStream(sessionId);
		}
		return () => {
			disconnectStream();
		};
	});
</script>

<svelte:window on:keydown={handleKeyDown} on:keyup={handleKeyUp} />

<Tabs bind:value={activeTab} class="space-y-6">
	<TabsList class="w-full max-w-md">
		<TabsTrigger value="stream">Stream</TabsTrigger>
		<TabsTrigger value="controls">Controls</TabsTrigger>
	</TabsList>

	<TabsContent value="stream" class="space-y-6">
		<Card>
			<CardHeader class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
				<div class="space-y-1">
					<CardTitle class="text-base">Remote desktop session</CardTitle>
					<CardDescription>
						Monitor the active screen stream for {client.codename}. Start a session to receive
						frames.
					</CardDescription>
				</div>
			</CardHeader>
			<CardContent>
				<div class="flex items-center gap-2">
					<Badge variant={sessionActive ? 'default' : 'outline'}>
						{sessionActive ? 'Active' : 'Inactive'}
					</Badge>
					{#if sessionId}
						<span class="text-xs text-muted-foreground">Session ID: {sessionId}</span>
					{/if}
				</div>
				<div
					tabindex="-1"
					bind:this={viewportEl}
					class="relative overflow-hidden rounded-lg border border-border bg-muted/30 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
					role="application"
					aria-label="Remote desktop viewport"
					onfocus={handleViewportFocus}
					onblur={handleViewportBlur}
					onpointerdown={handlePointerDown}
					onpointerup={handlePointerUp}
					onpointermove={handlePointerMove}
					onpointerleave={handlePointerLeave}
					onpointercancel={handlePointerLeave}
					onwheel={handleWheel}
					style="touch-action: none;"
				>
						<canvas bind:this={canvasEl} class="block h-full w-full bg-slate-950"></canvas>
						{#if !sessionActive}
								<div
										class="absolute inset-0 flex items-center justify-center text-sm text-muted-foreground"
								>
										Session inactive · start streaming to receive frames
								</div>
						{/if}
				</div>
				<div class="grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
					<div class="rounded-lg border border-border/60 bg-background/60 p-3">
						<p class="text-xs text-muted-foreground uppercase">FPS</p>
						<p class="text-sm font-semibold text-foreground">{formatMetric(fps, 'fps')}</p>
					</div>
					<div class="rounded-lg border border-border/60 bg-background/60 p-3">
						<p class="text-xs text-muted-foreground uppercase">GPU</p>
						<p class="text-sm font-semibold text-foreground">{formatPercent(gpu)}</p>
					</div>
					<div class="rounded-lg border border-border/60 bg-background/60 p-3">
						<p class="text-xs text-muted-foreground uppercase">CPU</p>
						<p class="text-sm font-semibold text-foreground">{formatPercent(cpu)}</p>
					</div>
					<div class="rounded-lg border border-border/60 bg-background/60 p-3">
						<p class="text-xs text-muted-foreground uppercase">Bandwidth</p>
						<p class="text-sm font-semibold text-foreground">{formatMetric(bandwidth, 'kbps')}</p>
					</div>
				</div>
				{#if errorMessage}
					<p class="text-sm text-destructive">{errorMessage}</p>
				{/if}
				{#if infoMessage}
					<p class="text-sm text-emerald-500">{infoMessage}</p>
				{/if}
			</CardContent>
			<CardFooter
				class="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground"
			>
				<span>Last frame: {formatTimestamp(session?.lastUpdatedAt)}</span>
				<div class="flex gap-2">
					{#if sessionActive}
						<Button variant="destructive" disabled={isStopping} onclick={stopSession}>
							{isStopping ? 'Stopping…' : 'Stop session'}
						</Button>
					{:else}
						<Button disabled={isStarting} onclick={startSession}>
							{isStarting ? 'Starting…' : 'Start session'}
						</Button>
					{/if}
				</div>
			</CardFooter>
		</Card>
	</TabsContent>

	<TabsContent value="controls">
		<Card>
			<CardHeader>
				<CardTitle class="text-base">Session controls</CardTitle>
				<CardDescription>
					Adjust encoder preferences and input sharing while the session is active.
				</CardDescription>
			</CardHeader>
			<CardContent class="space-y-6">
				<div class="grid gap-4 md:grid-cols-3">
					<div class="space-y-2">
						<Label class="text-sm font-medium" for="quality-select">Quality</Label>
						<Select
							type="single"
							value={quality}
							onValueChange={(value) => {
								quality = value as RemoteDesktopSettings['quality'];
								if (sessionActive) {
									void updateSession({ quality });
								}
							}}
						>
							<SelectTrigger
								id="quality-select"
								class="w-full"
								disabled={isUpdating && sessionActive}
							>
								<span class="truncate">{qualityLabel(quality)}</span>
							</SelectTrigger>
							<SelectContent>
								{#each qualityOptions as option (option.value)}
									<SelectItem value={option.value}>{option.label}</SelectItem>
								{/each}
							</SelectContent>
						</Select>
						<p class="text-xs text-muted-foreground">
							Auto balances fidelity and responsiveness based on observed frame pacing.
						</p>
					</div>
					<div class="space-y-2">
						<Label class="text-sm font-medium" for="mode-select">Stream mode</Label>
						<Select
							type="single"
							value={mode}
							onValueChange={(value) => {
								mode = value as RemoteDesktopSettings['mode'];
								if (sessionActive) {
									void updateSession({ mode });
								}
							}}
						>
							<SelectTrigger id="mode-select" class="w-full" disabled={isUpdating && sessionActive}>
								<span class="truncate">{modeLabel(mode)}</span>
							</SelectTrigger>
							<SelectContent>
								{#each modeOptions as option (option.value)}
									<SelectItem value={option.value}>{option.label}</SelectItem>
								{/each}
							</SelectContent>
						</Select>
						<p class="text-xs text-muted-foreground">
							Video clips bundle 100–300 ms of motion to improve smoothness.
						</p>
					</div>
					<div class="space-y-2">
						<Label class="text-sm font-medium" for="monitor-select">Monitor</Label>
						<Select
							type="single"
							value={monitor.toString()}
							onValueChange={(value) => {
								const next = Number.parseInt(value, 10);
								monitor = Number.isNaN(next) ? 0 : next;
								if (sessionActive) {
									void updateSession({ monitor });
								}
							}}
						>
							<SelectTrigger
								id="monitor-select"
								class="w-full"
								disabled={isUpdating && sessionActive}
							>
								<span class="truncate">{monitorLabel(monitor)}</span>
							</SelectTrigger>
							<SelectContent>
								{#each monitors as item (item.id)}
									<SelectItem value={item.id.toString()}>
										Monitor {item.id + 1} · {item.width}×{item.height}
									</SelectItem>
								{/each}
							</SelectContent>
						</Select>
						<p class="text-xs text-muted-foreground">
							Choose which display surface to capture when streaming.
						</p>
					</div>
				</div>
				<Separator />
				<div class="grid gap-4 md:grid-cols-2">
					<div
						class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/40 p-4"
					>
						<div>
							<p class="text-sm font-medium">Mouse control</p>
							<p class="text-xs text-muted-foreground">
								Allow pointer events to be relayed to the remote system.
							</p>
						</div>
						<Switch
							bind:checked={mouseEnabled}
							disabled={!sessionActive || isUpdating}
							aria-label="Toggle mouse control"
						/>
					</div>
					<div
						class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/40 p-4"
					>
						<div>
							<p class="text-sm font-medium">Keyboard control</p>
							<p class="text-xs text-muted-foreground">
								Forward keyboard input when focus is inside the session viewport.
							</p>
						</div>
						<Switch
							bind:checked={keyboardEnabled}
							disabled={!sessionActive || isUpdating}
							aria-label="Toggle keyboard control"
						/>
					</div>
				</div>
				<Separator />
				<div class="grid gap-3 text-sm">
					<p class="text-xs text-muted-foreground uppercase">Current metrics</p>
					<div class="grid gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-6">
						<div class="rounded-lg border border-border/60 bg-background/60 p-3">
							<p class="text-xs text-muted-foreground">Frame rate</p>
							<p class="text-sm font-semibold text-foreground">{formatMetric(fps, 'fps')}</p>
						</div>
						<div class="rounded-lg border border-border/60 bg-background/60 p-3">
							<p class="text-xs text-muted-foreground">GPU usage</p>
							<p class="text-sm font-semibold text-foreground">{formatPercent(gpu)}</p>
						</div>
						<div class="rounded-lg border border-border/60 bg-background/60 p-3">
							<p class="text-xs text-muted-foreground">CPU usage</p>
							<p class="text-sm font-semibold text-foreground">{formatPercent(cpu)}</p>
						</div>
						<div class="rounded-lg border border-border/60 bg-background/60 p-3">
							<p class="text-xs text-muted-foreground">Bandwidth</p>
							<p class="text-sm font-semibold text-foreground">{formatMetric(bandwidth, 'kbps')}</p>
						</div>
						<div class="rounded-lg border border-border/60 bg-background/60 p-3">
							<p class="text-xs text-muted-foreground">JPEG quality</p>
							<p class="text-sm font-semibold text-foreground">
								{clipQuality === null || Number.isNaN(clipQuality)
									? '--'
									: `Q${Math.round(clipQuality)}`}
							</p>
						</div>
						<div class="rounded-lg border border-border/60 bg-background/60 p-3">
							<p class="text-xs text-muted-foreground">Resolution</p>
							<p class="text-sm font-semibold text-foreground">
								{formatResolution(streamWidth, streamHeight)}
							</p>
						</div>
						<div class="rounded-lg border border-border/60 bg-background/60 p-3">
							<p class="text-xs text-muted-foreground">Latency</p>
							<p class="text-sm font-semibold text-foreground">{formatLatency(latencyMs)}</p>
						</div>
					</div>
					{#if droppedFrames > 0}
						<p class="text-xs text-muted-foreground">
							Dropped {droppedFrames} frame{droppedFrames === 1 ? '' : 's'} to keep playback responsive.
						</p>
					{/if}
				</div>
			</CardContent>
		</Card>
	</TabsContent>
</Tabs>
