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

	type ConnectionRow = {
		local: string;
		remote: string;
		state: 'LISTENING' | 'ESTABLISHED' | 'CLOSE_WAIT';
		process: string;
	};

	const { client } = $props<{ client: Client }>();

	const tool = getClientTool('tcp-connections');

	let localFilter = $state('');
	let remoteFilter = $state('');
	let stateFilter = $state<'all' | ConnectionRow['state']>('all');
	let includeIpv6 = $state(false);
	let includeDnsLookup = $state(true);
	let log = $state<WorkspaceLogEntry[]>([]);

	const rows = $state<ConnectionRow[]>([
		{
			local: '10.0.0.4:443',
			remote: '52.94.76.2:52000',
			state: 'ESTABLISHED',
			process: 'nginx.exe'
		},
		{ local: '0.0.0.0:3389', remote: '—', state: 'LISTENING', process: 'svchost.exe' },
		{
			local: '10.0.0.4:5985',
			remote: '10.0.5.3:53021',
			state: 'CLOSE_WAIT',
			process: 'wmiprvse.exe'
		}
	]);

	const filteredRows = $derived(
		rows.filter((row) => {
			if (stateFilter !== 'all' && row.state !== stateFilter) return false;
			if (localFilter && !row.local.includes(localFilter)) return false;
			if (remoteFilter && !row.remote.includes(remoteFilter)) return false;
			if (!includeIpv6 && row.local.includes(':') && row.local.includes('[')) return false;
			return true;
		})
	);

	function queue(status: WorkspaceLogEntry['status']) {
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry(
				'Connection sweep staged',
				`local ${localFilter || '*'} · remote ${remoteFilter || '*'} · state ${stateFilter} · dns ${includeDnsLookup ? 'on' : 'off'}`,
				status
			)
		);
	}
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Filters</CardTitle>
			<CardDescription>Reduce the result set by address, port, or state.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-3">
				<div class="grid gap-2">
					<Label for="tcp-local">Local address filter</Label>
					<Input id="tcp-local" bind:value={localFilter} placeholder="10.0.0.4" />
				</div>
				<div class="grid gap-2">
					<Label for="tcp-remote">Remote address filter</Label>
					<Input id="tcp-remote" bind:value={remoteFilter} placeholder="52.94" />
				</div>
				<div class="grid gap-2">
					<Label for="tcp-state">Connection state</Label>
					<Select
						type="single"
						value={stateFilter}
						onValueChange={(value) => (stateFilter = value as typeof stateFilter)}
					>
						<SelectTrigger id="tcp-state" class="w-full">
							<span class="capitalize">{stateFilter}</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All</SelectItem>
							<SelectItem value="ESTABLISHED">Established</SelectItem>
							<SelectItem value="LISTENING">Listening</SelectItem>
							<SelectItem value="CLOSE_WAIT">Close Wait</SelectItem>
						</SelectContent>
					</Select>
				</div>
			</div>
			<div class="grid gap-4 md:grid-cols-2">
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Include IPv6</p>
						<p class="text-xs text-muted-foreground">Capture ::1 and v6-bound listeners</p>
					</div>
					<Switch bind:checked={includeIpv6} />
				</label>
				<label
					class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
				>
					<div>
						<p class="text-sm font-medium text-foreground">Resolve DNS</p>
						<p class="text-xs text-muted-foreground">Perform reverse lookup for remote IPs</p>
					</div>
					<Switch bind:checked={includeDnsLookup} />
				</label>
			</div>
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" variant="outline" onclick={() => queue('draft')}>Save filters</Button>
			<Button type="button" onclick={() => queue('queued')}>Poll connections</Button>
		</CardFooter>
	</Card>

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Simulated results</CardTitle>
			<CardDescription>Preview of how the telemetry table will appear.</CardDescription>
		</CardHeader>
		<CardContent class="overflow-hidden rounded-lg border border-border/60 text-sm">
			<table class="w-full divide-y divide-border/60">
				<thead class="bg-muted/30">
					<tr>
						<th class="px-4 py-2 text-left font-medium">Local</th>
						<th class="px-4 py-2 text-left font-medium">Remote</th>
						<th class="px-4 py-2 text-left font-medium">State</th>
						<th class="px-4 py-2 text-left font-medium">Process</th>
					</tr>
				</thead>
				<tbody>
					{#each filteredRows as row (row.local + row.remote)}
						<tr class="odd:bg-muted/20">
							<td class="px-4 py-2 font-mono">{row.local}</td>
							<td class="px-4 py-2 font-mono">{row.remote}</td>
							<td class="px-4 py-2 uppercase">{row.state}</td>
							<td class="px-4 py-2">{row.process}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</CardContent>
	</Card>
</div>
