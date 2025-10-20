<script lang="ts">
	import { onMount } from 'svelte';
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
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import type { DialogToolId } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import { notifyToolActivationCommand } from '$lib/utils/agent-commands.js';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';
	import type {
		TcpConnectionEndpoint,
		TcpConnectionEntry,
		TcpConnectionSnapshot,
		TcpConnectionState
	} from '$lib/types/tcp-connections';

	interface TcpConnectionsStateResponse {
		snapshot: TcpConnectionSnapshot | null;
	}

	const {
		client,
		toolId = 'system-monitor',
		panel = 'network'
	} = $props<{ client: Client; toolId?: DialogToolId; panel?: 'network' }>();

	const stateOptions: { label: string; value: 'all' | TcpConnectionState }[] = [
		{ label: 'All states', value: 'all' },
		{ label: 'Established', value: 'ESTABLISHED' },
		{ label: 'Listening', value: 'LISTENING' },
		{ label: 'Close wait', value: 'CLOSE_WAIT' },
		{ label: 'Syn sent', value: 'SYN_SENT' },
		{ label: 'Syn received', value: 'SYN_RECEIVED' },
		{ label: 'Fin wait 1', value: 'FIN_WAIT_1' },
		{ label: 'Fin wait 2', value: 'FIN_WAIT_2' },
		{ label: 'Time wait', value: 'TIME_WAIT' },
		{ label: 'Last ack', value: 'LAST_ACK' },
		{ label: 'Closing', value: 'CLOSING' },
		{ label: 'Bound', value: 'BOUND' },
		{ label: 'Closed', value: 'CLOSED' },
		{ label: 'Unknown', value: 'UNKNOWN' }
	];

	const numberFormatter = new Intl.NumberFormat();
	const timestampFormatter = new Intl.DateTimeFormat(undefined, {
		dateStyle: 'medium',
		timeStyle: 'medium'
	});

	let localFilter = $state('');
	let remoteFilter = $state('');
	let stateFilter = $state<'all' | TcpConnectionState>('all');
	let includeIpv6 = $state(false);
	let includeDnsLookup = $state(true);
	let log = $state<WorkspaceLogEntry[]>([]);
	let snapshot = $state<TcpConnectionSnapshot | null>(null);
	let loading = $state(false);
	let refreshing = $state(false);
	let errorMessage = $state<string | null>(null);

	const rows = $derived(snapshot?.connections ?? []);
	const lastUpdated = $derived(snapshot?.capturedAt ?? null);

	function describeFilters(): string {
		return `local ${localFilter || '*'} · remote ${remoteFilter || '*'} · state ${stateFilter} · dns ${
			includeDnsLookup ? 'on' : 'off'
		} · ipv6 ${includeIpv6 ? 'on' : 'off'}`;
	}

	function recordLog(status: WorkspaceLogEntry['status'], detail: string) {
		log = appendWorkspaceLog(log, createWorkspaceLogEntry('TCP sweep', detail, status));
		notifyToolActivationCommand(client.id, toolId, {
			action: 'event:tcp-scan',
			metadata: { detail, status, panel }
		});
	}

	function buildQuery() {
		const query: Record<string, unknown> = {};
		const trimmedLocal = localFilter.trim();
		const trimmedRemote = remoteFilter.trim();
		if (trimmedLocal) query.localFilter = trimmedLocal;
		if (trimmedRemote) query.remoteFilter = trimmedRemote;
		if (stateFilter !== 'all') query.state = stateFilter;
		query.includeIpv6 = includeIpv6;
		query.resolveDns = includeDnsLookup;
		return query;
	}

	function formatTimestamp(value: string | null): string {
		if (!value) {
			return 'Never';
		}
		try {
			return timestampFormatter.format(new Date(value));
		} catch (err) {
			console.error('Failed to format timestamp', err);
			return value;
		}
	}

	function formatEndpoint(endpoint?: TcpConnectionEndpoint | null): string {
		if (!endpoint) {
			return '—';
		}
		if (endpoint.host && endpoint.host !== endpoint.address) {
			return `${endpoint.host}\n${endpoint.label ?? endpoint.address}`;
		}
		return endpoint.label ?? endpoint.address ?? '—';
	}

	function formatProcess(entry: TcpConnectionEntry): { label: string; hint: string } {
		if (!entry.process) {
			return { label: '—', hint: '' };
		}
		const parts: string[] = [];
		if (entry.process.name) parts.push(entry.process.name);
		if (entry.process.pid && entry.process.pid > 0) parts.push(`PID ${entry.process.pid}`);
		if (entry.process.username) parts.push(entry.process.username);
		const command = entry.process.commandLine ?? '';
		const executable = entry.process.executable ?? '';
		const hint = [executable, command].filter(Boolean).join('\n');
		if (parts.length === 0) {
			return {
				label: entry.process.pid && entry.process.pid > 0 ? `PID ${entry.process.pid}` : '—',
				hint
			};
		}
		return { label: parts.join(' · '), hint };
	}

	function formatState(state: TcpConnectionState): string {
		return state
			.replace(/_/g, ' ')
			.toLowerCase()
			.replace(/(^|\s)\S/g, (segment: string) => segment.toUpperCase());
	}

	async function loadSnapshot(options: { silent?: boolean } = {}) {
		if (!options.silent) {
			loading = true;
			errorMessage = null;
		}
		try {
			const response = await fetch(`/api/agents/${client.id}/tcp-connections`);
			if (!response.ok) {
				const detail = await response.text().catch(() => '');
				throw new Error(detail || `Request failed with status ${response.status}`);
			}
			const payload = (await response.json()) as TcpConnectionsStateResponse;
			snapshot = payload.snapshot ?? null;
		} catch (err) {
			errorMessage = (err as Error).message || 'Failed to load TCP connections';
		} finally {
			if (!options.silent) {
				loading = false;
			}
		}
	}

	async function refreshConnections() {
		refreshing = true;
		errorMessage = null;
		const detail = describeFilters();
		recordLog('queued', detail);
		try {
			const response = await fetch(`/api/agents/${client.id}/tcp-connections`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					action: 'refresh',
					waitMs: 12_000,
					query: buildQuery()
				})
			});
			if (!response.ok) {
				const message = await response.text().catch(() => '');
				throw new Error(message || `Sweep failed with status ${response.status}`);
			}
			const payload = (await response.json()) as TcpConnectionsStateResponse;
			snapshot = payload.snapshot ?? null;
			const count = payload.snapshot?.connections?.length ?? 0;
			recordLog('complete', `${detail} · captured ${numberFormatter.format(count)} sockets`);
		} catch (err) {
			const message = (err as Error).message || 'Failed to poll TCP connections';
			errorMessage = message;
			recordLog('failed', `${detail} · ${message}`);
		} finally {
			refreshing = false;
		}
	}

	onMount(() => {
		void loadSnapshot();
	});
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
							{#each stateOptions as option (option.value)}
								<SelectItem value={option.value}>{option.label}</SelectItem>
							{/each}
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
			<Button type="button" variant="outline" onclick={() => recordLog('draft', describeFilters())}
				>Save filters</Button
			>
			<Button type="button" onclick={refreshConnections} disabled={refreshing}>
				{refreshing ? 'Polling…' : 'Poll connections'}
			</Button>
		</CardFooter>
	</Card>

	{#if errorMessage}
		<Alert variant="destructive">
			<AlertTitle>Request failed</AlertTitle>
			<AlertDescription>{errorMessage}</AlertDescription>
		</Alert>
	{/if}

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">{tool?.title ?? 'TCP Connections'}</CardTitle>
			<CardDescription>
				{tool?.description ??
					'Live inventory of socket telemetry associated with running processes.'}
			</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4">
			<div class="flex flex-wrap items-center justify-between gap-3 text-sm text-muted-foreground">
				<span>Last updated: {formatTimestamp(lastUpdated)}</span>
				{#if snapshot}
					<span>
						Showing {numberFormatter.format(rows.length)}
						{#if snapshot.truncated}
							of {numberFormatter.format(snapshot.total)} (truncated)
						{:else if snapshot.total !== rows.length}
							of {numberFormatter.format(snapshot.total)}
						{/if}
					</span>
				{/if}
			</div>
			<div class="overflow-hidden rounded-lg border border-border/60 text-sm">
				{#if loading}
					<div class="px-4 py-6 text-center text-muted-foreground">
						Loading connection snapshot…
					</div>
				{:else if rows.length === 0}
					<div class="px-4 py-6 text-center text-muted-foreground">
						No TCP connections have been captured for this client yet.
					</div>
				{:else}
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
							{#each rows as row (row.id)}
								{@const processInfo = formatProcess(row)}
								<tr class="odd:bg-muted/20">
									<td class="px-4 py-2 font-mono whitespace-pre-wrap"
										>{formatEndpoint(row.local)}</td
									>
									<td class="px-4 py-2 font-mono whitespace-pre-wrap"
										>{formatEndpoint(row.remote)}</td
									>
									<td class="px-4 py-2 uppercase">{formatState(row.state)}</td>
									<td class="px-4 py-2" title={processInfo.hint}>{processInfo.label}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</div>
		</CardContent>
	</Card>
</div>
