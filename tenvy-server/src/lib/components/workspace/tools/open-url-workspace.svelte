<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
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
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';

	const { client } = $props<{ client: Client }>();
	void client;

	const tool = getClientTool('open-url');
	void tool;

	let url = $state('https://');
	let referer = $state('');
	let browserChoice = $state<'default' | 'edge' | 'chrome' | 'firefox'>('default');
	let scheduleMinutes = $state(0);
	let note = $state('');
	let log = $state<WorkspaceLogEntry[]>([]);

	function describePlan(): string {
		return `${url} · browser ${browserChoice} · ${scheduleMinutes > 0 ? `delay ${scheduleMinutes}m` : 'run now'}${referer ? ` · referer ${referer}` : ''}`;
	}

	function queue(status: WorkspaceLogEntry['status']) {
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry('URL launch staged', describePlan(), status)
		);
	}
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Launch parameters</CardTitle>
			<CardDescription>Define how and when the URL should be opened on the client.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-2">
				<Label for="open-url-target">URL</Label>
				<Input id="open-url-target" type="url" bind:value={url} placeholder="https://target" />
			</div>
			<div class="grid gap-4 md:grid-cols-2">
				<div class="grid gap-2">
					<Label for="open-url-browser">Browser</Label>
					<Select
						type="single"
						value={browserChoice}
						onValueChange={(value) => (browserChoice = value as typeof browserChoice)}
					>
						<SelectTrigger id="open-url-browser" class="w-full">
							<span class="capitalize">{browserChoice}</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="default">System default</SelectItem>
							<SelectItem value="edge">Microsoft Edge</SelectItem>
							<SelectItem value="chrome">Google Chrome</SelectItem>
							<SelectItem value="firefox">Mozilla Firefox</SelectItem>
						</SelectContent>
					</Select>
				</div>
				<div class="grid gap-2">
					<Label for="open-url-delay">Delay (minutes)</Label>
					<Input id="open-url-delay" type="number" min={0} step={1} bind:value={scheduleMinutes} />
				</div>
			</div>
			<div class="grid gap-2">
				<Label for="open-url-referer">Referer header</Label>
				<Input id="open-url-referer" bind:value={referer} placeholder="https://source.example" />
			</div>
			<div class="grid gap-2">
				<Label for="open-url-note">Operator note</Label>
				<textarea
					id="open-url-note"
					class="min-h-20 w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none"
					bind:value={note}
					placeholder="Explain why this URL is being launched."
				></textarea>
			</div>
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
			<Button type="button" onclick={() => queue('queued')}>Queue launch</Button>
		</CardFooter>
	</Card>
</div>
