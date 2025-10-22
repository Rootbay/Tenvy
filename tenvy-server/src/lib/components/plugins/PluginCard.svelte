<script lang="ts">
	import { cn } from '$lib/utils.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import {
		pluginCategoryLabels,
		pluginDeliveryModeLabels,
		pluginStatusLabels,
		pluginStatusStyles,
		type Plugin,
		type PluginUpdatePayload
	} from '$lib/data/plugin-view.js';
	import { Download, Info, PackageSearch, RefreshCcw, ShieldAlert, Wifi } from '@lucide/svelte';

	import {
		distributionModes,
		distributionNotice as defaultDistributionNotice,
		formatSignatureTime,
		signatureBadge,
		statusSeverity
	} from './utils.js';

	let {
		plugin,
		updatePlugin,
		distributionNotice = defaultDistributionNotice
	}: {
		plugin: Plugin;
		updatePlugin: (id: string, patch: PluginUpdatePayload) => void | Promise<void>;
		distributionNotice?: (plugin: Plugin) => string;
	} = $props();
</script>

<Card
	class={cn(
		'border-border/60 transition',
		plugin.status === 'error' && 'border-red-500/40',
		plugin.status === 'update' && 'border-amber-500/40',
		!plugin.enabled && 'opacity-90'
	)}
>
	<CardHeader class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
		<div class="space-y-2">
			<div class="flex flex-wrap items-center gap-3">
				<CardTitle class="text-base leading-tight font-semibold">{plugin.name}</CardTitle>
				<Badge variant="outline" class="px-2.5 py-1 text-xs font-medium text-muted-foreground">
					v{plugin.version}
				</Badge>
				<Badge
					variant="outline"
					class={cn('px-2.5 py-1 text-xs font-medium', pluginStatusStyles[plugin.status])}
				>
					{pluginStatusLabels[plugin.status]}
				</Badge>
				{@const sigBadge = signatureBadge(plugin.signature)}
				<Badge
					variant="outline"
					class={cn('flex items-center gap-1 px-2.5 py-1 text-xs font-medium', sigBadge.class)}
				>
					<svelte:component this={sigBadge.icon} class="h-3.5 w-3.5" />
					{sigBadge.label}
				</Badge>
			</div>
			<CardDescription class="max-w-2xl text-sm text-muted-foreground"
				>{plugin.description}</CardDescription
			>
			<div class="flex flex-wrap items-center gap-2">
				{#each plugin.capabilities as capability (capability)}
					<Badge variant="secondary" class="bg-muted text-muted-foreground">
						{capability}
					</Badge>
				{/each}
				{#if plugin.requiredModules.length > 0}
					<span class="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase">
						Requires
					</span>
					{#each plugin.requiredModules as module (module.id)}
						<Badge
							variant="secondary"
							class="border border-border/60 bg-background/60 text-foreground"
						>
							{module.title}
						</Badge>
					{/each}
				{/if}
			</div>
		</div>
		<div class="flex flex-col gap-4 text-sm text-muted-foreground">
			<div class="flex items-center gap-2">
				<Info class={cn('h-4 w-4', statusSeverity(plugin.status))} />
				<span class="font-medium text-foreground">{pluginCategoryLabels[plugin.category]}</span>
			</div>
			<div class="grid gap-1">
				<span>Maintainer: <strong class="font-medium text-foreground">{plugin.author}</strong></span
				>
				<span>Last deployed {plugin.lastDeployed}</span>
				<span>Health check {plugin.lastChecked}</span>
			</div>
		</div>
	</CardHeader>
	<CardContent class="grid gap-4 lg:grid-cols-2">
		<div class="grid gap-4 text-sm text-muted-foreground md:grid-cols-2 xl:grid-cols-3">
			<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
				<span class="text-xs tracking-wide uppercase">Signature</span>
				{@const sig = signatureBadge(plugin.signature)}
				<p class="flex items-center gap-2 text-sm font-semibold text-foreground">
					<svelte:component this={sig.icon} class="h-4 w-4" />
					{sig.label}
				</p>
				{#if plugin.signature.error}
					<p class="text-xs text-muted-foreground">{plugin.signature.error}</p>
				{:else if plugin.signature.signer}
					<p class="text-xs text-muted-foreground">Signer: {plugin.signature.signer}</p>
				{/if}
				<p class="text-xs text-muted-foreground">
					Checked {formatSignatureTime(plugin.signature.checkedAt)}
				</p>
			</div>
			<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
				<span class="text-xs tracking-wide uppercase">Installations</span>
				<p class="text-lg font-semibold text-foreground">{plugin.installations}</p>
			</div>
			<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
				<span class="text-xs tracking-wide uppercase">Package size</span>
				<p class="text-lg font-semibold text-foreground">{plugin.size}</p>
			</div>
			<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
				<span class="text-xs tracking-wide uppercase">Status</span>
				<p class="text-lg font-semibold text-foreground">{pluginStatusLabels[plugin.status]}</p>
			</div>
			<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
				<div class="flex items-center justify-between">
					<span class="text-xs tracking-wide uppercase">Manual deployments</span>
					<Download class="h-4 w-4 text-muted-foreground" />
				</div>
				<p class="text-lg font-semibold text-foreground">{plugin.distribution.manualTargets}</p>
				<p class="text-xs text-muted-foreground">Last push {plugin.distribution.lastManualPush}</p>
			</div>
			<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
				<div class="flex items-center justify-between">
					<span class="text-xs tracking-wide uppercase">Auto enrollments</span>
					<Wifi class="h-4 w-4 text-muted-foreground" />
				</div>
				<p class="text-lg font-semibold text-foreground">{plugin.distribution.autoTargets}</p>
				<p class="text-xs text-muted-foreground">Last sync {plugin.distribution.lastAutoSync}</p>
			</div>
			<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
				<div class="flex items-center justify-between">
					<span class="text-xs tracking-wide uppercase">Package artifact</span>
					<PackageSearch class="h-4 w-4 text-muted-foreground" />
				</div>
				<p class="font-medium break-words text-foreground">{plugin.artifact}</p>
				<p class="text-xs text-muted-foreground">
					Default: {pluginDeliveryModeLabels[plugin.distribution.defaultMode]}
				</p>
			</div>
		</div>
		<div class="flex flex-col gap-3">
			<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
				<div class="space-y-1">
					<p class="text-sm leading-tight font-medium">Plugin enabled</p>
					<p class="text-xs leading-tight text-muted-foreground">
						Controls whether the module can run on assigned clients.
					</p>
				</div>
				<Switch
					checked={plugin.enabled}
					aria-label={`Toggle ${plugin.name}`}
					onCheckedChange={(value) => {
						const nextStatus = value
							? plugin.status === 'disabled'
								? 'active'
								: plugin.status
							: 'disabled';

						void updatePlugin(plugin.id, {
							enabled: value,
							status: nextStatus
						});
					}}
				/>
			</div>
			<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
				<div class="space-y-1">
					<p class="text-sm leading-tight font-medium">Automatic updates</p>
					<p class="text-xs leading-tight text-muted-foreground">
						When enabled, new builds roll out without manual approval.
					</p>
				</div>
				<Switch
					checked={plugin.autoUpdate}
					aria-label={`Toggle auto update for ${plugin.name}`}
					onCheckedChange={(value) => void updatePlugin(plugin.id, { autoUpdate: value })}
				/>
			</div>
			<div class="space-y-3 rounded-md border border-border/60 px-3 py-2">
				<div class="space-y-1">
					<p class="text-sm leading-tight font-medium">Delivery mode</p>
					<p class="text-xs leading-tight text-muted-foreground">
						Choose how the plugin is distributed to agents and clients.
					</p>
				</div>
				<div class="flex flex-wrap gap-2">
					{#each distributionModes as mode (mode)}
						<Button
							type="button"
							size="sm"
							variant={plugin.distribution.defaultMode === mode ? 'default' : 'outline'}
							disabled={!plugin.enabled}
							aria-pressed={plugin.distribution.defaultMode === mode}
							onclick={() =>
								void updatePlugin(plugin.id, {
									distribution: {
										defaultMode: mode
									}
								})}
						>
							{pluginDeliveryModeLabels[mode]}
						</Button>
					{/each}
				</div>
				<div class="grid gap-3 sm:grid-cols-2">
					<div
						class="flex items-center justify-between rounded-md border border-dashed border-border/60 px-3 py-2"
					>
						<div class="min-w-0 space-y-1">
							<p class="text-sm leading-tight font-medium">Allow manual downloads</p>
							<p class="text-xs leading-tight text-muted-foreground">
								Permit operators to push the package to specific targets.
							</p>
						</div>
						<Switch
							checked={plugin.distribution.allowManualPush}
							disabled={!plugin.enabled}
							aria-label={`Toggle manual downloads for ${plugin.name}`}
							onCheckedChange={(value) =>
								void updatePlugin(plugin.id, {
									distribution: {
										allowManualPush: value
									}
								})}
						/>
					</div>
					<div
						class="flex items-center justify-between rounded-md border border-dashed border-border/60 px-3 py-2"
					>
						<div class="min-w-0 space-y-1">
							<p class="text-sm leading-tight font-medium">Allow auto-sync</p>
							<p class="text-xs leading-tight text-muted-foreground">
								Auto-download the plugin whenever an agent connects.
							</p>
						</div>
						<Switch
							checked={plugin.distribution.allowAutoSync}
							disabled={!plugin.enabled}
							aria-label={`Toggle auto sync for ${plugin.name}`}
							onCheckedChange={(value) =>
								void updatePlugin(plugin.id, {
									distribution: {
										allowAutoSync: value
									}
								})}
						/>
					</div>
				</div>
			</div>
		</div>
	</CardContent>
	<CardFooter class="flex flex-wrap items-center justify-between gap-3">
		<div class="flex items-center gap-2 text-xs tracking-wide text-muted-foreground uppercase">
			<ShieldAlert class="h-4 w-4" />
			{distributionNotice(plugin)}
		</div>
		<div class="flex items-center gap-2">
			<Button type="button" variant="outline" size="sm" class="gap-2">
				<RefreshCcw class="h-4 w-4" />
				Check for updates
			</Button>
			<Button type="button" size="sm" variant="ghost">Open details</Button>
		</div>
	</CardFooter>
</Card>
