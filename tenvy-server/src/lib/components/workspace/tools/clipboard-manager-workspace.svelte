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
	import { notifyToolActivationCommand } from '$lib/utils/agent-commands.js';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';

const { client } = $props<{ client: Client }>();

const tool = getClientTool('clipboard-manager');
void tool;

	let captureText = $state(true);
	let captureFiles = $state(false);
	let captureImages = $state(true);
	let redactSecrets = $state(true);
	let syncBack = $state(false);
	let pollInterval = $state(5);
	let log = $state<WorkspaceLogEntry[]>([]);

	function describePlan(): string {
		const segments = [
			`${captureText ? 'text' : ''} ${captureImages ? 'image' : ''} ${captureFiles ? 'files' : ''}`.trim(),
			`interval ${pollInterval}s`,
			redactSecrets ? 'redaction on' : 'redaction off'
		];
		if (syncBack) segments.push('bidirectional');
		return segments.join(' · ');
	}

	function queue(status: WorkspaceLogEntry['status']) {
		const detail = describePlan();
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry('Clipboard strategy staged', detail, status)
		);
		notifyToolActivationCommand(client.id, 'clipboard-manager', {
			action: 'event:Clipboard strategy staged',
			metadata: {
				detail,
				status,
				captureText,
				captureImages,
				captureFiles,
				redactSecrets,
				syncBack,
				pollInterval
			}
		});
	}
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Capture rules</CardTitle>
			<CardDescription>Choose which clipboard formats to monitor and how often.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-3">
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Capture text</p>
						<p class="text-xs text-muted-foreground">Record textual clipboard entries</p>
					</div>
					<Switch bind:checked={captureText} />
				</label>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Capture images</p>
						<p class="text-xs text-muted-foreground">Base64 encode small image payloads</p>
					</div>
					<Switch bind:checked={captureImages} />
				</label>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Capture files</p>
						<p class="text-xs text-muted-foreground">List file drops and stage for transfer</p>
					</div>
					<Switch bind:checked={captureFiles} />
				</label>
			</div>
			<div class="grid gap-4 md:grid-cols-2">
				<div class="grid gap-2">
					<Label for="clipboard-interval">Polling interval (seconds)</Label>
					<Input id="clipboard-interval" type="number" min={1} step={1} bind:value={pollInterval} />
				</div>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Redact secrets</p>
						<p class="text-xs text-muted-foreground">Mask passwords or card patterns</p>
					</div>
					<Switch bind:checked={redactSecrets} />
				</label>
			</div>
			<label
				class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2"
			>
				<div>
					<p class="text-sm font-medium text-foreground">Sync clipboard back</p>
					<p class="text-xs text-muted-foreground">Allow operator to push data to the host</p>
				</div>
				<Switch bind:checked={syncBack} />
			</label>
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
			<Button type="button" onclick={() => queue('queued')}>Queue monitoring</Button>
		</CardFooter>
	</Card>
</div>
