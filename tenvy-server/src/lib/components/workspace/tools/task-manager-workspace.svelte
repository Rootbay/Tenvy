<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
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
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';
	import { splitCommandLine } from '$lib/utils/command';
	import { cn } from '$lib/utils.js';
	import type {
		ProcessActionRequest,
		ProcessDetail,
		ProcessListResponse,
		ProcessStatus,
		ProcessSummary
	} from '$lib/types/task-manager';

	type SortKey = 'cpu' | 'memory' | 'name' | 'pid';
	type SortDirection = 'asc' | 'desc';

	const { client } = $props<{ client: Client }>();

	const tool = getClientTool('task-manager');

	let processes = $state<ProcessSummary[]>([]);
	let lastUpdated = $state<string | null>(null);
	let loading = $state(false);
	let errorMessage = $state<string | null>(null);
	let search = $state('');
	let statusFilter = $state<'all' | ProcessStatus>('all');
	let sortKey = $state<SortKey>('cpu');
	let sortDirection = $state<SortDirection>('desc');
	let autoRefresh = $state(true);
	let sampleInterval = $state(15);
	let log = $state<WorkspaceLogEntry[]>([]);
	let selectedPid = $state<number | null>(null);
	let selectedDetail = $state<ProcessDetail | null>(null);
	let detailLoading = $state(false);
	let detailError = $state<string | null>(null);
	let startCommand = $state('');
	let startArgs = $state('');
	let startCwd = $state('');
	let startEnv = $state('');
	let starting = $state(false);
	let startError = $state<string | null>(null);
	let actionInProgress = $state<number | null>(null);

	let refreshTimer: ReturnType<typeof setInterval> | null = null;

	const statusOptions: { label: string; value: 'all' | ProcessStatus }[] = [
		{ label: 'All statuses', value: 'all' },
		{ label: 'Running', value: 'running' },
		{ label: 'Sleeping', value: 'sleeping' },
		{ label: 'Stopped', value: 'stopped' },
		{ label: 'Idle', value: 'idle' },
		{ label: 'Suspended', value: 'suspended' },
		{ label: 'Zombie', value: 'zombie' }
	];

	const numberFormatter = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });
	const memoryFormatter = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });
	const dateFormatter = new Intl.DateTimeFormat(undefined, {
		dateStyle: 'medium',
		timeStyle: 'short'
	});

	function recordLog(
		action: string,
		detail: string,
		status: WorkspaceLogEntry['status'] = 'queued'
	) {
		log = appendWorkspaceLog(log, createWorkspaceLogEntry(action, detail, status));
	}

	function formatTimestamp(value: string | null): string {
		if (!value) {
			return '—';
		}
		try {
			return dateFormatter.format(new Date(value));
		} catch {
			return value;
		}
	}

	function formatCpu(value: number): string {
		return `${numberFormatter.format(value)}%`;
	}

	function formatMemory(bytes: number): string {
		if (!Number.isFinite(bytes) || bytes <= 0) {
			return '0 MB';
		}
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let value = bytes;
		let unitIndex = 0;
		while (value >= 1024 && unitIndex < units.length - 1) {
			value /= 1024;
			unitIndex += 1;
		}
		const formatted = unitIndex <= 1 ? Math.round(value).toString() : memoryFormatter.format(value);
		return `${formatted} ${units[unitIndex]}`;
	}

	function statusBadgeVariant(
		status: ProcessStatus
	): 'secondary' | 'outline' | 'default' | 'destructive' {
		switch (status) {
			case 'running':
				return 'secondary';
			case 'sleeping':
			case 'idle':
				return 'outline';
			case 'suspended':
			case 'stopped':
				return 'default';
			case 'zombie':
				return 'destructive';
			default:
				return 'outline';
		}
	}

	function describeProcess(process: ProcessSummary | ProcessDetail): string {
		const segments = [`PID ${process.pid}`, process.name];
		if (process.command && process.command !== process.name) {
			segments.push(process.command);
		}
		return segments.join(' · ');
	}

	function parseEnvironment(input: string): Record<string, string> | undefined {
		const entries = input
			.split(/\r?\n/)
			.map((line) => line.trim())
			.filter((line) => line && !line.startsWith('#'))
			.map((line) => {
				const equalsIndex = line.indexOf('=');
				if (equalsIndex === -1) {
					return null;
				}
				const key = line.slice(0, equalsIndex).trim();
				const value = line.slice(equalsIndex + 1).trim();
				if (!key) {
					return null;
				}
				return [key, value] as const;
			})
			.filter((entry): entry is readonly [string, string] => entry !== null);
		if (entries.length === 0) {
			return undefined;
		}
		return Object.fromEntries(entries);
	}

	async function loadProcesses(options: { silent?: boolean } = {}) {
		if (!options.silent) {
			loading = true;
			errorMessage = null;
		}
		try {
			const response = await fetch('/api/task-manager/processes');
			if (!response.ok) {
				const detail = await response.text().catch(() => '');
				throw new Error(detail || `Request failed with status ${response.status}`);
			}
			const payload = (await response.json()) as ProcessListResponse;
			processes = payload.processes;
			lastUpdated = payload.generatedAt;
			if (selectedPid && !processes.some((item) => item.pid === selectedPid)) {
				selectedPid = null;
				selectedDetail = null;
				detailError = null;
			} else if (selectedPid) {
				void loadProcessDetail(selectedPid, { silent: true });
			}
		} catch (err) {
			errorMessage = (err as Error).message || 'Failed to load processes';
		} finally {
			if (!options.silent) {
				loading = false;
			}
		}
	}

	async function loadProcessDetail(pid: number, options: { silent?: boolean } = {}) {
		if (!options.silent) {
			detailLoading = true;
			detailError = null;
		}
		try {
			const response = await fetch(`/api/task-manager/processes/${pid}`);
			if (!response.ok) {
				const detail = await response.text().catch(() => '');
				throw new Error(detail || `Request failed with status ${response.status}`);
			}
			selectedDetail = (await response.json()) as ProcessDetail;
			detailError = null;
		} catch (err) {
			selectedDetail = null;
			detailError = (err as Error).message || 'Failed to load process details';
		} finally {
			if (!options.silent) {
				detailLoading = false;
			}
		}
	}

	async function startNewProcess() {
		const trimmed = startCommand.trim();
		if (!trimmed) {
			startError = 'Command is required';
			return;
		}
		startError = null;
		starting = true;
		const args = startArgs.trim() ? splitCommandLine(startArgs) : [];
		const env = parseEnvironment(startEnv);
		recordLog(
			'Start process requested',
			`${trimmed}${args.length ? ` ${args.join(' ')}` : ''}`,
			'queued'
		);
		try {
			const response = await fetch('/api/task-manager/processes', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					command: trimmed,
					args,
					cwd: startCwd.trim() || undefined,
					env
				})
			});
			if (!response.ok) {
				const detail = await response.text().catch(() => '');
				throw new Error(detail || `Request failed with status ${response.status}`);
			}
			recordLog('Process start complete', `${trimmed} launched`, 'complete');
			startCommand = '';
			startArgs = '';
			startCwd = '';
			startEnv = '';
			await loadProcesses({ silent: true });
		} catch (err) {
			startError = (err as Error).message || 'Failed to start process';
			recordLog(
				'Process start failed',
				`${trimmed}: ${(err as Error).message || 'unexpected error'}`,
				'complete'
			);
		} finally {
			starting = false;
		}
	}

	async function handleAction(
		pid: number,
		action: ProcessActionRequest['action'],
		options: { label: string }
	) {
		const target = processes.find((item) => item.pid === pid);
		const descriptor = target ? describeProcess(target) : `PID ${pid}`;
		recordLog(`${options.label} requested`, descriptor, 'queued');
		actionInProgress = pid;
		try {
			const response = await fetch(`/api/task-manager/processes/${pid}/actions`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ action })
			});
			if (!response.ok) {
				const detail = await response.text().catch(() => '');
				throw new Error(detail || `Request failed with status ${response.status}`);
			}
			recordLog(`${options.label} complete`, descriptor, 'complete');
			await loadProcesses({ silent: true });
			if (selectedPid === pid) {
				if (action === 'stop' || action === 'force-stop') {
					selectedPid = null;
					selectedDetail = null;
				} else {
					void loadProcessDetail(pid, { silent: true });
				}
			}
		} catch (err) {
			recordLog(
				`${options.label} failed`,
				`${descriptor}: ${(err as Error).message || 'unexpected error'}`,
				'complete'
			);
		} finally {
			actionInProgress = null;
		}
	}

	function applySortKey(key: SortKey) {
		if (sortKey !== key) {
			sortKey = key;
			sortDirection = key === 'name' ? 'asc' : 'desc';
		}
	}

	function toggleSort(key: SortKey) {
		if (sortKey === key) {
			sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
			return;
		}
		applySortKey(key);
	}

	function selectProcess(pid: number) {
		if (selectedPid === pid) {
			return;
		}
		selectedPid = pid;
		void loadProcessDetail(pid);
	}

	function clearSelection() {
		selectedPid = null;
		selectedDetail = null;
		detailError = null;
	}

	const filteredProcesses = $derived(
		(() => {
			const term = search.trim().toLowerCase();
			const status = statusFilter;
			const list = processes.filter((process) => {
				const matchesStatus = status === 'all' || process.status === status;
				if (!matchesStatus) {
					return false;
				}
				if (!term) {
					return true;
				}
				const haystack = [
					process.name,
					process.command,
					process.user ?? '',
					String(process.pid),
					process.ppid ? String(process.ppid) : ''
				]
					.join(' ')
					.toLowerCase();
				return haystack.includes(term);
			});
			return list.sort((a, b) => {
				const direction = sortDirection === 'asc' ? 1 : -1;
				switch (sortKey) {
					case 'cpu':
						return (a.cpu - b.cpu) * direction;
					case 'memory':
						return (a.memory - b.memory) * direction;
					case 'name':
						return a.name.localeCompare(b.name) * direction;
					case 'pid':
						return (a.pid - b.pid) * direction;
					default:
						return 0;
				}
			});
		})()
	);

	const heroMetadata = $derived(
		(() => [
			{ label: 'Processes', value: processes.length ? `${processes.length}` : '—' },
			{ label: 'Auto refresh', value: autoRefresh ? `Every ${sampleInterval}s` : 'Paused' },
			{ label: 'Last update', value: formatTimestamp(lastUpdated) }
		])()
	);

	function refreshImmediately() {
		void loadProcesses();
	}

	function ensureRefreshTimer() {
		if (refreshTimer) {
			clearInterval(refreshTimer);
			refreshTimer = null;
		}
		if (!autoRefresh) {
			return;
		}
		const interval = Math.max(sampleInterval, 5) * 1000;
		refreshTimer = setInterval(() => {
			void loadProcesses({ silent: true });
		}, interval);
	}

	$effect(() => {
		autoRefresh;
		sampleInterval;
		ensureRefreshTimer();
		return () => {
			if (refreshTimer) {
				clearInterval(refreshTimer);
				refreshTimer = null;
			}
		};
	});

	onMount(() => {
		void loadProcesses();
		return () => {
			if (refreshTimer) {
				clearInterval(refreshTimer);
				refreshTimer = null;
			}
		};
	});
</script>

<div class="space-y-6">
	<div class="grid gap-6 lg:grid-cols-[2fr_1fr]">
		<div class="space-y-6">
			<Card>
				<CardHeader>
					<CardTitle class="text-base">Process controls</CardTitle>
					<CardDescription>Search, sort, and tune snapshot behaviour.</CardDescription>
				</CardHeader>
				<CardContent class="space-y-6">
					<div class="grid gap-4 md:grid-cols-3">
						<div class="grid gap-2">
							<Label for="process-search">Search</Label>
							<Input
								id="process-search"
								bind:value={search}
								placeholder="powershell.exe, 1234, system"
							/>
						</div>
						<div class="grid gap-2">
							<Label for="status-filter">Status</Label>
							<Select
								type="single"
								value={statusFilter}
								onValueChange={(value) => (statusFilter = value as typeof statusFilter)}
							>
								<SelectTrigger id="status-filter" class="w-full">
									<span class="capitalize">{statusFilter.replace('-', ' ')}</span>
								</SelectTrigger>
								<SelectContent>
									{#each statusOptions as option (option.value)}
										<SelectItem value={option.value}>{option.label}</SelectItem>
									{/each}
								</SelectContent>
							</Select>
						</div>
						<div class="grid gap-2">
							<Label>Sort by</Label>
							<div class="flex gap-2">
								<Select
									type="single"
									value={sortKey}
									onValueChange={(value) => applySortKey(value as SortKey)}
								>
									<SelectTrigger class="w-full">
										<span class="capitalize">{sortKey}</span>
									</SelectTrigger>
									<SelectContent>
										<SelectItem value="cpu">CPU</SelectItem>
										<SelectItem value="memory">Memory</SelectItem>
										<SelectItem value="name">Name</SelectItem>
										<SelectItem value="pid">PID</SelectItem>
									</SelectContent>
								</Select>
								<Button
									type="button"
									variant="outline"
									class="min-w-20"
									onclick={() => toggleSort(sortKey)}
								>
									{sortDirection === 'asc' ? 'Ascending' : 'Descending'}
								</Button>
							</div>
						</div>
					</div>
					<div class="grid gap-4 md:grid-cols-3">
						<div class="grid gap-2">
							<Label for="sample-interval">Sample interval (s)</Label>
							<Input
								id="sample-interval"
								type="number"
								min={5}
								step={5}
								bind:value={sampleInterval}
							/>
						</div>
						<label
							class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
						>
							<div>
								<p class="text-sm font-medium text-foreground">Auto refresh</p>
								<p class="text-xs text-muted-foreground">Pull updated snapshots automatically</p>
							</div>
							<Switch bind:checked={autoRefresh} />
						</label>
						<div class="flex items-end">
							<Button type="button" class="w-full" variant="outline" onclick={refreshImmediately}>
								Refresh now
							</Button>
						</div>
					</div>
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle class="text-base">Processes</CardTitle>
					<CardDescription>
						{#if loading}
							Fetching active processes…
						{:else}
							Showing {filteredProcesses.length} of {processes.length} processes.
						{/if}
					</CardDescription>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if errorMessage}
						<Alert variant="destructive">
							<AlertTitle>Process snapshot failed</AlertTitle>
							<AlertDescription>{errorMessage}</AlertDescription>
						</Alert>
					{/if}
					<div class="overflow-hidden rounded-lg border border-border/60 text-sm">
						<table class="w-full divide-y divide-border/60">
							<thead class="bg-muted/40">
								<tr class="text-left">
									<th class="px-4 py-2 font-medium">PID</th>
									<th class="px-4 py-2 font-medium">Process</th>
									<th class="px-4 py-2 font-medium">CPU</th>
									<th class="px-4 py-2 font-medium">Memory</th>
									<th class="px-4 py-2 font-medium">Status</th>
									<th class="px-4 py-2 font-medium">User</th>
									<th class="px-4 py-2 font-medium">Actions</th>
								</tr>
							</thead>
							<tbody>
								{#if filteredProcesses.length === 0}
									<tr>
										<td class="px-4 py-6 text-center text-muted-foreground" colspan={7}>
											{loading
												? 'Loading process snapshot…'
												: 'No processes match the current filters.'}
										</td>
									</tr>
								{:else}
									{#each filteredProcesses as process (process.pid)}
										<tr
											class={cn(
												'cursor-pointer transition hover:bg-muted/30',
												selectedPid === process.pid ? 'bg-muted/40' : 'odd:bg-muted/10'
											)}
											onclick={() => selectProcess(process.pid)}
										>
											<td class="px-4 py-2 font-mono">{process.pid}</td>
											<td class="px-4 py-2">
												<div class="font-medium">{process.name}</div>
												<div class="line-clamp-1 text-xs text-muted-foreground">
													{process.command}
												</div>
											</td>
											<td class="px-4 py-2">{formatCpu(process.cpu)}</td>
											<td class="px-4 py-2">{formatMemory(process.memory)}</td>
											<td class="px-4 py-2">
												<Badge variant={statusBadgeVariant(process.status)} class="uppercase">
													{process.status}
												</Badge>
											</td>
											<td class="px-4 py-2">
												{#if process.user}
													<span class="font-mono text-xs">{process.user}</span>
												{:else}
													<span class="text-muted-foreground">—</span>
												{/if}
											</td>
											<td class="px-4 py-2">
												<div class="flex flex-wrap gap-2">
													<Button
														type="button"
														size="sm"
														variant="outline"
														disabled={actionInProgress === process.pid}
														onclick={(event) => {
															event.stopPropagation();
															void handleAction(process.pid, 'stop', {
																label: 'Stop process'
															});
														}}
													>
														Stop
													</Button>
													<Button
														type="button"
														size="sm"
														variant="outline"
														disabled={actionInProgress === process.pid}
														onclick={(event) => {
															event.stopPropagation();
															void handleAction(process.pid, 'restart', {
																label: 'Restart process'
															});
														}}
													>
														Restart
													</Button>
												</div>
											</td>
										</tr>
									{/each}
								{/if}
							</tbody>
						</table>
					</div>
				</CardContent>
				<CardFooter
					class="flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground"
				>
					<span>Last refreshed: {formatTimestamp(lastUpdated)}</span>
					{#if selectedPid}
						<Button variant="ghost" size="sm" type="button" onclick={clearSelection}>
							Clear selection
						</Button>
					{/if}
				</CardFooter>
			</Card>
		</div>

		<div class="space-y-6">
			<Card>
				<CardHeader>
					<CardTitle class="text-base">Start new process</CardTitle>
					<CardDescription>Launch a process on the controller host.</CardDescription>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if startError}
						<Alert variant="destructive">
							<AlertTitle>Unable to start process</AlertTitle>
							<AlertDescription>{startError}</AlertDescription>
						</Alert>
					{/if}
					<div class="grid gap-3">
						<div class="grid gap-2">
							<Label for="start-command">Command</Label>
							<Input id="start-command" bind:value={startCommand} placeholder="/usr/bin/python" />
						</div>
						<div class="grid gap-2">
							<Label for="start-args">Arguments</Label>
							<Input id="start-args" bind:value={startArgs} placeholder="-m http.server" />
						</div>
						<div class="grid gap-2">
							<Label for="start-cwd">Working directory</Label>
							<Input id="start-cwd" bind:value={startCwd} placeholder="/var/tmp" />
						</div>
						<div class="grid gap-2">
							<Label for="start-env">Environment variables</Label>
							<Textarea id="start-env" bind:value={startEnv} placeholder="KEY=value" rows={3} />
							<p class="text-xs text-muted-foreground">
								Provide one <code>KEY=value</code> pair per line. Lines beginning with
								<code>#</code> are ignored.
							</p>
						</div>
					</div>
				</CardContent>
				<CardFooter class="flex justify-end gap-2">
					<Button
						type="button"
						variant="outline"
						onclick={() => {
							startCommand = '';
							startArgs = '';
							startCwd = '';
							startEnv = '';
							startError = null;
						}}
					>
						Reset
					</Button>
					<Button type="button" onclick={startNewProcess} disabled={starting}>
						{starting ? 'Starting…' : 'Start process'}
					</Button>
				</CardFooter>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle class="text-base">Process details</CardTitle>
					<CardDescription>
						{#if selectedPid}
							Selected PID {selectedPid}
						{:else}
							Choose a process to view metadata and actions.
						{/if}
					</CardDescription>
				</CardHeader>
				<CardContent class="space-y-4">
					{#if detailLoading}
						<p class="text-sm text-muted-foreground">Loading process details…</p>
					{:else if detailError}
						<Alert variant="destructive">
							<AlertTitle>Unable to load process</AlertTitle>
							<AlertDescription>{detailError}</AlertDescription>
						</Alert>
					{:else if selectedDetail}
						<div class="space-y-3 text-sm">
							<div>
								<p class="font-semibold text-foreground">{selectedDetail.name}</p>
								<p class="text-xs text-muted-foreground">
									{selectedDetail.command}
								</p>
							</div>
							<div class="grid gap-2 text-xs text-muted-foreground">
								<div class="flex justify-between">
									<span>PID</span>
									<span class="font-mono text-foreground">{selectedDetail.pid}</span>
								</div>
								{#if selectedDetail.ppid}
									<div class="flex justify-between">
										<span>Parent PID</span>
										<span class="font-mono text-foreground">{selectedDetail.ppid}</span>
									</div>
								{/if}
								{#if selectedDetail.user}
									<div class="flex justify-between">
										<span>User</span>
										<span class="font-mono text-foreground">{selectedDetail.user}</span>
									</div>
								{/if}
								<div class="flex justify-between">
									<span>Status</span>
									<span>
										<Badge variant={statusBadgeVariant(selectedDetail.status)} class="uppercase">
											{selectedDetail.status}
										</Badge>
									</span>
								</div>
								<div class="flex justify-between">
									<span>CPU</span>
									<span class="font-mono text-foreground">{formatCpu(selectedDetail.cpu)}</span>
								</div>
								<div class="flex justify-between">
									<span>Memory</span>
									<span class="font-mono text-foreground"
										>{formatMemory(selectedDetail.memory)}</span
									>
								</div>
								{#if selectedDetail.path}
									<div class="flex flex-col gap-1">
										<span>Executable path</span>
										<span class="font-mono break-all text-foreground">{selectedDetail.path}</span>
									</div>
								{/if}
								{#if selectedDetail.arguments && selectedDetail.arguments.length > 0}
									<div class="flex flex-col gap-1">
										<span>Arguments</span>
										<span class="font-mono break-all text-foreground">
											{selectedDetail.arguments.join(' ')}
										</span>
									</div>
								{/if}
								{#if selectedDetail.startedAt}
									<div class="flex justify-between">
										<span>Started</span>
										<span class="font-mono text-foreground">
											{formatTimestamp(selectedDetail.startedAt)}
										</span>
									</div>
								{/if}
								{#if selectedDetail.priority !== undefined}
									<div class="flex justify-between">
										<span>Priority</span>
										<span class="font-mono text-foreground">{selectedDetail.priority}</span>
									</div>
								{/if}
								{#if selectedDetail.nice !== undefined}
									<div class="flex justify-between">
										<span>Nice</span>
										<span class="font-mono text-foreground">{selectedDetail.nice}</span>
									</div>
								{/if}
							</div>
						</div>
					{:else}
						<p class="text-sm text-muted-foreground">Select a process from the table to inspect.</p>
					{/if}
				</CardContent>
				{#if selectedDetail}
					{@const detail = selectedDetail}
					<CardFooter class="flex flex-wrap gap-2">
						<Button
							type="button"
							variant="destructive"
							disabled={actionInProgress === detail.pid}
							onclick={() =>
								void handleAction(detail.pid, 'stop', {
									label: 'Stop process'
								})}
						>
							Stop
						</Button>
						<Button
							type="button"
							variant="outline"
							disabled={actionInProgress === detail.pid}
							onclick={() =>
								void handleAction(detail.pid, 'restart', {
									label: 'Restart process'
								})}
						>
							Restart
						</Button>
						<Button
							type="button"
							variant="outline"
							disabled={actionInProgress === detail.pid ||
								detail.status === 'suspended' ||
								detail.status === 'stopped'}
							onclick={() =>
								void handleAction(detail.pid, 'suspend', {
									label: 'Suspend process'
								})}
						>
							Suspend
						</Button>
						<Button
							type="button"
							variant="outline"
							disabled={actionInProgress === detail.pid ||
								(detail.status !== 'suspended' && detail.status !== 'stopped')}
							onclick={() =>
								void handleAction(detail.pid, 'resume', {
									label: 'Resume process'
								})}
						>
							Resume
						</Button>
					</CardFooter>
				{/if}
			</Card>
		</div>
	</div>
</div>
