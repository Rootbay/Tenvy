<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import { notifyToolActivationCommand } from '$lib/utils/agent-commands.js';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';

	type WebcamDevice = {
		id: string;
		label: string;
	};

	type StillCapture = {
		id: string;
		url: string;
		width: number;
		height: number;
		timestamp: string;
		cameraLabel: string;
	};

	type RecordingClip = {
		id: string;
		url: string;
		mimeType: string;
		size: number;
		durationMs: number;
		createdAt: string;
		cameraLabel: string;
		resolution: string;
		frameRate: number | null;
	};

const { client } = $props<{ client: Client }>();
void client;

const tool = getClientTool('webcam-control');
void tool;

	const RESOLUTION_OPTIONS = [
		{ value: '3840×2160', label: '3840×2160 · 4K' },
		{ value: '1920×1080', label: '1920×1080 · 1080p' },
		{ value: '1280×720', label: '1280×720 · 720p' },
		{ value: '640×480', label: '640×480 · VGA' }
	] as const;

	let devices = $state<WebcamDevice[]>([]);
	let selectedCamera = $state('');
	let resolution = $state<(typeof RESOLUTION_OPTIONS)[number]['value']>('1280×720');
	let frameRate = $state(30);
	let zoom = $state(1);
	let zoomSupported = $state(false);
	let zoomMin = $state(1);
	let zoomMax = $state(1);
	let zoomStep = $state(0.1);
	let previewActive = $state(false);
	let initializing = $state(false);
	let mediaSupported = $state(false);
	let errorMessage = $state<string | null>(null);
	let log = $state<WorkspaceLogEntry[]>([]);
	let captures = $state<StillCapture[]>([]);
	let recordings = $state<RecordingClip[]>([]);
	let recordingActive = $state(false);
	let recordingSeconds = $state(0);

	let videoElement: HTMLVideoElement | null = null;
	let stream: MediaStream | null = null;
	let mediaRecorder: MediaRecorder | null = null;
	let recordedChunks: Blob[] = [];
	let recordingTimer: ReturnType<typeof setInterval> | null = null;
	let recordingStartedAt = 0;
	let discardRecording = false;

const objectUrls = new SvelteSet<string>();

	function generateId(): string {
		return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
	}

	function parseResolution(value: string): { width: number; height: number } {
		const [rawWidth, rawHeight] = value.split(/[×x]/u);
		const width = Number.parseInt(rawWidth ?? '', 10);
		const height = Number.parseInt(rawHeight ?? '', 10);
		if (!Number.isFinite(width) || width <= 0 || !Number.isFinite(height) || height <= 0) {
			return { width: 1280, height: 720 };
		}
		return { width, height };
	}

	function clamp(value: number, min: number, max: number): number {
		if (!Number.isFinite(value)) {
			return min;
		}
		if (max < min) {
			[min, max] = [max, min];
		}
		if (value < min) {
			return min;
		}
		if (value > max) {
			return max;
		}
		return value;
	}

function cameraLabel(): string {
	if (selectedCamera === '') {
		return devices.length > 0 ? 'Auto select' : 'No camera';
	}
	const match = devices.find((device) => device.id === selectedCamera);
	return match?.label ?? 'Selected device';
}

function describeTrack(track: MediaStreamTrack | null): string {
		const label = cameraLabel();
		const { width: fallbackWidth, height: fallbackHeight } = parseResolution(resolution);
		if (!track || typeof track.getSettings !== 'function') {
			return `${label} · ${fallbackWidth}×${fallbackHeight} @ ${frameRate}fps`;
		}
		const settings = track.getSettings();
		const width =
			typeof settings.width === 'number' && !Number.isNaN(settings.width)
				? Math.round(settings.width)
				: fallbackWidth;
		const height =
			typeof settings.height === 'number' && !Number.isNaN(settings.height)
				? Math.round(settings.height)
				: fallbackHeight;
		const fps =
			typeof settings.frameRate === 'number' && !Number.isNaN(settings.frameRate)
				? Math.round(settings.frameRate)
				: frameRate;
		return `${label} · ${width}×${height} @ ${fps}fps`;
	}

	function formatResolutionLabel(settings: MediaTrackSettings | null): string {
		const { width: fallbackWidth, height: fallbackHeight } = parseResolution(resolution);
		if (!settings) {
			return `${fallbackWidth}×${fallbackHeight}`;
		}
		const width =
			typeof settings.width === 'number' && !Number.isNaN(settings.width)
				? Math.round(settings.width)
				: fallbackWidth;
		const height =
			typeof settings.height === 'number' && !Number.isNaN(settings.height)
				? Math.round(settings.height)
				: fallbackHeight;
		return `${width}×${height}`;
	}

	function logAction(
		action: string,
		detail: string,
		status: WorkspaceLogEntry['status'] = 'complete',
		metadata?: Record<string, unknown>
	) {
		log = appendWorkspaceLog(log, createWorkspaceLogEntry(action, detail, status));
		notifyToolActivationCommand(client.id, 'webcam-control', {
			action: `event:${action}`,
			metadata: {
				detail,
				status,
				...metadata
			}
		});
	}

	function buildConstraints(): MediaStreamConstraints {
		const { width, height } = parseResolution(resolution);
		const video: MediaTrackConstraints = {
			width: { ideal: width },
			height: { ideal: height },
			frameRate: { ideal: frameRate },
			facingMode: 'user'
		};
		if (selectedCamera) {
			video.deviceId = { exact: selectedCamera };
		}
		return { video, audio: false } satisfies MediaStreamConstraints;
	}

	function resetZoomCapabilities() {
		zoomSupported = false;
		zoom = 1;
		zoomMin = 1;
		zoomMax = 1;
		zoomStep = 0.1;
	}

	async function configureZoom(track: MediaStreamTrack | null) {
		if (
			!track ||
			typeof track.getCapabilities !== 'function' ||
			typeof track.getSettings !== 'function'
		) {
			resetZoomCapabilities();
			return;
		}

		try {
			const capabilities = track.getCapabilities() as MediaTrackCapabilities & {
				zoom?: { min?: number; max?: number; step?: number };
			};
			if (!capabilities || typeof capabilities.zoom !== 'object') {
				resetZoomCapabilities();
				return;
			}
			const { min, max, step } = capabilities.zoom;
			const rangeMin = Number.isFinite(min) && min !== undefined ? min : 1;
			const rangeMax = Number.isFinite(max) && max !== undefined ? max : Math.max(rangeMin, 1);
			const rangeStep = Number.isFinite(step) && step && step > 0 ? step : 0.1;
			zoomMin = rangeMin;
			zoomMax = rangeMax;
			zoomStep = rangeStep;

			const settings = track.getSettings();
			const current =
				typeof settings.zoom === 'number' && !Number.isNaN(settings.zoom)
					? clamp(settings.zoom, rangeMin, rangeMax)
					: clamp(zoom, rangeMin, rangeMax);
			zoom = current;
			zoomSupported = true;
		} catch {
			resetZoomCapabilities();
		}
	}

	async function applyZoom(value: number) {
		const track = stream?.getVideoTracks()[0] ?? null;
		if (!track || typeof track.applyConstraints !== 'function') {
			throw new Error('Zoom is not supported by the active track.');
		}
		const zoomConstraints = { zoom: value } as unknown as MediaTrackConstraintSet;
		await track.applyConstraints({ advanced: [zoomConstraints] });
	}

	async function handleZoomInput(event: Event & { currentTarget: HTMLInputElement }) {
		const target = event.currentTarget;
		const numeric = Number(target.value);
		const nextValue = clamp(numeric, zoomMin, zoomMax);
		const previous = zoom;
		zoom = nextValue;
		if (!zoomSupported || !previewActive) {
			return;
		}
		try {
			await applyZoom(nextValue);
		} catch (err) {
			zoom = previous;
			target.value = previous.toString();
			const message =
				err instanceof DOMException || err instanceof Error
					? err.message
					: 'Unable to adjust zoom for this camera.';
			errorMessage = message;
			logAction('Zoom adjustment failed', message, 'draft');
		}
	}

	function formatSeconds(value: number): string {
		const totalSeconds = Math.max(0, Math.floor(value));
		const hours = Math.floor(totalSeconds / 3600);
		const minutes = Math.floor((totalSeconds % 3600) / 60);
		const seconds = totalSeconds % 60;
		if (hours > 0) {
			return `${hours}:${minutes.toString().padStart(2, '0')}:${seconds
				.toString()
				.padStart(2, '0')}`;
		}
		return `${minutes}:${seconds.toString().padStart(2, '0')}`;
	}

	function formatDurationLabel(totalSeconds: number): string {
		const hours = Math.floor(totalSeconds / 3600);
		const minutes = Math.floor((totalSeconds % 3600) / 60);
		const seconds = totalSeconds % 60;
		const segments: string[] = [];
		if (hours > 0) {
			segments.push(`${hours}h`);
		}
		if (minutes > 0) {
			segments.push(`${minutes}m`);
		}
		segments.push(`${seconds}s`);
		return segments.join(' ');
	}

	function formatTimestamp(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) {
			return value;
		}
		return new Intl.DateTimeFormat(undefined, {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		}).format(date);
	}

	function formatBytes(value: number): string {
		if (!Number.isFinite(value) || value <= 0) {
			return '0 B';
		}
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let current = value;
		let index = 0;
		while (current >= 1024 && index < units.length - 1) {
			current /= 1024;
			index += 1;
		}
		const decimals = index === 0 ? 0 : 1;
		return `${current.toFixed(decimals)} ${units[index]}`;
	}

	function recordingExtension(mimeType: string): string {
		if (mimeType.includes('mp4')) {
			return 'mp4';
		}
		if (mimeType.includes('ogg')) {
			return 'ogg';
		}
		return 'webm';
	}

	function stopRecordingTimer(reset = false) {
		if (recordingTimer) {
			clearInterval(recordingTimer);
			recordingTimer = null;
		}
		if (reset) {
			recordingSeconds = 0;
		}
	}

	function startRecordingTimer() {
		stopRecordingTimer(true);
		recordingTimer = setInterval(() => {
			if (recordingStartedAt > 0) {
				const elapsed = Math.max(0, Date.now() - recordingStartedAt);
				recordingSeconds = Math.round(elapsed / 1000);
			}
		}, 500);
	}

	async function stopRecording(discard = false): Promise<void> {
		const recorder = mediaRecorder;
		if (!recorder) {
			recordingActive = false;
			discardRecording = false;
			stopRecordingTimer(true);
			return;
		}
		if (recorder.state !== 'recording') {
			recordingActive = false;
			discardRecording = false;
			stopRecordingTimer(true);
			return;
		}

		discardRecording = discard;
		recordingActive = false;
		stopRecordingTimer(false);

		await new Promise<void>((resolve) => {
			const cleanup = () => {
				recorder.removeEventListener('stop', cleanup);
				resolve();
			};
			recorder.addEventListener('stop', cleanup, { once: true });
			try {
				recorder.stop();
			} catch {
				recorder.removeEventListener('stop', cleanup);
				resolve();
			}
		});
	}

	async function stopPreview(
		reason?: string,
		options: { discardRecording?: boolean } = {}
	): Promise<void> {
		const detail = describeTrack(stream?.getVideoTracks()[0] ?? null);
		await stopRecording(options.discardRecording ?? false);
		releaseStream();
		if (reason) {
			logAction(reason, detail, 'complete');
		}
	}

	function releaseStream() {
		if (stream) {
			for (const track of stream.getTracks()) {
				if (typeof track.removeEventListener === 'function') {
					track.removeEventListener('ended', handleTrackEnded);
				}
				track.stop();
			}
		}
		if (videoElement) {
			videoElement.pause();
			videoElement.srcObject = null;
		}
		stream = null;
		previewActive = false;
		resetZoomCapabilities();
	}

	function selectRecorderMimeType(): string | undefined {
		if (
			typeof MediaRecorder === 'undefined' ||
			typeof MediaRecorder.isTypeSupported !== 'function'
		) {
			return undefined;
		}
		const candidates = [
			'video/webm;codecs=vp9',
			'video/webm;codecs=vp8',
			'video/webm;codecs=vp8.0',
			'video/webm',
			'video/mp4'
		];
		for (const candidate of candidates) {
			try {
				if (MediaRecorder.isTypeSupported(candidate)) {
					return candidate;
				}
			} catch {
				// ignore unsupported candidate
			}
		}
		return undefined;
	}

	async function startPreview(options: { restart?: boolean } = {}) {
		if (initializing) {
			return;
		}
		if (
			!mediaSupported ||
			typeof navigator === 'undefined' ||
			!navigator.mediaDevices?.getUserMedia
		) {
			errorMessage = 'Webcam preview is not supported in this environment.';
			logAction('Webcam preview failed', 'MediaDevices API unavailable', 'draft');
			return;
		}

		initializing = true;
		const restarting = options.restart ?? false;
		errorMessage = null;

		await stopPreview(undefined, { discardRecording: true });

		try {
			const constraints = buildConstraints();
			const nextStream = await navigator.mediaDevices.getUserMedia(constraints);
			stream = nextStream;
			const track = nextStream.getVideoTracks()[0] ?? null;
			if (track && typeof track.addEventListener === 'function') {
				track.addEventListener('ended', handleTrackEnded);
			}
			await tick();
			if (videoElement) {
				videoElement.srcObject = nextStream;
				try {
					await videoElement.play();
				} catch {
					// playback errors can be ignored in muted preview mode
				}
			}
			previewActive = true;
			await configureZoom(track);
			const settings =
				track && typeof track.getSettings === 'function'
					? (track.getSettings() as MediaTrackSettings)
					: undefined;
			const detail = describeTrack(track);
			const resolutionLabel = formatResolutionLabel(settings ?? null);
			const fpsSetting =
				settings && typeof settings.frameRate === 'number'
					? Math.round(settings.frameRate)
					: frameRate;
			logAction(
				restarting ? 'Webcam preview restarted' : 'Webcam preview started',
				detail,
				'complete',
				{
					camera: cameraLabel(),
					resolution: resolutionLabel,
					frameRate: fpsSetting
				}
			);
			void refreshDevices();
		} catch (err) {
			releaseStream();
			const message =
				err instanceof DOMException || err instanceof Error
					? err.message
					: 'Unable to start webcam preview.';
			errorMessage = message;
			logAction('Webcam preview failed', message, 'draft', {
				camera: cameraLabel(),
				resolution,
				frameRate
			});
		} finally {
			initializing = false;
		}
	}

	async function restartPreview() {
		if (!previewActive || initializing) {
			return;
		}
		await startPreview({ restart: true });
	}

	async function refreshDevices() {
		if (
			!mediaSupported ||
			typeof navigator === 'undefined' ||
			!navigator.mediaDevices?.enumerateDevices
		) {
			return;
		}
		try {
			const allDevices = await navigator.mediaDevices.enumerateDevices();
			const cameras = allDevices
				.filter((device) => device.kind === 'videoinput')
				.map((device, index) => ({
					id: device.deviceId,
					label: device.label || `Camera ${index + 1}`
				}));
			devices = cameras;

			if (selectedCamera && !cameras.some((device) => device.id === selectedCamera)) {
				if (cameras.length > 0) {
					selectedCamera = cameras[0].id;
				} else {
					selectedCamera = '';
				}
				if (previewActive && cameras.length === 0) {
					await stopPreview('Webcam preview stopped', { discardRecording: true });
				}
			} else if (!selectedCamera && cameras.length > 0) {
				selectedCamera = cameras[0].id;
			}
		} catch (err) {
			if (err instanceof DOMException && err.name === 'NotAllowedError') {
				return;
			}
			const message = err instanceof Error ? err.message : 'Unable to enumerate cameras.';
			errorMessage = message;
		}
	}

	async function handleFrameRateChange(event: Event & { currentTarget: HTMLInputElement }) {
		const target = event.currentTarget;
		const numeric = Number(target.value);
		const next = clamp(Math.round(Number.isFinite(numeric) ? numeric : frameRate), 1, 120);
		frameRate = next;
		target.value = next.toString();
		if (previewActive && !initializing) {
			await restartPreview();
		}
	}

	function handleResolutionChange(value: string) {
		if (value === resolution) {
			return;
		}
		resolution = value as (typeof RESOLUTION_OPTIONS)[number]['value'];
		if (previewActive && !initializing) {
			void restartPreview();
		}
	}

	function handleCameraChange(value: string) {
		if (value === selectedCamera) {
			return;
		}
		selectedCamera = value;
		if (previewActive && !initializing) {
			void restartPreview();
		}
	}

	function handleTrackEnded() {
		void stopPreview('Webcam preview ended', { discardRecording: false });
	}

	function captureStill() {
		if (!previewActive || !videoElement) {
			const message = 'Start the preview before capturing still images.';
			errorMessage = message;
			logAction('Webcam capture failed', message, 'draft');
			return;
		}
		const width = videoElement.videoWidth;
		const height = videoElement.videoHeight;
		if (width === 0 || height === 0) {
			const message = 'The webcam stream is not ready yet.';
			errorMessage = message;
			logAction('Webcam capture failed', message, 'draft');
			return;
		}
		const canvas = document.createElement('canvas');
		canvas.width = width;
		canvas.height = height;
		const context = canvas.getContext('2d');
		if (!context) {
			const message = 'Unable to read frames from the webcam stream.';
			errorMessage = message;
			logAction('Webcam capture failed', message, 'draft');
			return;
		}
		context.drawImage(videoElement, 0, 0, width, height);
		const url = canvas.toDataURL('image/png');
		const capture: StillCapture = {
			id: generateId(),
			url,
			width,
			height,
			timestamp: new Date().toISOString(),
			cameraLabel: cameraLabel()
		} satisfies StillCapture;
		captures = [capture, ...captures].slice(0, 8);
		errorMessage = null;
		logAction('Webcam still captured', `${capture.cameraLabel} · ${width}×${height}`, 'complete', {
			camera: capture.cameraLabel,
			width,
			height
		});
	}

	function removeCapture(id: string) {
		captures = captures.filter((capture) => capture.id !== id);
	}

	async function startRecording() {
		if (!previewActive || !stream) {
			const message = 'Start the preview before recording video.';
			errorMessage = message;
			logAction('Webcam recording failed', message, 'draft');
			return;
		}
		if (typeof MediaRecorder === 'undefined') {
			const message = 'MediaRecorder is not supported in this environment.';
			errorMessage = message;
			logAction('Webcam recording failed', message, 'draft');
			return;
		}
		if (mediaRecorder && mediaRecorder.state === 'recording') {
			return;
		}

	const track = stream.getVideoTracks()[0] ?? null;
	const settings = track?.getSettings() ?? null;
		const resolutionLabel = formatResolutionLabel(settings);
		const fps =
			typeof settings?.frameRate === 'number' && !Number.isNaN(settings.frameRate)
				? Math.round(settings.frameRate)
				: frameRate;
		const cameraName = cameraLabel();
		const mimeType = selectRecorderMimeType();

		let recorder: MediaRecorder;
		try {
			recorder = mimeType ? new MediaRecorder(stream, { mimeType }) : new MediaRecorder(stream);
		} catch (err) {
			const message =
				err instanceof DOMException || err instanceof Error
					? err.message
					: 'Unable to start recording.';
			errorMessage = message;
			logAction('Webcam recording failed', message, 'draft');
			return;
		}

		recordedChunks = [];
		discardRecording = false;
		mediaRecorder = recorder;

		recorder.addEventListener('dataavailable', (event) => {
			if (event.data && event.data.size > 0) {
				recordedChunks.push(event.data);
			}
		});

		recorder.addEventListener('error', (event) => {
			const errorEvent = event as Event & { error?: DOMException };
			const message = errorEvent.error?.message ?? 'An unknown recording error occurred.';
			errorMessage = message;
			logAction('Webcam recording error', message, 'draft');
		});

		recorder.addEventListener('stop', () => {
			const durationMs = Math.max(0, Date.now() - recordingStartedAt);
			const seconds = Math.max(0, Math.round(durationMs / 1000));
			const blob =
				recordedChunks.length > 0
					? new Blob(recordedChunks, {
							type: recorder.mimeType || mimeType || 'video/webm'
						})
					: null;

			if (discardRecording) {
				recordedChunks = [];
				discardRecording = false;
				mediaRecorder = null;
				recordingStartedAt = 0;
				stopRecordingTimer(true);
				return;
			}

			if (!blob) {
				recordedChunks = [];
				mediaRecorder = null;
				recordingStartedAt = 0;
				stopRecordingTimer(true);
				logAction('Webcam recording discarded', 'No data was captured', 'draft');
				return;
			}

			const url = URL.createObjectURL(blob);
			objectUrls.add(url);
			const clip: RecordingClip = {
				id: generateId(),
				url,
				mimeType: blob.type || mimeType || 'video/webm',
				size: blob.size,
				durationMs,
				createdAt: new Date().toISOString(),
				cameraLabel: cameraName,
				resolution: resolutionLabel,
				frameRate: fps ?? null
			} satisfies RecordingClip;

			recordings = [clip, ...recordings].slice(0, 8);
			recordedChunks = [];
			mediaRecorder = null;
			recordingStartedAt = 0;
			stopRecordingTimer(true);
			logAction(
				'Webcam recording saved',
				`${cameraName} · ${resolutionLabel}${fps ? ` @ ${fps}fps` : ''} · ${formatDurationLabel(seconds)}`,
				'complete'
			);
		});

		try {
			recorder.start();
			recordingStartedAt = Date.now();
			recordingActive = true;
			startRecordingTimer();
			errorMessage = null;
			logAction(
				'Webcam recording started',
				`${cameraName} · ${resolutionLabel}${fps ? ` @ ${fps}fps` : ''}`,
				'in-progress'
			);
		} catch (err) {
			recordedChunks = [];
			mediaRecorder = null;
			recordingStartedAt = 0;
			stopRecordingTimer(true);
			const message =
				err instanceof DOMException || err instanceof Error
					? err.message
					: 'Unable to start recording.';
			errorMessage = message;
			logAction('Webcam recording failed', message, 'draft');
		}
	}

	async function handleToggleRecording() {
		if (recordingActive) {
			await stopRecording(false);
		} else {
			await startRecording();
		}
	}

	function removeRecording(id: string) {
		const clip = recordings.find((item) => item.id === id);
		if (clip) {
			URL.revokeObjectURL(clip.url);
			objectUrls.delete(clip.url);
		}
		recordings = recordings.filter((clip) => clip.id !== id);
	}

	onMount(() => {
		const supported = typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia;
		mediaSupported = supported;
		if (!supported) {
			errorMessage = 'MediaDevices API unavailable in this environment.';
			return () => {
				for (const url of objectUrls) {
					URL.revokeObjectURL(url);
				}
				objectUrls.clear();
			};
		}

		void refreshDevices();

		const handleDeviceChange = () => {
			void refreshDevices();
		};

		navigator.mediaDevices.addEventListener?.('devicechange', handleDeviceChange);

		return () => {
			navigator.mediaDevices.removeEventListener?.('devicechange', handleDeviceChange);
			void stopPreview(undefined, { discardRecording: true });
			for (const url of objectUrls) {
				URL.revokeObjectURL(url);
			}
			objectUrls.clear();
		};
	});
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Live preview</CardTitle>
			<CardDescription
				>Start the webcam stream and adjust capture parameters in real time.</CardDescription
			>
		</CardHeader>
		<CardContent>
			<div class="grid gap-6 lg:grid-cols-[3fr_2fr]">
				<div class="space-y-4">
					<div
						class="relative aspect-video overflow-hidden rounded-xl border border-border/60 bg-black"
					>
						<video
							bind:this={videoElement}
							class="h-full w-full object-cover"
							autoplay
							muted
							playsinline
						></video>
						{#if !previewActive}
							<div
								class="absolute inset-0 flex items-center justify-center bg-background/80 text-sm text-muted-foreground"
							>
								<p>{initializing ? 'Requesting camera access…' : 'Preview inactive'}</p>
							</div>
						{/if}
						{#if recordingActive}
							<div
								class="text-destructive-foreground absolute top-4 left-4 flex items-center gap-2 rounded-full bg-destructive/80 px-3 py-1 text-xs font-medium shadow-sm"
							>
								<span class="bg-destructive-foreground size-2 rounded-full"></span>
								Recording {formatSeconds(recordingSeconds)}
							</div>
						{/if}
					</div>

					<div class="flex flex-wrap gap-3">
						<Button
							type="button"
							onclick={() => {
								void startPreview({ restart: previewActive });
							}}
							disabled={initializing || !mediaSupported}
						>
							Start preview
						</Button>
						<Button
							type="button"
							variant="outline"
							onclick={() => {
								void stopPreview('Webcam preview stopped', { discardRecording: false });
							}}
							disabled={!previewActive}
						>
							Stop preview
						</Button>
						<Button
							type="button"
							variant="secondary"
							onclick={captureStill}
							disabled={!previewActive}
						>
							Capture still
						</Button>
						<Button
							type="button"
							variant={recordingActive ? 'destructive' : 'secondary'}
							onclick={() => {
								void handleToggleRecording();
							}}
							disabled={!previewActive || initializing || !mediaSupported}
						>
							{recordingActive ? 'Stop recording' : 'Record video'}
						</Button>
					</div>

					{#if errorMessage}
						<p class="text-sm text-destructive">{errorMessage}</p>
					{/if}
				</div>

				<div class="space-y-4">
					<div class="grid gap-2">
						<Label for="webcam-camera">Camera</Label>
						{#if mediaSupported && devices.length > 0}
							<Select type="single" value={selectedCamera} onValueChange={handleCameraChange}>
								<SelectTrigger id="webcam-camera" class="w-full">
									<span class="truncate">{cameraLabel()}</span>
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="">Auto (system default)</SelectItem>
									{#each devices as device (device.id)}
										<SelectItem value={device.id}>{device.label}</SelectItem>
									{/each}
								</SelectContent>
							</Select>
						{:else}
							<Input id="webcam-camera" value="No cameras detected" readonly disabled />
						{/if}
					</div>

					<div class="grid gap-2">
						<Label for="webcam-resolution">Resolution</Label>
						<Select type="single" value={resolution} onValueChange={handleResolutionChange}>
							<SelectTrigger id="webcam-resolution" class="w-full">
								<span>{resolution}</span>
							</SelectTrigger>
							<SelectContent>
								{#each RESOLUTION_OPTIONS as option (option.value)}
									<SelectItem value={option.value}>{option.label}</SelectItem>
								{/each}
							</SelectContent>
						</Select>
					</div>

					<div class="grid gap-2">
						<Label for="webcam-framerate">Frame rate (fps)</Label>
						<Input
							id="webcam-framerate"
							type="number"
							min={1}
							max={120}
							bind:value={frameRate}
							onchange={handleFrameRateChange}
						/>
					</div>

					<div class="grid gap-2">
						<Label for="webcam-zoom">Zoom</Label>
						<div class="flex items-center gap-3">
							<Input
								id="webcam-zoom"
								type="range"
								min={zoomMin}
								max={zoomMax}
								step={zoomStep}
								value={zoom}
								oninput={handleZoomInput}
								disabled={!zoomSupported || !previewActive}
							/>
							<span class="w-16 text-right text-sm text-muted-foreground">
								{zoomSupported ? `${zoom.toFixed(2)}×` : '—'}
							</span>
						</div>
						<p class="text-xs text-muted-foreground">
							{zoomSupported
								? 'Adjust optical zoom when supported by the selected camera.'
								: 'Zoom control is unavailable for this camera.'}
						</p>
					</div>
				</div>
			</div>
		</CardContent>
	</Card>

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Still captures</CardTitle>
			<CardDescription>Download snapshots captured from the live webcam feed.</CardDescription>
		</CardHeader>
		<CardContent>
			{#if captures.length === 0}
				<p class="text-sm text-muted-foreground">
					No still captures yet. Use “Capture still” while the preview is active.
				</p>
			{:else}
				<div class="grid gap-4 md:grid-cols-2">
					{#each captures as capture (capture.id)}
						<div class="space-y-3 rounded-lg border border-border/60 bg-muted/30 p-3">
							<img
								src={capture.url}
								alt={`Capture from ${capture.cameraLabel}`}
								class="aspect-video w-full rounded-md object-cover"
							/>
							<div class="space-y-1 text-xs">
								<p class="font-medium text-foreground">{capture.cameraLabel}</p>
								<p class="text-muted-foreground">
									{capture.width}×{capture.height} · {formatTimestamp(capture.timestamp)}
								</p>
							</div>
							<div class="flex gap-2">
								<Button
									href={capture.url}
									download={`capture-${capture.id}.png`}
									variant="outline"
									size="sm"
								>
									Download
								</Button>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									onclick={() => {
										removeCapture(capture.id);
									}}
								>
									Remove
								</Button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</CardContent>
	</Card>

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Recorded clips</CardTitle>
			<CardDescription>Clips are stored locally until you download or remove them.</CardDescription>
		</CardHeader>
		<CardContent>
			{#if recordings.length === 0}
				<p class="text-sm text-muted-foreground">
					No recordings yet. Start the preview and use “Record video” to capture the feed.
				</p>
			{:else}
				<div class="space-y-3">
					{#each recordings as clip (clip.id)}
						<div
							class="flex flex-col gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:flex-row md:items-center md:justify-between"
						>
							<div class="space-y-1 text-xs">
								<p class="text-sm font-medium text-foreground">{clip.cameraLabel}</p>
								<p class="text-muted-foreground">
									{clip.resolution}
									{clip.frameRate ? ` @ ${clip.frameRate}fps` : ''}
									· {formatDurationLabel(Math.max(0, Math.round(clip.durationMs / 1000)))}
									· {formatBytes(clip.size)}
								</p>
								<p class="text-muted-foreground">{formatTimestamp(clip.createdAt)}</p>
							</div>
							<div class="flex gap-2">
								<Button
									href={clip.url}
									download={`recording-${clip.id}.${recordingExtension(clip.mimeType)}`}
									variant="outline"
									size="sm"
								>
									Download
								</Button>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									onclick={() => {
										removeRecording(clip.id);
									}}
								>
									Remove
								</Button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</CardContent>
	</Card>
</div>
