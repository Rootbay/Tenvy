<script lang="ts">
	import { get } from 'svelte/store';
	import { onDestroy, onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
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
	import { listAppVncApplications } from '$lib/data/app-vnc-apps';
	import type { Client } from '$lib/data/clients';
	import { createAppVncSessionController } from '$lib/stores/app-vnc-session';
	import type {
		AppVncApplicationDescriptor,
		AppVncInputEvent,
		AppVncSessionSettings
	} from '$lib/types/app-vnc';

	const { client } = $props<{ client: Client }>();

	const applications = listAppVncApplications();
	const qualityOptions: { value: AppVncSessionSettings['quality']; label: string }[] = [
		{ value: 'lossless', label: 'Lossless' },
		{ value: 'balanced', label: 'Balanced' },
		{ value: 'bandwidth', label: 'Bandwidth saver' }
	];
	const platformLabels: Record<AppVncApplicationDescriptor['platforms'][number], string> = {
		windows: 'Windows',
		linux: 'Linux',
		macos: 'macOS'
	};

	const {
		session,
		frameUrl,
		frameWidth,
		frameHeight,
		lastHeartbeat,
		isStarting,
		isStopping,
		isUpdating,
		errorMessage,
		infoMessage,
		startSession,
		updateSession,
		stopSession,
		refreshSession,
		enqueueEvent,
		dispose
	} = createAppVncSessionController({ clientId: client.id, initialSession: null });

	let monitor = $state('Primary');
	let quality = $state<AppVncSessionSettings['quality']>('balanced');
	let captureCursor = $state(true);
	let clipboardSync = $state(false);
	let blockLocalInput = $state(false);
	let heartbeatInterval = $state<number | string>(30);
	let appId = $state('');
	let windowTitle = $state('');
	let viewportEl: HTMLDivElement | null = null;
	let pointerActive = false;
	let activePointerId: number | null = null;

	const normalizedAppId = $derived(() => appId.trim());
	const selectedApp = $derived<AppVncApplicationDescriptor | null>(() => {
		const trimmed = normalizedAppId;
		return applications.find((app) => app.id === trimmed) ?? null;
	});
	const appSelectionLabel = $derived(() => {
		if (selectedApp) {
			return selectedApp.name;
		}
		return normalizedAppId ? `Custom · ${normalizedAppId}` : 'Manual selection';
	});

	function formatPlatforms(platforms?: AppVncApplicationDescriptor['platforms']): string {
		if (!platforms || platforms.length === 0) {
			return '';
		}
		return platforms.map((platform) => platformLabels[platform] ?? platform).join(', ');
	}

	function handleAppSelection(value: string) {
		appId = value.trim();
	}

	function resolveHeartbeatInterval(): number {
		const value = heartbeatInterval;
		if (typeof value === 'number') {
			return value;
		}
		const parsed = Number.parseInt(value, 10);
		if (Number.isFinite(parsed)) {
			return parsed;
		}
		return 30;
	}

	function buildSessionSettings(): AppVncSessionSettings {
		const heartbeat = resolveHeartbeatInterval();
		heartbeatInterval = heartbeat;
		const trimmedAppId = normalizedAppId;
		const trimmedWindowTitle = windowTitle.trim();
		return {
			monitor,
			quality,
			captureCursor,
			clipboardSync,
			blockLocalInput,
			heartbeatInterval: heartbeat,
			appId: trimmedAppId || undefined,
			windowTitle: trimmedWindowTitle || undefined
		} satisfies AppVncSessionSettings;
	}

	async function handleStartSession() {
		await startSession(buildSessionSettings());
	}

	async function handleUpdateSession() {
		await updateSession(buildSessionSettings());
	}

	async function handleStopSession() {
		await stopSession();
	}

	function pointerPosition(event: PointerEvent) {
		const element = viewportEl;
		if (!element) {
			return null;
		}
		const rect = element.getBoundingClientRect();
		if (rect.width <= 0 || rect.height <= 0) {
			return null;
		}
		const x = (event.clientX - rect.left) / rect.width;
		const y = (event.clientY - rect.top) / rect.height;
		return {
			x: Math.min(Math.max(x, 0), 1),
			y: Math.min(Math.max(y, 0), 1)
		};
	}

	function handlePointerMove(event: PointerEvent) {
		const current = get(session);
		if (!current?.active || !pointerActive) {
			return;
		}
		const position = pointerPosition(event);
		if (!position) {
			return;
		}
		enqueueEvent({
			type: 'pointer-move',
			capturedAt: Date.now(),
			x: position.x,
			y: position.y,
			normalized: true
		} satisfies AppVncInputEvent);
	}

	function handlePointerDown(event: PointerEvent) {
		const current = get(session);
		if (!current?.active) {
			return;
		}
		viewportEl?.focus();
		pointerActive = true;
		activePointerId = event.pointerId;
		try {
			event.currentTarget?.setPointerCapture?.(event.pointerId);
		} catch {
			// ignore capture failures
		}
		const position = pointerPosition(event);
		if (position) {
			enqueueEvent({
				type: 'pointer-move',
				capturedAt: Date.now(),
				x: position.x,
				y: position.y,
				normalized: true
			} satisfies AppVncInputEvent);
		}
		enqueueEvent({
			type: 'pointer-button',
			capturedAt: Date.now(),
			button: event.button === 2 ? 'right' : event.button === 1 ? 'middle' : 'left',
			pressed: true
		} satisfies AppVncInputEvent);
	}

	function handlePointerUp(event: PointerEvent) {
		if (pointerActive && activePointerId === event.pointerId) {
			enqueueEvent({
				type: 'pointer-button',
				capturedAt: Date.now(),
				button: event.button === 2 ? 'right' : event.button === 1 ? 'middle' : 'left',
				pressed: false
			} satisfies AppVncInputEvent);
			pointerActive = false;
			activePointerId = null;
			try {
				event.currentTarget?.releasePointerCapture?.(event.pointerId);
			} catch {
				// ignore release failure
			}
		}
	}

	function handlePointerCancel(event: PointerEvent) {
		if (!pointerActive) {
			return;
		}
		pointerActive = false;
		activePointerId = null;
		try {
			event.currentTarget?.releasePointerCapture?.(event.pointerId);
		} catch {
			// ignore
		}
	}

	function handleWheel(event: WheelEvent) {
		const current = get(session);
		if (!current?.active) {
			return;
		}
		event.preventDefault();
		enqueueEvent({
			type: 'pointer-scroll',
			capturedAt: Date.now(),
			deltaX: event.deltaX,
			deltaY: event.deltaY,
			deltaMode: event.deltaMode
		} satisfies AppVncInputEvent);
	}

	function handleKey(event: KeyboardEvent, pressed: boolean) {
		const current = get(session);
		if (!current?.active) {
			return;
		}
		event.preventDefault();
		enqueueEvent({
			type: 'key',
			capturedAt: Date.now(),
			pressed,
			key: event.key,
			code: event.code,
			keyCode: event.keyCode,
			repeat: event.repeat,
			altKey: event.altKey,
			ctrlKey: event.ctrlKey,
			shiftKey: event.shiftKey,
			metaKey: event.metaKey
		} satisfies AppVncInputEvent);
	}

	$effect(() => {
		const current = $session;
		if (current && current.active) {
			quality = current.settings.quality;
			monitor = current.settings.monitor;
			captureCursor = current.settings.captureCursor;
			clipboardSync = current.settings.clipboardSync;
			blockLocalInput = current.settings.blockLocalInput;
			heartbeatInterval = current.settings.heartbeatInterval;
			appId = current.settings.appId?.trim() ?? '';
			windowTitle = current.settings.windowTitle?.trim() ?? '';
		}
	});

	onMount(() => {
		void refreshSession();
	});

	onDestroy(() => {
		dispose();
	});
</script>

<div class="space-y-4">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Session parameters</CardTitle>
			<CardDescription>
				Configure the isolated App VNC workspace before engaging the client.
			</CardDescription>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-2">
				<div class="grid gap-4">
					<div class="grid gap-2">
						<Label for="workspace-avnc-application">Application profile</Label>
						<Select
							type="single"
							value={selectedApp ? selectedApp.id : ''}
							onValueChange={handleAppSelection}
						>
							<SelectTrigger id="workspace-avnc-application" class="w-full">
								<span class="truncate">{appSelectionLabel}</span>
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="">
									<span class="flex flex-col gap-0.5">
										<span class="font-medium">Manual selection</span>
										<span class="text-xs text-muted-foreground"
											>Provide a custom identifier below</span
										>
									</span>
								</SelectItem>
								{#each applications as application (application.id)}
									<SelectItem value={application.id}>
										<span class="flex flex-col gap-0.5">
											<span class="font-medium">{application.name}</span>
											<span class="text-xs text-muted-foreground">{application.summary}</span>
										</span>
									</SelectItem>
								{/each}
							</SelectContent>
						</Select>
						{#if selectedApp}
							<p class="text-xs text-muted-foreground">
								{selectedApp.summary}
								{#if selectedApp.platforms?.length}
									· Supports {formatPlatforms(selectedApp.platforms)}
								{/if}
								{#if selectedApp.windowTitleHint}
									· Window title hint: “{selectedApp.windowTitleHint}”
								{/if}
							</p>
						{:else if normalizedAppId}
							<p class="text-xs text-muted-foreground">
								Custom profile targeting “{normalizedAppId}”. Ensure the agent recognises this
								identifier.
							</p>
						{:else}
							<p class="text-xs text-muted-foreground">
								Choose one of {applications.length} built-in profiles or provide your own identifier.
							</p>
						{/if}
					</div>
					<div class="grid gap-2">
						<Label for="workspace-avnc-app-id">Custom app identifier</Label>
						<Input
							id="workspace-avnc-app-id"
							placeholder="Override or provide your own identifier"
							bind:value={appId}
						/>
						<p class="text-xs text-muted-foreground">
							Applied value is forwarded directly to the agent; leave blank to rely on the selected
							profile.
						</p>
					</div>
				</div>
				<div class="grid gap-4">
					<div class="grid gap-2">
						<Label for="workspace-avnc-monitor">Preferred monitor</Label>
						<Input id="workspace-avnc-monitor" placeholder="Primary display" bind:value={monitor} />
						<p class="text-xs text-muted-foreground">
							The agent advertises active displays when a session handshake is established.
						</p>
					</div>
					<div class="grid gap-2">
						<Label for="workspace-avnc-quality">Encoding profile</Label>
						<Select
							type="single"
							value={quality}
							onValueChange={(value) => (quality = value as typeof quality)}
						>
							<SelectTrigger id="workspace-avnc-quality" class="w-full">
								<span class="truncate">
									{qualityOptions.find((option) => option.value === quality)?.label ?? quality}
								</span>
							</SelectTrigger>
							<SelectContent>
								{#each qualityOptions as option (option.value)}
									<SelectItem value={option.value}>{option.label}</SelectItem>
								{/each}
							</SelectContent>
						</Select>
					</div>
				</div>
			</div>

			<div class="grid gap-4 md:grid-cols-3">
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Mirror cursor</p>
						<p class="text-xs text-muted-foreground">Display remote cursor state</p>
					</div>
					<Switch bind:checked={captureCursor} />
				</label>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Clipboard tunnel</p>
						<p class="text-xs text-muted-foreground">Enable clipboard mirroring</p>
					</div>
					<Switch bind:checked={clipboardSync} />
				</label>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Lock local input</p>
						<p class="text-xs text-muted-foreground">Block physical keyboard and mouse locally</p>
					</div>
					<Switch bind:checked={blockLocalInput} />
				</label>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				<div class="grid gap-2">
					<Label for="workspace-avnc-heartbeat">Heartbeat interval (seconds)</Label>
					<Input
						id="workspace-avnc-heartbeat"
						type="number"
						min={10}
						step={5}
						bind:value={heartbeatInterval}
					/>
					<p class="text-xs text-muted-foreground">
						Controls how often the agent renews the isolated session lease.
					</p>
				</div>
				<div class="grid gap-2">
					<Label for="workspace-avnc-window">Window title filter</Label>
					<Input
						id="workspace-avnc-window"
						placeholder="Optional window title"
						bind:value={windowTitle}
					/>
					<p class="text-xs text-muted-foreground">
						Restrict the session to windows matching this title fragment.
					</p>
				</div>
			</div>

			<div class="flex flex-wrap gap-3">
				{#if $session?.active}
					<Button type="button" onclick={handleUpdateSession} disabled={$isUpdating}>
						{$isUpdating ? 'Updating…' : 'Update session'}
					</Button>
					<Button
						type="button"
						variant="outline"
						onclick={handleStopSession}
						disabled={$isStopping}
					>
						{$isStopping ? 'Stopping…' : 'Stop session'}
					</Button>
				{:else}
					<Button type="button" onclick={handleStartSession} disabled={$isStarting}>
						{$isStarting ? 'Starting…' : 'Start session'}
					</Button>
				{/if}
			</div>

			{#if $errorMessage}
				<p class="text-sm text-destructive">{$errorMessage}</p>
			{/if}
			{#if $infoMessage}
				<p class="text-sm text-emerald-500">{$infoMessage}</p>
			{/if}
		</CardContent>
	</Card>

	<Card>
		<CardHeader>
			<CardTitle class="text-base">Live application surface</CardTitle>
			<CardDescription>
				Engage with the remote workspace using covert application VNC transport.
			</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4">
			<div
				class="relative flex h-[360px] w-full items-center justify-center overflow-hidden rounded-lg border bg-black"
				tabindex="0"
				bind:this={viewportEl}
				data-testid="app-vnc-viewport"
				on:pointerdown={handlePointerDown}
				on:pointermove={handlePointerMove}
				on:pointerup={handlePointerUp}
				on:pointercancel={handlePointerCancel}
				on:wheel={handleWheel}
				on:keydown={(event) => handleKey(event, true)}
				on:keyup={(event) => handleKey(event, false)}
			>
				{#if $frameUrl}
					<img
						src={$frameUrl}
						alt="App VNC frame"
						class="max-h-full max-w-full select-none"
						draggable={false}
					/>
				{:else}
					<p class="text-sm text-muted-foreground">No frame data available yet.</p>
				{/if}
			</div>
			<div class="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2">
				<div>
					<span class="font-medium text-foreground">Session</span>
					<span class="ml-2">{$session?.active ? 'Active' : 'Idle'}</span>
				</div>
				<div>
					<span class="font-medium text-foreground">Frame size</span>
					<span class="ml-2">
						{#if $frameWidth && $frameHeight}
							{$frameWidth}×{$frameHeight}
						{:else}
							—
						{/if}
					</span>
				</div>
				<div>
					<span class="font-medium text-foreground">Last heartbeat</span>
					<span class="ml-2">{$lastHeartbeat ?? '—'}</span>
				</div>
				<div>
					<span class="font-medium text-foreground">Session ID</span>
					<span class="ml-2 truncate">{$session?.sessionId ?? '—'}</span>
				</div>
			</div>
		</CardContent>
	</Card>
</div>
