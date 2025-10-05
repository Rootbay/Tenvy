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
		pluginStatusLabels,
		pluginStatusStyles,
		plugins as pluginSeed,
		type Plugin,
		type PluginCategory,
		type PluginStatus
	} from '$lib/data/plugins.js';

	import { Check, Info, RefreshCcw, Search, ShieldAlert } from '@lucide/svelte';

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

	let registry: Plugin[] = pluginSeed.map((plugin) => ({ ...plugin }));

	let searchTerm = '';
	let statusFilter: (typeof statusFilters)[number]['value'] = 'all';
	let categoryFilter: (typeof categoryFilters)[number]['value'] = 'all';
	let autoUpdateOnly = false;

	const updatePlugin = (id: string, patch: Partial<Plugin>) => {
		registry = registry.map((plugin) => (plugin.id === id ? { ...plugin, ...patch } : plugin));
	};

	const resetFilters = () => {
		searchTerm = '';
		statusFilter = 'all';
		categoryFilter = 'all';
		autoUpdateOnly = false;
	};

	const statusSeverity = (status: PluginStatus) => {
		switch (status) {
			case 'error':
				return 'text-red-500';
			case 'update':
				return 'text-amber-500';
			default:
				return 'text-muted-foreground';
		}
	};

	$: normalizedSearch = searchTerm.trim().toLowerCase();

	$: filteredPlugins = registry.filter((plugin) => {
		const matchesSearch =
			normalizedSearch.length === 0 ||
			[plugin.name, plugin.description, plugin.author, plugin.version, ...plugin.capabilities]
				.join(' ')
				.toLowerCase()
				.includes(normalizedSearch);

		const matchesStatus = statusFilter === 'all' || plugin.status === statusFilter;
		const matchesCategory = categoryFilter === 'all' || plugin.category === categoryFilter;
		const matchesAuto = !autoUpdateOnly || plugin.autoUpdate;

		return matchesSearch && matchesStatus && matchesCategory && matchesAuto;
	});

	$: totalInstalled = registry.length;
	$: activeCount = registry.filter((plugin) => plugin.enabled).length;
	$: updatesPending = registry.filter((plugin) => plugin.status === 'update').length;
	$: autoManagedCount = registry.filter((plugin) => plugin.autoUpdate).length;
	$: totalCoverage = registry.reduce((acc, plugin) => acc + plugin.installations, 0);

	$: filtersActive =
		normalizedSearch.length > 0 ||
		statusFilter !== 'all' ||
		categoryFilter !== 'all' ||
		autoUpdateOnly;
</script>

<section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
	<Card class="border-border/60">
		<CardHeader class="space-y-1">
			<CardTitle class="text-sm font-medium">Installed plugins</CardTitle>
			<CardDescription>Total modules currently registered.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-1">
			<div class="text-2xl font-semibold">{totalInstalled}</div>
			<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium">Registry</Badge>
		</CardContent>
	</Card>
	<Card class="border-border/60">
		<CardHeader class="space-y-1">
			<CardTitle class="text-sm font-medium">Active</CardTitle>
			<CardDescription>Enabled plugins delivering capability.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-1">
			<div class="text-2xl font-semibold">{activeCount}</div>
			<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium">Live</Badge>
		</CardContent>
	</Card>
	<Card class="border-border/60">
		<CardHeader class="space-y-1">
			<CardTitle class="text-sm font-medium">Updates pending</CardTitle>
			<CardDescription>Plugins waiting for rollout.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-1">
			<div class="text-2xl font-semibold">{updatesPending}</div>
			<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium">Maintenance</Badge>
		</CardContent>
	</Card>
	<Card class="border-border/60">
		<CardHeader class="space-y-1">
			<CardTitle class="text-sm font-medium">Endpoint coverage</CardTitle>
			<CardDescription>Clients receiving plugin capabilities.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-1">
			<div class="text-2xl font-semibold">{totalCoverage}</div>
			<Badge variant="outline" class="w-fit px-2.5 py-1 text-xs font-medium">Installations</Badge>
		</CardContent>
	</Card>
</section>

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
			</div>
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
						<div class="flex flex-wrap gap-4 text-sm text-muted-foreground">
							<div class="rounded-md border border-border/60 px-3 py-2">
								<span class="text-xs tracking-wide uppercase">Installations</span>
								<p class="text-lg font-semibold text-foreground">{plugin.installations}</p>
							</div>
							<div class="rounded-md border border-border/60 px-3 py-2">
								<span class="text-xs tracking-wide uppercase">Package size</span>
								<p class="text-lg font-semibold text-foreground">{plugin.size}</p>
							</div>
							<div class="rounded-md border border-border/60 px-3 py-2">
								<span class="text-xs tracking-wide uppercase">Status</span>
								<p class="text-lg font-semibold text-foreground">
									{pluginStatusLabels[plugin.status]}
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
						</div>
					</CardContent>
					<CardFooter class="flex flex-wrap items-center justify-between gap-3">
						<div
							class="flex items-center gap-2 text-xs tracking-wide text-muted-foreground uppercase"
						>
							<ShieldAlert class="h-4 w-4" />
							{plugin.enabled ? 'Policy enforced across deployments' : 'Plugin currently disabled'}
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
