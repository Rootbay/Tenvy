<script lang="ts">
	import { browser } from '$app/environment';
	import { goto, invalidateAll } from '$app/navigation';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import {
		Table,
		TableBody,
		TableCell,
		TableHead,
		TableHeader,
		TableRow
	} from '$lib/components/ui/table/index.js';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select/index.js';
	import ClientToolDialog from '$lib/components/client-tool-dialog.svelte';
	import ClientsTableRow from '$lib/components/clients/clients-table-row.svelte';
	import DeployAgentDialog from '$lib/components/clients/deploy-agent-dialog.svelte';
	import PingAgentDialog from '$lib/components/clients/ping-agent-dialog.svelte';
	import ShellCommandDialog from '$lib/components/clients/shell-command-dialog.svelte';
	import { sectionToolMap, type SectionKey } from '$lib/client-sections';
	import { createClientsTableStore } from '$lib/stores/clients-table';
	import {
		buildClientToolUrl,
		getClientTool,
		isDialogTool,
		type DialogToolId
	} from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { get } from 'svelte/store';
	import { onDestroy } from 'svelte';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Search from '@lucide/svelte/icons/search';
	import AlertCircle from '@lucide/svelte/icons/alert-circle';
	import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
	import type {
		AgentConnectionAction,
		AgentConnectionRequest,
		AgentSnapshot
	} from '../../../../../shared/types/agent';

	const statusLabels: Record<AgentSnapshot['status'], string> = {
		online: 'Online',
		offline: 'Offline',
		error: 'Error'
	};

	let { data } = $props<{ data: { agents: AgentSnapshot[] } }>();

	const clientsTable = createClientsTableStore(data.agents ?? []);

	$effect(() => {
		clientsTable.setAgents(data.agents ?? []);
	});

	const perPageOptions = [10, 25, 50];

	let toolDialog = $state<{ agentId: string; toolId: DialogToolId } | null>(null);
	let toolDialogAgent = $state<AgentSnapshot | null>(null);
	let toolDialogClient = $state<Client | null>(null);

	type PageAlertVariant = 'default' | 'destructive';

	type PageAlert = {
		title: string;
		description: string;
		variant: PageAlertVariant;
	};

	let connectionAlert = $state<PageAlert | null>(null);

	$effect(() => {
		if (toolDialog && !toolDialogAgent) {
			toolDialog = null;
		}
	});

	$effect(() => {
		const agentId = toolDialog?.agentId ?? null;
		const agents = $clientsTable.agents;
		const agent = agentId ? (agents.find((item) => item.id === agentId) ?? null) : null;
		toolDialogAgent = agent;
		toolDialogClient = agent ? mapAgentToClient(agent) : null;
	});

	let pingMessages = $state<Record<string, string>>({});
	let shellCommands = $state<Record<string, string>>({});
	let shellTimeouts = $state<Record<string, number | undefined>>({});
	let commandErrors = $state<Record<string, string | null>>({});
	let commandSuccess = $state<Record<string, string | null>>({});
	let commandPending = $state<Record<string, boolean>>({});

	let pingDialogAgentId = $state<string | null>(null);
	let shellDialogAgentId = $state<string | null>(null);
	let pingAgent = $state<AgentSnapshot | null>(null);
	let shellAgent = $state<AgentSnapshot | null>(null);
	let deployDialogOpen = $state(false);

	$effect(() => {
		const agentId = pingDialogAgentId;
		const agents = $clientsTable.agents;
		pingAgent = agentId ? (agents.find((agent) => agent.id === agentId) ?? null) : null;
	});

	$effect(() => {
		const agentId = shellDialogAgentId;
		const agents = $clientsTable.agents;
		shellAgent = agentId ? (agents.find((agent) => agent.id === agentId) ?? null) : null;
	});

	type CopyFeedback = { message: string; variant: 'success' | 'error' } | null;
	let copyFeedback = $state<CopyFeedback>(null);
	let copyFeedbackTimeout: number | undefined;
	let connectionAlertTimeout: number | undefined;

	function updateRecord<T>(records: Record<string, T>, key: string, value: T): Record<string, T> {
		return { ...records, [key]: value };
	}

	function hasActiveConnection(agent: AgentSnapshot): boolean {
		return agent.status === 'online';
	}

	function showConnectionAlert(alert: PageAlert) {
		connectionAlert = alert;

		if (!browser) {
			return;
		}

		if (connectionAlertTimeout !== undefined) {
			window.clearTimeout(connectionAlertTimeout);
		}

		connectionAlertTimeout = window.setTimeout(() => {
			connectionAlert = null;
			connectionAlertTimeout = undefined;
		}, 4000);
	}

	function formatDate(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) {
			return 'Unknown';
		}
		return date.toLocaleString();
	}

	function formatRelative(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) {
			return 'unknown';
		}
		const diffMs = Date.now() - date.getTime();
		if (diffMs <= 0) {
			return 'just now';
		}
		const diffSeconds = Math.floor(diffMs / 1000);
		const units: [Intl.RelativeTimeFormatUnit, number][] = [
			['day', 86_400],
			['hour', 3_600],
			['minute', 60],
			['second', 1]
		];
		for (const [unit, seconds] of units) {
			if (diffSeconds >= seconds || unit === 'second') {
				const value = Math.floor(diffSeconds / seconds) * -1;
				return new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' }).format(value, unit);
			}
		}
		return 'just now';
	}

	type MetadataLocation = AgentSnapshot['metadata']['location'];

	function countryCodeToFlag(code: string | null | undefined): string {
		if (!code) {
			return '🌐';
		}
		const normalized = code.trim().toUpperCase();
		if (normalized.length !== 2) {
			return '🌐';
		}

		const codePoints = Array.from(normalized).map((char) => 0x1f1e6 + char.charCodeAt(0) - 65);
		if (codePoints.some((point) => Number.isNaN(point))) {
			return '🌐';
		}

		return String.fromCodePoint(...codePoints);
	}

	function getLocationDisplay(location: MetadataLocation): { label: string; flag: string } {
		const fallback = { label: 'Unknown', flag: '🌐' } as const;

		if (!location) {
			return fallback;
		}

		const label =
			location.source?.trim() ??
			[location.city, location.region, location.country]
				.map((part) => part?.trim())
				.filter((part): part is string => Boolean(part && part.length > 0))
				.join(', ');

		const normalizedCountryCode = location.countryCode?.trim().toUpperCase();
		const codeCandidate =
			normalizedCountryCode && normalizedCountryCode.length === 2
				? normalizedCountryCode
				: location.country && location.country.trim().length === 2
					? location.country.trim().toUpperCase()
					: undefined;
		const flag = countryCodeToFlag(codeCandidate ?? null);

		return { label: label || fallback.label, flag: flag || fallback.flag };
	}

	function getAgentLocation(agent: AgentSnapshot): { label: string; flag: string } {
		return getLocationDisplay(agent.metadata.location);
	}

	function getAgentGroup(agent: AgentSnapshot): string {
		return agent.metadata.group?.trim() || '—';
	}

	function formatPing(agent: AgentSnapshot): string {
		const ping = agent.metrics?.pingMs ?? agent.metrics?.latencyMs;
		if (typeof ping === 'number' && Number.isFinite(ping) && ping >= 0) {
			return `${Math.round(ping)} ms`;
		}
		return '—';
	}

	function findAgentById(agentId: string | null): AgentSnapshot | null {
		if (!agentId) {
			return null;
		}
		const { agents } = get(clientsTable);
		return agents.find((agent) => agent.id === agentId) ?? null;
	}

	function getError(key: string): string | null {
		return commandErrors[key] ?? null;
	}

	function getSuccess(key: string): string | null {
		return commandSuccess[key] ?? null;
	}

	function isPending(key: string): boolean {
		return commandPending[key] ?? false;
	}

	async function queueCommand(
		agentId: string,
		body: unknown,
		key: string,
		successMessage: string
	): Promise<boolean> {
		commandPending = updateRecord(commandPending, key, true);
		commandErrors = updateRecord(commandErrors, key, null);
		commandSuccess = updateRecord(commandSuccess, key, null);

		try {
			const response = await fetch(`/api/agents/${agentId}/commands`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(body)
			});
			if (!response.ok) {
				const message = (await response.text()) || 'Failed to queue command';
				commandErrors = updateRecord(commandErrors, key, message.trim());
				return false;
			}
			commandSuccess = updateRecord(commandSuccess, key, successMessage);
			await invalidateAll();
			return true;
		} catch (err) {
			commandErrors = updateRecord(
				commandErrors,
				key,
				err instanceof Error ? err.message : 'Unknown error'
			);
			return false;
		} finally {
			commandPending = updateRecord(commandPending, key, false);
		}
	}

	async function requestConnectionAction(
		agent: AgentSnapshot,
		action: AgentConnectionAction
	): Promise<boolean> {
		const agentLabel = agent.metadata.hostname?.trim() || agent.id;

		try {
			const response = await fetch(`/api/agents/${agent.id}/connection`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ action } satisfies AgentConnectionRequest)
			});

			if (!response.ok) {
				const message = (await response.text()) || 'Unable to update connection';
				showConnectionAlert({
					title: 'Connection request failed',
					description: message.trim(),
					variant: 'destructive'
				});
				return false;
			}

			await invalidateAll();

			const titles: Record<AgentConnectionAction, string> = {
				disconnect: 'Agent disconnected',
				reconnect: 'Agent reconnected'
			};

			const descriptions: Record<AgentConnectionAction, string> = {
				disconnect: `${agentLabel} is now disconnected from the controller.`,
				reconnect: `${agentLabel} has been reconnected to the controller.`
			};

			showConnectionAlert({
				title: titles[action],
				description: descriptions[action],
				variant: 'default'
			});

			return true;
		} catch (err) {
			showConnectionAlert({
				title: 'Connection request failed',
				description: err instanceof Error ? err.message : 'Unable to update connection',
				variant: 'destructive'
			});
			return false;
		}
	}

	async function sendPing(agentId: string): Promise<boolean> {
		const key = `ping:${agentId}`;
		const message = pingMessages[agentId]?.trim();
		const success = await queueCommand(
			agentId,
			{
				name: 'ping',
				payload: message ? { message } : {}
			},
			key,
			'Ping queued'
		);
		if (success) {
			pingMessages = updateRecord(pingMessages, agentId, '');
		}
		return success;
	}

	async function sendShell(agentId: string): Promise<boolean> {
		const key = `shell:${agentId}`;
		const command = shellCommands[agentId]?.trim();
		if (!command) {
			commandErrors = updateRecord(commandErrors, key, 'Command is required');
			return false;
		}

		const timeout = shellTimeouts[agentId];
		const payload: { command: string; timeoutSeconds?: number } = { command };
		if (timeout && timeout > 0) {
			payload.timeoutSeconds = timeout;
		}

		const success = await queueCommand(
			agentId,
			{
				name: 'shell',
				payload
			},
			key,
			'Shell command queued'
		);

		if (success) {
			shellCommands = updateRecord(shellCommands, agentId, '');
		}

		return success;
	}

	function openPingDialog(agentId: string) {
		const agent = findAgentById(agentId);
		if (!agent) {
			return;
		}

		if (!hasActiveConnection(agent)) {
			const agentLabel = agent.metadata.hostname?.trim() || agent.id;
			showConnectionAlert({
				title: 'Connection unavailable',
				description: `${agentLabel} is not currently connected. Re-establish the session to queue a ping.`,
				variant: 'destructive'
			});
			return;
		}

		pingDialogAgentId = agentId;
		commandErrors = updateRecord(commandErrors, `ping:${agentId}`, null);
		commandSuccess = updateRecord(commandSuccess, `ping:${agentId}`, null);
	}

	function sendCommandError(agent: AgentSnapshot, action: string) {
		const agentLabel = agent.metadata.hostname?.trim() || agent.id;
		showConnectionAlert({
			title: 'Connection unavailable',
			description: `${agentLabel} is not currently connected. Re-establish the session to ${action}.`,
			variant: 'destructive'
		});
	}

	async function disconnectAgent(agent: AgentSnapshot) {
		if (!hasActiveConnection(agent)) {
			sendCommandError(agent, 'disconnect the agent');
			return;
		}

		await requestConnectionAction(agent, 'disconnect');
	}

	async function reconnectAgent(agent: AgentSnapshot) {
		await requestConnectionAction(agent, 'reconnect');
	}

	function openShellDialog(agentId: string) {
		const agent = findAgentById(agentId);
		if (!agent) {
			return;
		}

		if (!hasActiveConnection(agent)) {
			sendCommandError(agent, 'run shell commands');
			return;
		}

		shellDialogAgentId = agentId;
		commandErrors = updateRecord(commandErrors, `shell:${agentId}`, null);
		commandSuccess = updateRecord(commandSuccess, `shell:${agentId}`, null);
	}

	function closePingDialog() {
		pingDialogAgentId = null;
	}

	function closeShellDialog() {
		shellDialogAgentId = null;
	}

	async function handlePingSubmit() {
		const agent = findAgentById(pingDialogAgentId);
		if (!agent) return;
		const success = await sendPing(agent.id);
		if (success) {
			closePingDialog();
		}
	}

	async function handleShellSubmit() {
		const agent = findAgentById(shellDialogAgentId);
		if (!agent) return;
		const success = await sendShell(agent.id);
		if (success) {
			closeShellDialog();
		}
	}

	function setCopyFeedback(feedback: CopyFeedback) {
		copyFeedback = feedback;
		if (!browser) {
			return;
		}
		if (copyFeedbackTimeout !== undefined) {
			window.clearTimeout(copyFeedbackTimeout);
		}
		if (feedback) {
			copyFeedbackTimeout = window.setTimeout(() => {
				copyFeedback = null;
				copyFeedbackTimeout = undefined;
			}, 2500);
		}
	}

	async function copyAgentId(agentId: string) {
		if (!browser) return;
		try {
			await navigator.clipboard.writeText(agentId);
			setCopyFeedback({ message: 'Agent ID copied to clipboard', variant: 'success' });
		} catch (err) {
			console.error(err);
			setCopyFeedback({ message: 'Unable to copy agent ID', variant: 'error' });
		}
	}

	onDestroy(() => {
		if (!browser) {
			return;
		}

		if (copyFeedbackTimeout !== undefined) {
			window.clearTimeout(copyFeedbackTimeout);
			copyFeedbackTimeout = undefined;
		}

		if (connectionAlertTimeout !== undefined) {
			window.clearTimeout(connectionAlertTimeout);
			connectionAlertTimeout = undefined;
		}
	});

	function inferClientPlatform(os: string): Client['platform'] {
		const normalized = os.toLowerCase();
		if (normalized.includes('mac')) {
			return 'macos';
		}
		if (normalized.includes('win')) {
			return 'windows';
		}
		return 'linux';
	}

	function mapAgentStatusToClientStatus(status: AgentSnapshot['status']): Client['status'] {
		if (status === 'online') {
			return 'online';
		}
		if (status === 'offline') {
			return 'offline';
		}
		return 'idle';
	}

	function determineClientRisk(status: AgentSnapshot['status']): Client['risk'] {
		return status === 'error' ? 'High' : 'Medium';
	}

	function mapAgentToClient(agent: AgentSnapshot): Client {
		const tags = agent.metadata.tags ?? [];

		return {
			id: agent.id,
			codename: agent.metadata.hostname?.toUpperCase() ?? agent.id.toUpperCase(),
			hostname: agent.metadata.hostname,
			ip: agent.metadata.ipAddress ?? 'Unknown',
			location: getAgentLocation(agent).label,
			os: agent.metadata.os,
			platform: inferClientPlatform(agent.metadata.os),
			version: agent.metadata.version ?? 'Unknown',
			status: mapAgentStatusToClientStatus(agent.status),
			lastSeen: formatRelative(agent.lastSeen),
			tags,
			risk: determineClientRisk(agent.status),
			notes: tags.length > 0 ? `Tags: ${tags.join(', ')}` : undefined
		};
	}

	function openSection(section: SectionKey, agent: AgentSnapshot) {
		const toolId = sectionToolMap[section];
		if (!toolId) {
			return;
		}

		if (toolId === 'reconnect') {
			toolDialog = null;
			void reconnectAgent(agent);
			return;
		}

		if (toolId === 'disconnect') {
			toolDialog = null;
			void disconnectAgent(agent);
			return;
		}

		if (!hasActiveConnection(agent)) {
			const agentLabel = agent.metadata.hostname?.trim() || agent.id;
			showConnectionAlert({
				title: 'Connection unavailable',
				description: `${agentLabel} is not currently connected. Re-establish the session to access agent tools.`,
				variant: 'destructive'
			});
			return;
		}

		const tool = getClientTool(toolId);
		const target = tool.target ?? '_blank';

		toolDialog = null;

		if (target === 'dialog' && isDialogTool(toolId)) {
			toolDialog = { agentId: agent.id, toolId };
			return;
		}

		const url = buildClientToolUrl(agent.id, tool);

		if (!browser) {
			return;
		}

		if (target === '_self') {
			goto(url);
			return;
		}

		if (target === '_blank') {
			const newWindow = window.open(url, '_blank', 'noopener');

			if (!newWindow) {
				console.warn('Pop-up blocked when attempting to open client tool in a new tab.');
				return;
			}

			newWindow.opener = null;
			newWindow.focus?.();
			return;
		}

		window.open(url, target, 'noopener');
	}
</script>

<svelte:head>
	<title>Clients · Tenvy</title>
</svelte:head>

<section class="space-y-6">
	{#if $clientsTable.agents.length === 0}
		<Alert class="border-dashed border-border/60">
			<AlertCircle class="h-4 w-4" />
			<AlertTitle>No agents connected</AlertTitle>
			<AlertDescription>
				Launch a client instance to have it register and appear here automatically.
			</AlertDescription>
			<div class="col-start-2 mt-3">
				<Button type="button" onclick={() => (deployDialogOpen = true)}>
					View deployment guide
				</Button>
			</div>
		</Alert>
	{/if}

	<div class="space-y-4 rounded-lg border border-border/60 bg-background/60 p-4">
		<div class="grid gap-4 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)] lg:items-end">
			<div class="flex flex-col gap-2">
				<Label for="client-search" class="text-sm font-medium">Search</Label>
				<div class="relative">
					<Search
						class="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
					/>
					<Input
						id="client-search"
						placeholder="Hostname, user, ID, IP, or tag"
						value={$clientsTable.searchQuery}
						oninput={(event) => clientsTable.setSearchQuery(event.currentTarget.value)}
						class="pl-10"
					/>
				</div>
			</div>
			<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				<div class="flex flex-col gap-2">
					<Label for="client-status-filter" class="text-sm font-medium">Status</Label>
					<Select
						type="single"
						value={$clientsTable.statusFilter}
						onValueChange={(value) =>
							clientsTable.setStatusFilter(value as 'all' | AgentSnapshot['status'])}
					>
						<SelectTrigger id="client-status-filter" class="w-full">
							<span class="truncate">
								{$clientsTable.statusFilter === 'all'
									? 'All statuses'
									: statusLabels[$clientsTable.statusFilter]}
							</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All statuses</SelectItem>
							<SelectItem value="online">Online</SelectItem>
							<SelectItem value="offline">Offline</SelectItem>
							<SelectItem value="error">Error</SelectItem>
						</SelectContent>
					</Select>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="client-tag-filter" class="text-sm font-medium">Tag</Label>
					<Select
						type="single"
						value={$clientsTable.tagFilter}
						onValueChange={(value) => clientsTable.setTagFilter(value === 'all' ? 'all' : value)}
					>
						<SelectTrigger id="client-tag-filter" class="w-full">
							<span class="truncate">
								{$clientsTable.tagFilter === 'all' ? 'All tags' : $clientsTable.tagFilter}
							</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All tags</SelectItem>
							{#if $clientsTable.availableTags.length > 0}
								{#each $clientsTable.availableTags as tag (tag)}
									<SelectItem value={tag}>{tag}</SelectItem>
								{/each}
							{:else}
								<SelectItem value="all" disabled>No tags available</SelectItem>
							{/if}
						</SelectContent>
					</Select>
				</div>
				<div class="flex flex-col gap-2">
					<Label for="client-page-size" class="text-sm font-medium">Per page</Label>
					<Select
						type="single"
						value={$clientsTable.perPage.toString()}
						onValueChange={(value) => {
							const next = Number.parseInt(value, 10);
							clientsTable.setPerPage(Number.isNaN(next) ? perPageOptions[0] : next);
						}}
					>
						<SelectTrigger id="client-page-size" class="w-full">
							<span>{$clientsTable.perPage} rows</span>
						</SelectTrigger>
						<SelectContent>
							{#each perPageOptions as option (option)}
								<SelectItem value={option.toString()}>{option} rows</SelectItem>
							{/each}
						</SelectContent>
					</Select>
				</div>
			</div>
		</div>
		{#if connectionAlert}
			<Alert variant={connectionAlert.variant}>
				<AlertCircle class="h-4 w-4" />
				<AlertTitle>{connectionAlert.title}</AlertTitle>
				<AlertDescription>{connectionAlert.description}</AlertDescription>
			</Alert>
		{/if}

		{#if copyFeedback}
			<Alert
				variant={copyFeedback.variant === 'error' ? 'destructive' : 'default'}
				class={copyFeedback.variant === 'success'
					? 'border-emerald-500/40 bg-emerald-500/10 text-emerald-500'
					: undefined}
			>
				{#if copyFeedback.variant === 'error'}
					<AlertCircle class="h-4 w-4" />
					<AlertTitle>Copy failed</AlertTitle>
				{:else}
					<CheckCircle2 class="h-4 w-4" />
					<AlertTitle>Copied</AlertTitle>
				{/if}
				<AlertDescription>{copyFeedback.message}</AlertDescription>
			</Alert>
		{/if}
	</div>

	<div class="space-y-4">
		<ScrollArea class="rounded-lg border border-border/60">
			<div class="min-w-[960px]">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead class="w-[22rem]">Location</TableHead>
							<TableHead class="w-[8rem]">IP</TableHead>
							<TableHead class="w-[12rem]">Username</TableHead>
							<TableHead class="w-[12rem]">Group</TableHead>
							<TableHead class="w-[7rem] text-center">OS</TableHead>
							<TableHead class="w-[10rem]">Ping</TableHead>
							<TableHead class="w-[9rem]">Version</TableHead>
							<TableHead class="w-[9rem]">Date</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{#if $clientsTable.paginatedAgents.length === 0}
							<TableRow>
								<TableCell colspan={8} class="py-12 text-center text-sm text-muted-foreground">
									{#if $clientsTable.agents.length === 0}
										No agents connected yet.
									{:else}
										No agents match your current filters.
									{/if}
								</TableCell>
							</TableRow>
						{:else}
							{#each $clientsTable.paginatedAgents as agent (agent.id)}
								<ClientsTableRow
									{agent}
									{formatDate}
									{formatPing}
									{getAgentGroup}
									{getAgentLocation}
									{openSection}
									{copyAgentId}
								/>
							{/each}
						{/if}
					</TableBody>
				</Table>
			</div>
		</ScrollArea>

		<div class="px-4 text-sm text-muted-foreground">
			{#if $clientsTable.filteredAgents.length === 0}
				No agents to display.
			{:else}
				Showing {$clientsTable.pageRange.start}–{$clientsTable.pageRange.end} of {$clientsTable
					.filteredAgents.length}
				{$clientsTable.filteredAgents.length === 1 ? ' agent' : ' agents'}.
			{/if}
		</div>

		{#if $clientsTable.filteredAgents.length > 0 && $clientsTable.totalPages > 1}
			<nav class="flex justify-center">
				<ul class="flex flex-row items-center gap-1">
					<li>
						<Button
							type="button"
							variant="ghost"
							class="flex h-9 items-center gap-1 px-2.5 sm:pl-2.5"
							onclick={() => clientsTable.previousPage()}
							disabled={$clientsTable.currentPage <= 1}
						>
							<ChevronLeft class="size-4" />
							<span class="sr-only sm:not-sr-only">Previous</span>
						</Button>
					</li>
					{#each $clientsTable.paginationItems as item}
						<li>
							{#if item === 'ellipsis'}
								<span class="flex h-9 w-9 items-center justify-center text-sm text-muted-foreground"
									>…</span
								>
							{:else}
								<Button
									type="button"
									variant={item === $clientsTable.currentPage ? 'outline' : 'ghost'}
									class="h-9 w-9 px-0"
									onclick={() => clientsTable.goToPage(item)}
								>
									{item}
								</Button>
							{/if}
						</li>
					{/each}
					<li>
						<Button
							type="button"
							variant="ghost"
							class="flex h-9 items-center gap-1 px-2.5 sm:pl-2.5"
							onclick={() => clientsTable.nextPage()}
							disabled={$clientsTable.currentPage >= $clientsTable.totalPages}
						>
							<span class="sr-only sm:not-sr-only">Next</span>
							<ChevronRight class="size-4" />
						</Button>
					</li>
				</ul>
			</nav>
		{/if}
	</div>

	{#if toolDialog && toolDialogAgent && toolDialogClient}
		{#key `${toolDialog.agentId}-${toolDialog.toolId}`}
			<ClientToolDialog
				client={toolDialogClient}
				toolId={toolDialog.toolId}
				on:close={() => (toolDialog = null)}
			/>
		{/key}
	{/if}

	<DeployAgentDialog open={deployDialogOpen} on:close={() => (deployDialogOpen = false)} />

	{#if pingAgent}
		<PingAgentDialog
			agent={pingAgent}
			message={pingMessages[pingAgent!.id] ?? ''}
			error={getError(`ping:${pingAgent!.id}`)}
			success={getSuccess(`ping:${pingAgent!.id}`)}
			pending={isPending(`ping:${pingAgent!.id}`)}
			on:close={closePingDialog}
			on:submit={handlePingSubmit}
			on:messageChange={(event) =>
				(pingMessages = updateRecord(pingMessages, pingAgent!.id, event.detail))}
		/>
	{/if}

	{#if shellAgent}
		<ShellCommandDialog
			agent={shellAgent}
			command={shellCommands[shellAgent!.id] ?? ''}
			timeout={shellTimeouts[shellAgent!.id]}
			error={getError(`shell:${shellAgent!.id}`)}
			success={getSuccess(`shell:${shellAgent!.id}`)}
			pending={isPending(`shell:${shellAgent!.id}`)}
			on:close={closeShellDialog}
			on:submit={handleShellSubmit}
			on:commandChange={(event) =>
				(shellCommands = updateRecord(shellCommands, shellAgent!.id, event.detail))}
			on:timeoutChange={(event) =>
				(shellTimeouts = updateRecord(shellTimeouts, shellAgent!.id, event.detail))}
		/>
	{/if}
</section>
