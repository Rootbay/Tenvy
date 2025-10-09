<script lang="ts">
	import { cn } from '$lib/utils.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle
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
                plugins as pluginSeed,
                type Plugin,
                type PluginCategory,
                type PluginDeliveryMode,
                type PluginStatus
        } from '$lib/data/plugins.js';
        import {
                Check,
                Download,
                Info,
                PackageSearch,
                RefreshCcw,
                Search,
                ShieldAlert,
                SlidersHorizontal,
                Wifi
        } from '@lucide/svelte';

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

	let registry = $state<Plugin[]>(pluginSeed.map((p) => ({ ...p })));
	let searchTerm = $state('');
	let statusFilter = $state<'all' | PluginStatus>('all');
	let categoryFilter = $state<'all' | PluginCategory>('all');
	let autoUpdateOnly = $state(false);
	let filtersOpen = $state(false);

        function updatePlugin(id: string, patch: Partial<Plugin>) {
                registry = registry.map((plugin: Plugin) =>
                        plugin.id === id ? { ...plugin, ...patch } : plugin
                );
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
			case 'error': return 'text-red-500';
			case 'update': return 'text-amber-500';
			default: return 'text-muted-foreground';
		}
	}

	const normalizedSearch = $derived(searchTerm.trim().toLowerCase());

	const filteredPlugins: Plugin[] = $derived.by(() => {
		const term = normalizedSearch;
		return registry.filter((plugin: Plugin) => {
			const matchesSearch =
				term.length === 0 ||
				[plugin.name, plugin.description, plugin.author, plugin.version, ...plugin.capabilities]
					.join(' ')
					.toLowerCase()
					.includes(term);

			const matchesStatus = statusFilter === 'all' || plugin.status === statusFilter;
			const matchesCategory = categoryFilter === 'all' || plugin.category === categoryFilter;
			const matchesAuto = !autoUpdateOnly || plugin.autoUpdate;

			return matchesSearch && matchesStatus && matchesCategory && matchesAuto;
		});
	});

	const totalInstalled   = $derived(registry.length);
	const activeCount      = $derived(registry.filter((p: Plugin) => p.enabled).length);
	const updatesPending   = $derived(registry.filter((p: Plugin) => p.status === 'update').length);
	const autoManagedCount = $derived(registry.filter((p: Plugin) => p.autoUpdate).length);
	const totalCoverage    = $derived(registry.reduce((acc: number, p: Plugin) => acc + p.installations, 0));

	const filtersActive = $derived(
		normalizedSearch.length > 0 ||
		statusFilter !== 'all' ||
		categoryFilter !== 'all' ||
		autoUpdateOnly
	);
</script>

<section class="space-y-6">
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
					<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium">{totalInstalled} - Installed plugins</Badge>
					<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium">{updatesPending} - Updating plugins</Badge>
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
						<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">Status</p>
						<div class="flex flex-wrap gap-2">
							{#each statusFilters as option}
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
							{#each categoryFilters as option}
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
                                                        <p class="font-medium text-foreground break-words">{plugin.artifact}</p>
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

										updatePlugin(plugin.id, {
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
                                                                        onCheckedChange={(value) => updatePlugin(plugin.id, { autoUpdate: value })}
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
                                                                        {#each distributionModes as mode}
                                                                                <Button
                                                                                        type="button"
                                                                                        size="sm"
                                                                                        variant={plugin.distribution.defaultMode === mode ? 'default' : 'outline'}
                                                                                        disabled={!plugin.enabled}
                                                                                        aria-pressed={plugin.distribution.defaultMode === mode}
                                                                                        onclick={() =>
                                                                                                updatePlugin(plugin.id, {
                                                                                                        distribution: {
                                                                                                                ...plugin.distribution,
                                                                                                                defaultMode: mode
                                                                                                        }
                                                                                                })
                                                                                        }
                                                                                >
                                                                                        {pluginDeliveryModeLabels[mode]}
                                                                                </Button>
                                                                        {/each}
                                                                </div>
                                                                <div class="grid gap-3 sm:grid-cols-2">
                                                                        <div class="flex items-center justify-between rounded-md border border-dashed border-border/60 px-3 py-2">
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
                                                                                                updatePlugin(plugin.id, {
                                                                                                        distribution: {
                                                                                                                ...plugin.distribution,
                                                                                                                allowManualPush: value
                                                                                                        }
                                                                                                })
                                                                                        }
                                                                                />
                                                                        </div>
                                                                        <div class="flex items-center justify-between rounded-md border border-dashed border-border/60 px-3 py-2">
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
                                                                                                updatePlugin(plugin.id, {
                                                                                                        distribution: {
                                                                                                                ...plugin.distribution,
                                                                                                                allowAutoSync: value
                                                                                                        }
                                                                                                })
                                                                                        }
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
