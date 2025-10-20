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
	import { Input } from '$lib/components/ui/input/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import {
		pluginCategories,
		pluginCategoryLabels,
		pluginDeliveryModeLabels,
		pluginStatusLabels,
		pluginStatusStyles,
		type Plugin,
		type PluginCategory,
		type PluginDeliveryMode,
		type PluginStatus,
		type PluginUpdatePayload
	} from '$lib/data/plugin-view.js';
	import type { PluginManifest } from '../../../../../shared/types/plugin-manifest.js';
	import type { UserRole } from '$lib/server/auth.js';
	import {
		Check,
		Download,
		Info,
		PackageSearch,
		RefreshCcw,
		Search,
		ShieldAlert,
		SlidersHorizontal,
		Wifi,
		ShieldCheck,
		GitFork
	} from '@lucide/svelte';

	type MarketplaceStatus = 'pending' | 'approved' | 'rejected';

	type MarketplaceListing = {
		id: string;
		name: string;
		summary: string | null;
		repositoryUrl: string;
		version: string;
		pricingTier: string;
		status: MarketplaceStatus;
		manifest: PluginManifest;
		submittedBy: string | null;
		reviewerId: string | null;
	};

	type MarketplaceEntitlement = {
		id: string;
		listingId: string;
		tenantId: string;
		seats: number;
		status: string;
		listing: MarketplaceListing;
	};

	type AuthenticatedUser = { id: string; role: UserRole };

	let {
		data
	}: {
		data: {
			plugins: Plugin[];
			listings: MarketplaceListing[];
			entitlements: MarketplaceEntitlement[];
			user: AuthenticatedUser;
		};
	} = $props();

	const statusFilters = [
		{ label: 'All', value: 'all' },
		{ label: 'Active', value: 'active' },
		{ label: 'Disabled', value: 'disabled' },
		{ label: 'Updates', value: 'update' },
		{ label: 'Attention', value: 'error' }
	] satisfies { label: string; value: 'all' | PluginStatus }[];

	const categoryFilters = [
		{ label: 'All categories', value: 'all' },
		...pluginCategories.map((category) => ({
			label: pluginCategoryLabels[category],
			value: category
		}))
	] satisfies { label: string; value: 'all' | PluginCategory }[];

	let registry = $state<Plugin[]>(data.plugins.map((plugin) => ({ ...plugin })));
	let marketplaceListings = $state<MarketplaceListing[]>(
		data.listings.map((listing) => ({ ...listing }))
	);
	let marketplaceEntitlements = $state<MarketplaceEntitlement[]>(
		data.entitlements.map((entitlement) => ({ ...entitlement }))
	);
	const currentUser = $state<AuthenticatedUser>(data.user);
	let searchTerm = $state('');
	let statusFilter = $state<'all' | PluginStatus>('all');
	let categoryFilter = $state<'all' | PluginCategory>('all');
	let autoUpdateOnly = $state(false);
	let filtersOpen = $state(false);

	function mergePluginPatch(plugin: Plugin, patch: PluginUpdatePayload): Plugin {
		const next: Plugin = { ...plugin };

		if (patch.status !== undefined) next.status = patch.status;
		if (patch.enabled !== undefined) next.enabled = patch.enabled;
		if (patch.autoUpdate !== undefined) next.autoUpdate = patch.autoUpdate;
		if (patch.installations !== undefined) next.installations = patch.installations;

		if (patch.distribution) {
			next.distribution = {
				...next.distribution,
				...patch.distribution
			};
		}

		return next;
	}

	async function updatePlugin(id: string, patch: PluginUpdatePayload) {
		const previous = registry;
		registry = registry.map((plugin: Plugin) =>
			plugin.id === id ? mergePluginPatch(plugin, patch) : plugin
		);

		try {
			const response = await fetch(`/api/plugins/${id}`, {
				method: 'PATCH',
				headers: {
					'content-type': 'application/json'
				},
				body: JSON.stringify(patch)
			});

			if (!response.ok) {
				const message = await response.text().catch(() => null);
				throw new Error(message || `Failed to update plugin ${id}`);
			}

			const payload = (await response.json()) as { plugin: Plugin };
			registry = registry.map((plugin: Plugin) => (plugin.id === id ? payload.plugin : plugin));
		} catch (err) {
			console.error('Failed to update plugin', err);
			registry = previous;
		}
	}

	const distributionModes: PluginDeliveryMode[] = ['manual', 'automatic'];

	function distributionNotice(plugin: Plugin): string {
		if (!plugin.enabled) return 'Plugin currently disabled';

		const notes = [`Default: ${pluginDeliveryModeLabels[plugin.distribution.defaultMode]}`];

		if (!plugin.distribution.allowManualPush) {
			notes.push('manual pushes blocked');
		}

		if (!plugin.distribution.allowAutoSync) {
			notes.push('auto-sync paused');
		}

		return notes.join(' · ');
	}

	function resetFilters() {
		searchTerm = '';
		statusFilter = 'all';
		categoryFilter = 'all';
		autoUpdateOnly = false;
	}

	function statusSeverity(status: PluginStatus) {
		switch (status) {
			case 'error':
				return 'text-red-500';
			case 'update':
				return 'text-amber-500';
			default:
				return 'text-muted-foreground';
		}
	}

	const normalizedSearch = $derived(searchTerm.trim().toLowerCase());

	const filteredPlugins: Plugin[] = $derived.by(() => {
		const term = normalizedSearch;
		return registry.filter((plugin: Plugin) => {
			const matchesSearch =
				term.length === 0 ||
				[
					plugin.name,
					plugin.description,
					plugin.author,
					plugin.version,
					...plugin.capabilities,
					...plugin.requiredModules.map((module) => module.title)
				]
					.join(' ')
					.toLowerCase()
					.includes(term);

			const matchesStatus = statusFilter === 'all' || plugin.status === statusFilter;
			const matchesCategory = categoryFilter === 'all' || plugin.category === categoryFilter;
			const matchesAuto = !autoUpdateOnly || plugin.autoUpdate;

			return matchesSearch && matchesStatus && matchesCategory && matchesAuto;
		});
	});

	const totalInstalled = $derived(registry.length);
	const updatesPending = $derived(registry.filter((p: Plugin) => p.status === 'update').length);
	const autoManagedCount = $derived(registry.filter((p: Plugin) => p.autoUpdate).length);
	const totalCoverage = $derived(
		registry.reduce((acc: number, p: Plugin) => acc + p.installations, 0)
	);

	const filtersActive = $derived(
		normalizedSearch.length > 0 ||
			statusFilter !== 'all' ||
			categoryFilter !== 'all' ||
			autoUpdateOnly
	);

	const listingStatusStyles: Record<MarketplaceStatus, string> = {
		approved: 'border border-emerald-500/40 bg-emerald-500/10 text-emerald-500',
		pending: 'border border-amber-500/40 bg-amber-500/10 text-amber-500',
		rejected: 'border border-red-500/40 bg-red-500/10 text-red-500'
	};

	function isEntitled(listingId: string): boolean {
		return marketplaceEntitlements.some((entry) => entry.listingId === listingId);
	}

	const canPurchase = $derived(currentUser.role === 'admin' || currentUser.role === 'operator');

	const canSubmitMarketplace = $derived(
		currentUser.role === 'admin' || currentUser.role === 'developer'
	);

	async function purchaseListing(listing: MarketplaceListing) {
		if (!canPurchase) {
			return;
		}

		try {
			const response = await fetch('/api/marketplace/entitlements', {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify({ listingId: listing.id })
			});

			if (!response.ok) {
				const message = await response.text().catch(() => null);
				throw new Error(message ?? 'Failed to purchase listing');
			}

			const payload = (await response.json()) as {
				entitlement: MarketplaceEntitlement;
			};
			marketplaceEntitlements = [...marketplaceEntitlements, payload.entitlement];
		} catch (err) {
			console.error('Failed to purchase listing', err);
		}
	}
</script>

<section class="space-y-6">
	<Card class="border-border/60">
		<CardHeader class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
			<div class="space-y-1">
				<CardTitle>Marketplace</CardTitle>
				<CardDescription>
					Deploy community plugins backed by signed releases from public repositories.
				</CardDescription>
			</div>
			<Badge variant="secondary" class="gap-2 text-xs">
				<ShieldCheck class="h-4 w-4" />
				{marketplaceListings.length} listing{marketplaceListings.length === 1 ? '' : 's'}
			</Badge>
		</CardHeader>
		<CardContent>
			{#if marketplaceListings.length === 0}
				<p class="text-sm text-muted-foreground">
					No marketplace submissions yet. Developers can publish plugins once approved by an
					administrator.
				</p>
			{:else}
				<div class="grid gap-4 lg:grid-cols-2 xl:grid-cols-3">
					{#each marketplaceListings as listing (listing.id)}
						<div
							class="flex flex-col justify-between rounded-lg border border-border bg-card p-4 shadow-sm"
						>
							<div class="space-y-3">
								<div class="flex items-start justify-between gap-3">
									<div class="space-y-1">
										<h3 class="text-base leading-tight font-semibold">{listing.name}</h3>
										<p class="text-xs tracking-wide text-muted-foreground uppercase">
											Version {listing.version} · {listing.pricingTier}
										</p>
									</div>
									<Badge class={listingStatusStyles[listing.status]}>{listing.status}</Badge>
								</div>
								<p class="text-sm leading-relaxed text-muted-foreground">
									{listing.summary ?? listing.manifest.description ?? 'No description provided.'}
								</p>
								<div class="flex flex-col gap-2 text-xs text-muted-foreground">
									<div class="flex items-center gap-2">
										<GitFork class="h-3.5 w-3.5" />
										<a
											href={listing.repositoryUrl}
											rel="noreferrer"
											target="_blank"
											class="truncate underline decoration-dotted hover:text-foreground"
										>
											{listing.repositoryUrl}
										</a>
									</div>
									<div class="flex items-center gap-2">
										<ShieldCheck class="h-3.5 w-3.5" />
										<span>{listing.manifest.license.spdxId}</span>
									</div>
								</div>
							</div>
							<div class="mt-4 flex items-center justify-between">
								<span class="text-xs text-muted-foreground">
									{listing.manifest.capabilities?.length ?? 0} capability{(listing.manifest
										.capabilities?.length ?? 0) === 1
										? ''
										: 'ies'}
								</span>
								<Button
									size="sm"
									variant={isEntitled(listing.id) ? 'outline' : 'default'}
									disabled={!canPurchase || isEntitled(listing.id) || listing.status !== 'approved'}
									onclick={() => purchaseListing(listing)}
								>
									{#if isEntitled(listing.id)}
										Entitled
									{:else if listing.status !== 'approved'}
										Awaiting review
									{:else}
										Purchase
									{/if}
								</Button>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</CardContent>
		{#if canSubmitMarketplace}
			<CardFooter class="text-xs text-muted-foreground">
				Developers can submit new plugins via the controller API after uploading signed release
				assets to GitHub.
			</CardFooter>
		{/if}
	</Card>
	<Card class="border-border/60">
		<CardHeader class="space-y-4">
			<div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
				<div class="relative w-full max-w-sm">
					<Search
						class="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
					/>
					<Input
						type="search"
						placeholder="Search plugins, capabilities, or authors"
						class="pl-10"
						bind:value={searchTerm}
					/>
				</div>
				<div class="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
					<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium"
						>{totalInstalled} - Installed plugins</Badge
					>
					<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium"
						>{updatesPending} - Updating plugins</Badge
					>
					<div class="flex items-center gap-2">
						<Check class="h-4 w-4" />
						{autoManagedCount} auto-managed
					</div>
					<div class="flex items-center gap-2">
						<Info class="h-4 w-4" />
						{totalCoverage} endpoints
					</div>
					{#if filtersActive}
						<Button type="button" variant="ghost" size="sm" class="gap-2" onclick={resetFilters}>
							<RefreshCcw class="h-4 w-4" />
							Reset filters
						</Button>
					{/if}
				</div>
				<div class="flex justify-end">
					<Button
						type="button"
						size="icon"
						variant="ghost"
						class={cn(
							'text-muted-foreground transition-colors',
							filtersOpen && 'bg-muted text-foreground hover:bg-muted'
						)}
						aria-label={filtersOpen ? 'Hide filters' : 'Show filters'}
						aria-pressed={filtersOpen}
						aria-expanded={filtersOpen}
						onclick={() => (filtersOpen = !filtersOpen)}
					>
						<SlidersHorizontal class="h-4 w-4" />
					</Button>
				</div>
			</div>
			{#if filtersOpen}
				<Separator />
				<div class="grid gap-4 xl:grid-cols-3">
					<div class="space-y-2">
						<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
							Status
						</p>
						<div class="flex flex-wrap gap-2">
							{#each statusFilters as option (option.value)}
								<Button
									type="button"
									size="sm"
									variant={statusFilter === option.value ? 'default' : 'outline'}
									onclick={() => (statusFilter = option.value)}
								>
									{option.label}
								</Button>
							{/each}
						</div>
					</div>
					<div class="space-y-2">
						<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
							Category
						</p>
						<div class="flex flex-wrap gap-2">
							{#each categoryFilters as option (option.value)}
								<Button
									type="button"
									size="sm"
									variant={categoryFilter === option.value ? 'default' : 'outline'}
									onclick={() => (categoryFilter = option.value)}
								>
									{option.label}
								</Button>
							{/each}
						</div>
					</div>
					<div class="space-y-2">
						<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
							Auto updates
						</p>
						<div class="flex items-center gap-3 rounded-md border border-border/60 px-3 py-2">
							<Switch bind:checked={autoUpdateOnly} aria-label="Toggle auto update filter" />
							<div class="min-w-0">
								<p class="text-sm leading-tight font-medium">Only show auto-managed</p>
								<p class="text-xs leading-tight text-muted-foreground">
									Limit results to plugins with automatic updates enabled.
								</p>
							</div>
						</div>
					</div>
				</div>
			{/if}
		</CardHeader>
	</Card>

	{#if filteredPlugins.length > 0}
		<div class="grid gap-4">
			{#each filteredPlugins as plugin (plugin.id)}
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
								<Badge
									variant="outline"
									class="px-2.5 py-1 text-xs font-medium text-muted-foreground"
								>
									v{plugin.version}
								</Badge>
								<Badge
									variant="outline"
									class={cn('px-2.5 py-1 text-xs font-medium', pluginStatusStyles[plugin.status])}
								>
									{pluginStatusLabels[plugin.status]}
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
									<span
										class="text-[10px] font-semibold tracking-wide text-muted-foreground uppercase"
									>
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
								<span class="font-medium text-foreground"
									>{pluginCategoryLabels[plugin.category]}</span
								>
							</div>
							<div class="grid gap-1">
								<span
									>Maintainer: <strong class="font-medium text-foreground">{plugin.author}</strong
									></span
								>
								<span>Last deployed {plugin.lastDeployed}</span>
								<span>Health check {plugin.lastChecked}</span>
							</div>
						</div>
					</CardHeader>
					<CardContent class="grid gap-4 lg:grid-cols-2">
						<div class="grid gap-4 text-sm text-muted-foreground md:grid-cols-2 xl:grid-cols-3">
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
								<p class="text-lg font-semibold text-foreground">
									{pluginStatusLabels[plugin.status]}
								</p>
							</div>
							<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
								<div class="flex items-center justify-between">
									<span class="text-xs tracking-wide uppercase">Manual deployments</span>
									<Download class="h-4 w-4 text-muted-foreground" />
								</div>
								<p class="text-lg font-semibold text-foreground">
									{plugin.distribution.manualTargets}
								</p>
								<p class="text-xs text-muted-foreground">
									Last push {plugin.distribution.lastManualPush}
								</p>
							</div>
							<div class="space-y-1 rounded-md border border-border/60 px-3 py-2">
								<div class="flex items-center justify-between">
									<span class="text-xs tracking-wide uppercase">Auto enrollments</span>
									<Wifi class="h-4 w-4 text-muted-foreground" />
								</div>
								<p class="text-lg font-semibold text-foreground">
									{plugin.distribution.autoTargets}
								</p>
								<p class="text-xs text-muted-foreground">
									Last sync {plugin.distribution.lastAutoSync}
								</p>
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
							<div
								class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2"
							>
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
							<div
								class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2"
							>
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
						<div
							class="flex items-center gap-2 text-xs tracking-wide text-muted-foreground uppercase"
						>
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
			{/each}
		</div>
	{:else}
		<Card class="border-border/60">
			<CardHeader>
				<CardTitle class="text-base font-semibold">No plugins match the current filters</CardTitle>
				<CardDescription
					>Adjust filters or clear the search query to see registered modules again.</CardDescription
				>
			</CardHeader>
			<CardFooter>
				<Button type="button" onclick={resetFilters} class="gap-2">
					<RefreshCcw class="h-4 w-4" />
					Reset filters
				</Button>
			</CardFooter>
		</Card>
	{/if}
</section>
