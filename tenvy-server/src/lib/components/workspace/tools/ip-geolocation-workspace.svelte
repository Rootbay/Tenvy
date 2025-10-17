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

	const { client } = $props<{ client: Client }>();
	void client;

	const tool = getClientTool('ip-geolocation');
	void tool;

	let provider = $state<'ipinfo' | 'maxmind' | 'db-ip'>('ipinfo');
	let includeTimezone = $state(true);
	let includeMap = $state(true);
	let cacheHours = $state(6);
	let log = $state<WorkspaceLogEntry[]>([]);

	function describePlan(): string {
		return `${provider} provider · cache ${cacheHours}h · timezone ${includeTimezone ? 'on' : 'off'} · map ${includeMap ? 'on' : 'off'}`;
	}

	function queue(status: WorkspaceLogEntry['status']) {
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry('Geo lookup staged', describePlan(), status)
		);
	}
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Lookup configuration</CardTitle>
			<CardDescription>Adjust provider and enrichment options.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-3">
				<div class="grid gap-2">
					<Label for="geo-provider">Provider</Label>
					<Select
						type="single"
						value={provider}
						onValueChange={(value) => (provider = value as typeof provider)}
					>
						<SelectTrigger id="geo-provider" class="w-full">
							<span class="uppercase">{provider}</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="ipinfo">IPInfo</SelectItem>
							<SelectItem value="maxmind">MaxMind</SelectItem>
							<SelectItem value="db-ip">DB-IP</SelectItem>
						</SelectContent>
					</Select>
				</div>
				<div class="grid gap-2">
					<Label for="geo-cache">Cache duration (hours)</Label>
					<Input id="geo-cache" type="number" min={1} step={1} bind:value={cacheHours} />
				</div>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Include timezone</p>
						<p class="text-xs text-muted-foreground">Add timezone and offset metadata</p>
					</div>
					<Switch bind:checked={includeTimezone} />
				</label>
			</div>
			<label
				class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2"
			>
				<div>
					<p class="text-sm font-medium text-foreground">Render map preview</p>
					<p class="text-xs text-muted-foreground">Display static map with approximate location</p>
				</div>
				<Switch bind:checked={includeMap} />
			</label>
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
			<Button type="button" onclick={() => queue('queued')}>Queue lookup</Button>
		</CardFooter>
	</Card>

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Preview</CardTitle>
			<CardDescription
				>Example location card that will display once the lookup runs.</CardDescription
			>
		</CardHeader>
		<CardContent class="space-y-3">
			<div class="rounded-lg border border-border/60 bg-muted/30 p-4">
				<p class="text-sm font-medium text-foreground">Lisbon, Portugal</p>
				<p class="text-xs text-muted-foreground">38.7223° N, 9.1393° W · Timezone: Europe/Lisbon</p>
			</div>
			{#if includeMap}
				<div
					class="h-40 rounded-lg border border-border/60 bg-gradient-to-br from-sky-500/20 via-sky-500/10 to-transparent"
				></div>
			{/if}
		</CardContent>
	</Card>
</div>
