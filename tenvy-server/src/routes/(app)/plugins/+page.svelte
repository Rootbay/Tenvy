<script lang="ts">
	import { cn } from '$lib/utils.js';
	import MarketplaceGrid from '$lib/components/plugins/MarketplaceGrid.svelte';
	import PluginCard from '$lib/components/plugins/PluginCard.svelte';
	import {
		marketplaceStatusStyles,
		distributionNotice,
		formatSignatureTime,
		signatureBadge
	} from '$lib/components/plugins/utils.js';
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
		pluginStatusLabels,
		pluginStatusStyles,
		type Plugin,
		type PluginCategory,
		type PluginStatus,
		type PluginUpdatePayload
	} from '$lib/data/plugin-view.js';
	import type {
		AuthenticatedUser,
		MarketplaceEntitlement,
		MarketplaceListing
	} from '$lib/components/plugins/types.js';
	import { Check, Info, RefreshCcw, Search, SlidersHorizontal } from '@lucide/svelte';

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
	const currentUser = $state<AuthenticatedUser>(data.user ?? null);
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

	function resetFilters() {
		searchTerm = '';
		statusFilter = 'all';
		categoryFilter = 'all';
		autoUpdateOnly = false;
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

	const canPurchase = $derived(currentUser?.role === 'admin' || currentUser?.role === 'operator');

	const canSubmitMarketplace = $derived(
		currentUser?.role === 'admin' || currentUser?.role === 'developer'
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
	<MarketplaceGrid
		listings={marketplaceListings}
		entitlements={marketplaceEntitlements}
		{canPurchase}
		{canSubmitMarketplace}
		{purchaseListing}
		{signatureBadge}
		{formatSignatureTime}
		statusStyles={marketplaceStatusStyles}
	/>
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
				<PluginCard {plugin} {updatePlugin} {distributionNotice} />
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
