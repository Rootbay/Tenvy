<script lang="ts">
	import { cn } from '$lib/utils.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import {
		Tooltip,
		TooltipContent,
		TooltipProvider,
		TooltipTrigger
	} from '$lib/components/ui/tooltip/index.js';
	import {
		ShieldAlert,
		ShieldCheck,
		Download,
		History,
		RefreshCcw,
		Info,
		Package
	} from '@lucide/svelte';
	import {
		pluginStatusLabels,
		pluginStatusStyles,
		pluginApprovalLabels
	} from '$lib/data/plugin-view.js';
	import type { ClientPlugin } from '$lib/data/client-plugin-view.js';

	let { data }: { data: { clientId: string; plugins: ClientPlugin[] } } = $props();

	const clientId = data.clientId;
	let registry = $state<ClientPlugin[]>(data.plugins.map((plugin) => ({ ...plugin })));

	const installedCount = $derived(registry.length);
	const approvalsPending = $derived(
		registry.filter((plugin) => plugin.approvalStatus === 'pending').length
	);
	const blockedPlugins = $derived(
		registry.filter((plugin) => plugin.telemetry.status === 'blocked').length
	);
	const autoSyncEnabled = $derived(registry.filter((plugin) => plugin.autoUpdate).length);

	function approvalVariant(status: string) {
		switch (status) {
			case 'approved':
				return 'bg-emerald-500/10 text-emerald-500 border-emerald-500/40';
			case 'rejected':
				return 'bg-red-500/10 text-red-500 border-red-500/40';
			default:
				return 'bg-amber-500/10 text-amber-500 border-amber-500/40';
		}
	}

	function telemetryVariant(status: string) {
		switch (status) {
			case 'installed':
				return 'bg-emerald-500/10 text-emerald-500 border-emerald-500/40';
			case 'failed':
			case 'blocked':
				return 'bg-red-500/10 text-red-500 border-red-500/40';
			case 'installing':
				return 'bg-sky-500/10 text-sky-500 border-sky-500/40';
			default:
				return 'bg-muted text-muted-foreground border-muted/60';
		}
	}

	function applyPatch(id: string, next: ClientPlugin) {
		registry = registry.map((plugin) => (plugin.id === id ? next : plugin));
	}

	async function updatePluginEnabled(id: string, enabled: boolean) {
		const previous = registry;
		registry = registry.map((plugin) =>
			plugin.id === id
				? {
						...plugin,
						telemetry: { ...plugin.telemetry, enabled }
					}
				: plugin
		);

		try {
			const response = await fetch(`/api/clients/${clientId}/plugins/${id}`, {
				method: 'PATCH',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ enabled })
			});

			if (!response.ok) {
				const message = await response.text().catch(() => null);
				throw new Error(message || `Failed to update plugin ${id}`);
			}

			const payload = (await response.json()) as { plugin: ClientPlugin };
			applyPatch(id, payload.plugin);
		} catch (err) {
			console.error('Failed to update client plugin', err);
			registry = previous;
		}
	}
</script>

<section class="space-y-6">
	<Card class="border-border/60">
		<CardHeader>
			<CardTitle class="text-xl font-semibold">Plugin telemetry</CardTitle>
			<CardDescription>
				Live installation status and delivery controls for this client.
			</CardDescription>
		</CardHeader>
		<CardContent class="grid gap-6 sm:grid-cols-2 xl:grid-cols-4">
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Catalog size</p>
				<p class="flex items-center gap-2 text-lg font-semibold">
					<Package class="h-4 w-4 text-primary" />
					{installedCount}
				</p>
			</div>
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Auto-sync</p>
				<p class="flex items-center gap-2 text-lg font-semibold">
					<RefreshCcw class="h-4 w-4 text-primary" />
					{autoSyncEnabled}
				</p>
			</div>
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Approvals pending</p>
				<p class="flex items-center gap-2 text-lg font-semibold">
					<ShieldAlert class="h-4 w-4 text-amber-500" />
					{approvalsPending}
				</p>
			</div>
			<div class="space-y-1">
				<p class="text-sm text-muted-foreground">Blocked installs</p>
				<p class="flex items-center gap-2 text-lg font-semibold">
					<ShieldCheck
						class={`h-4 w-4 ${blockedPlugins > 0 ? 'text-red-500' : 'text-emerald-500'}`}
					/>
					{blockedPlugins}
				</p>
			</div>
		</CardContent>
	</Card>

	<div class="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">
		{#each registry as plugin (plugin.id)}
			<Card class="flex flex-col border-border/70 bg-card/40">
				<CardHeader class="space-y-4">
					<div class="flex items-start justify-between gap-4">
						<div>
							<CardTitle class="text-lg font-semibold">
								{plugin.name}
							</CardTitle>
							<CardDescription class="mt-1 flex flex-wrap items-center gap-2 text-xs">
								<span
									class="rounded-full border border-border/80 px-2 py-0.5 font-medium text-muted-foreground"
								>
									v{plugin.version}
								</span>
								<span
									class="rounded-full border border-border/80 px-2 py-0.5 text-muted-foreground"
								>
									{plugin.category}
								</span>
							</CardDescription>
						</div>
						<div class="flex flex-col items-end gap-2 text-xs">
							<Badge class={cn('border', pluginStatusStyles[plugin.globalStatus])}>
								{pluginStatusLabels[plugin.globalStatus]}
							</Badge>
							<Badge class={cn('border', approvalVariant(plugin.approvalStatus))}>
								{pluginApprovalLabels[plugin.approvalStatus as keyof typeof pluginApprovalLabels] ??
									plugin.approvalStatus}
							</Badge>
						</div>
					</div>
					<p class="text-sm leading-6 text-muted-foreground">{plugin.description}</p>
				</CardHeader>
				<CardContent class="flex flex-1 flex-col gap-4">
					<div
						class="rounded-md border border-border/60 bg-muted/10 p-3 text-sm text-muted-foreground"
					>
						<div class="flex items-center justify-between gap-4">
							<div class="space-y-1">
								<p class="font-medium text-foreground">Delivery policy</p>
								<p>
									Default: <span class="font-semibold">{plugin.distribution.defaultMode}</span>
									· Auto-update {plugin.distribution.autoUpdate ? 'enabled' : 'disabled'}
								</p>
								<p class="truncate">
									Artifact: <span class="font-mono text-xs">{plugin.artifact}</span>
								</p>
							</div>
						</div>
						<Separator class="my-3" />
						<div class="flex flex-wrap items-center gap-3 text-xs">
							<span class="rounded border border-border/60 px-2 py-0.5">{plugin.size}</span>
							{#if plugin.expectedHash}
								<TooltipProvider>
									<Tooltip>
										<TooltipTrigger class="rounded border border-border/60 px-2 py-0.5 font-mono">
											hash
										</TooltipTrigger>
										<TooltipContent side="bottom" class="max-w-xs break-all">
											{plugin.expectedHash}
										</TooltipContent>
									</Tooltip>
								</TooltipProvider>
							{/if}
						</div>
					</div>

					<div class="rounded-md border border-border/60 bg-background/60 p-3 text-sm">
						<div class="flex items-center justify-between gap-4">
							<div class="flex flex-col gap-1">
								<span class="text-xs tracking-wide text-muted-foreground uppercase"
									>Installation</span
								>
								<Badge class={cn('border', telemetryVariant(plugin.telemetry.status))}>
									{plugin.telemetry.status}
								</Badge>
							</div>
							<div class="flex items-center gap-2 text-xs text-muted-foreground">
								<Switch
									aria-label={`Toggle ${plugin.name}`}
									checked={plugin.telemetry.enabled}
									onCheckedChange={(event) => updatePluginEnabled(plugin.id, event.detail)}
								/>
								<span>{plugin.telemetry.enabled ? 'Enabled' : 'Disabled'}</span>
							</div>
						</div>
						<Separator class="my-3" />
						<div class="grid gap-2 text-xs text-muted-foreground">
							<div class="flex items-center gap-2">
								<History class="h-3.5 w-3.5" />
								<span>Last deployed: {plugin.telemetry.lastDeployed}</span>
							</div>
							<div class="flex items-center gap-2">
								<RefreshCcw class="h-3.5 w-3.5" />
								<span>Last check: {plugin.telemetry.lastChecked}</span>
							</div>
							<div class="flex items-center gap-2">
								<Download class="h-3.5 w-3.5" />
								<span>Hash: {plugin.telemetry.hash ?? 'n/a'}</span>
							</div>
							{#if plugin.telemetry.error}
								<div class="flex items-start gap-2 text-red-500">
									<Info class="mt-0.5 h-3.5 w-3.5" />
									<span>{plugin.telemetry.error}</span>
								</div>
							{/if}
						</div>
					</div>
				</CardContent>
				<CardFooter class="justify-end text-xs text-muted-foreground">
					<div class="flex flex-wrap gap-2">
						{#if plugin.requirements.platforms.length > 0}
							<span>Platforms: {plugin.requirements.platforms.join(', ')}</span>
						{/if}
						{#if plugin.requirements.architectures.length > 0}
							<Separator orientation="vertical" />
							<span>Architectures: {plugin.requirements.architectures.join(', ')}</span>
						{/if}
						{#if plugin.requirements.requiredModules.length > 0}
							<Separator orientation="vertical" />
							<span>Requires modules: {plugin.requirements.requiredModules.join(', ')}</span>
						{/if}
					</div>
				</CardFooter>
			</Card>
		{/each}
	</div>
</section>
