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
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';

	const { client } = $props<{ client: Client }>();

	const tool = getClientTool('report-window');

	let feed = $state<'live' | 'batch'>('live');
	let refreshSeconds = $state(5);
	let includeScreenshots = $state(false);
	let includeCommands = $state(true);
	let log = $state<WorkspaceLogEntry[]>([]);

	const previewMetrics = [
		{ label: 'CPU', value: '22%' },
		{ label: 'Memory', value: '3.1 GB' },
		{ label: 'Pending Commands', value: '2' }
	];

	function describePlan(): string {
		return `${feed} feed · refresh ${refreshSeconds}s · screenshots ${includeScreenshots ? 'on' : 'off'} · commands ${includeCommands ? 'included' : 'excluded'}`;
	}

	function queue(status: WorkspaceLogEntry['status']) {
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry('Report window staged', describePlan(), status)
		);
	}
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Feed configuration</CardTitle>
			<CardDescription
				>Adjust how frequently data is collected and what extras are included.</CardDescription
			>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-3">
				<div class="grid gap-2">
					<Label for="report-feed">Feed type</Label>
					<Select
						type="single"
						value={feed}
						onValueChange={(value) => (feed = value as typeof feed)}
					>
						<SelectTrigger id="report-feed" class="w-full">
							<span class="capitalize">{feed}</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="live">Live</SelectItem>
							<SelectItem value="batch">Batch</SelectItem>
						</SelectContent>
					</Select>
				</div>
				<div class="grid gap-2">
					<Label for="report-refresh">Refresh (seconds)</Label>
					<Input
						id="report-refresh"
						type="number"
						min={feed === 'live' ? 2 : 30}
						step={feed === 'live' ? 1 : 30}
						bind:value={refreshSeconds}
					/>
				</div>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Include command stream</p>
						<p class="text-xs text-muted-foreground">Show queued/complete command states</p>
					</div>
					<Switch bind:checked={includeCommands} />
				</label>
			</div>
			<label
				class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2"
			>
				<div>
					<p class="text-sm font-medium text-foreground">Embed screenshots</p>
					<p class="text-xs text-muted-foreground">Attach periodic mini screenshots to the feed</p>
				</div>
				<Switch bind:checked={includeScreenshots} />
			</label>
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
			<Button type="button" onclick={() => queue('queued')}>Queue workspace</Button>
		</CardFooter>
	</Card>

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Preview</CardTitle>
			<CardDescription
				>Representative metrics that will surface once telemetry is live.</CardDescription
			>
		</CardHeader>
		<CardContent class="grid gap-4 md:grid-cols-3">
			{#each previewMetrics as metric (metric.label)}
				<div class="rounded-lg border border-border/60 bg-muted/30 p-4">
					<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
						{metric.label}
					</p>
					<p class="mt-2 text-lg font-semibold text-foreground">{metric.value}</p>
				</div>
			{/each}
		</CardContent>
	</Card>
</div>
