<script lang="ts">
	import { cn } from '$lib/utils.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import {
		availableTags,
		clients,
		riskStyles,
		statusLabels,
		statusStyles,
		statusSummaryOrder,
		type ClientPlatform,
		type ClientStatus
	} from '$lib/data/clients.js';
	import { LayoutGrid, Search, List, Users } from '@lucide/svelte';

	const statusFilters = [
		{ label: 'All', value: 'all' },
		{ label: 'Online', value: 'online' },
		{ label: 'Idle', value: 'idle' },
		{ label: 'Dormant', value: 'dormant' },
		{ label: 'Offline', value: 'offline' }
	] satisfies { label: string; value: 'all' | ClientStatus }[];

	const platformFilters = [
		{ label: 'All platforms', value: 'all' },
		{ label: 'Windows', value: 'windows' },
		{ label: 'Linux', value: 'linux' },
		{ label: 'macOS', value: 'macos' }
	] satisfies { label: string; value: 'all' | ClientPlatform }[];

	let searchTerm = '';
	let statusFilter: (typeof statusFilters)[number]['value'] = 'all';
	let platformFilter: (typeof platformFilters)[number]['value'] = 'all';
	let tagFilter: string | 'all' = 'all';
	let viewMode: 'table' | 'grid' = 'table';

	const statusSummary = clients.reduce(
		(acc, client) => {
			acc[client.status] += 1;
			return acc;
		},
		{
			online: 0,
			idle: 0,
			dormant: 0,
			offline: 0
		} satisfies Record<ClientStatus, number>
	);

	$: normalizedSearch = searchTerm.trim().toLowerCase();
	$: filteredClients = clients.filter((client) => {
		const matchesSearch =
			normalizedSearch.length === 0 ||
			[client.codename, client.hostname, client.ip, client.location, client.os, client.version]
				.join(' ')
				.toLowerCase()
				.includes(normalizedSearch) ||
			client.tags.some((tag) => tag.toLowerCase().includes(normalizedSearch));

		const matchesStatus = statusFilter === 'all' || client.status === statusFilter;
		const matchesPlatform = platformFilter === 'all' || client.platform === platformFilter;
		const matchesTag = tagFilter === 'all' || client.tags.includes(tagFilter);

		return matchesSearch && matchesStatus && matchesPlatform && matchesTag;
	});
</script>

<section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
	{#each statusSummaryOrder as status}
		<Card class="border-border/60">
			<CardHeader class="space-y-1">
				<CardTitle class="text-sm font-medium">{statusLabels[status]}</CardTitle>
				<CardDescription>Active clients matching this posture.</CardDescription>
			</CardHeader>
			<CardContent class="space-y-1">
				<div class="text-2xl font-semibold">{statusSummary[status]}</div>
				<Badge
					variant="outline"
					class={cn('w-fit px-2.5 py-1 text-xs font-medium', statusStyles[status])}
				>
					{statusLabels[status]}
				</Badge>
			</CardContent>
		</Card>
	{/each}
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
						placeholder="Filter by codename, host, IP, or tag"
						class="pl-10"
						bind:value={searchTerm}
					/>
				</div>
				<div class="flex items-center gap-2">
					<Button
						type="button"
						size="sm"
						variant={viewMode === 'table' ? 'default' : 'outline'}
						onclick={() => (viewMode = 'table')}
					>
						<LayoutGrid class="mr-2 h-4 w-4" />
						Table
					</Button>
					<Button
						type="button"
						size="sm"
						variant={viewMode === 'grid' ? 'default' : 'outline'}
						onclick={() => (viewMode = 'grid')}
					>
						<List class="mr-2 h-4 w-4" />
						Cards
					</Button>
				</div>
			</div>
			<Separator />
			<div class="grid gap-4 lg:grid-cols-3">
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
						Platform
					</p>
					<div class="flex flex-wrap gap-2">
						{#each platformFilters as option}
							<Button
								type="button"
								size="sm"
								variant={platformFilter === option.value ? 'default' : 'outline'}
								onclick={() => (platformFilter = option.value)}
							>
								{option.label}
							</Button>
						{/each}
					</div>
				</div>
				<div class="space-y-2">
					<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">Tags</p>
					<div class="flex flex-wrap gap-2">
						<Button
							type="button"
							size="sm"
							variant={tagFilter === 'all' ? 'default' : 'outline'}
							onclick={() => (tagFilter = 'all')}
						>
							All tags
						</Button>
						{#each availableTags as tag}
							<Button
								type="button"
								size="sm"
								variant={tagFilter === tag ? 'default' : 'outline'}
								onclick={() => (tagFilter = tag)}
							>
								{tag}
							</Button>
						{/each}
					</div>
				</div>
			</div>
		</CardHeader>
	</Card>

	{#if filteredClients.length > 0}
		{#if viewMode === 'table'}
			<Card class="overflow-hidden border-border/60">
				<CardHeader class="border-b border-border/60">
					<CardTitle class="text-base font-semibold">Client inventory</CardTitle>
					<CardDescription>Structured view of connected and dormant clients.</CardDescription>
				</CardHeader>
				<CardContent class="p-0">
					<div class="overflow-x-auto">
						<table class="min-w-full divide-y divide-border/60 text-sm">
							<thead class="bg-muted/20">
								<tr>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Codename
									</th>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Host
									</th>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Status
									</th>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Platform
									</th>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Location
									</th>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Last seen
									</th>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Tags
									</th>
									<th
										class="px-4 py-3 text-left text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>
										Risk
									</th>
								</tr>
							</thead>
							<tbody class="divide-y divide-border/60 bg-background">
								{#each filteredClients as client}
									<tr class="hover:bg-muted/30">
										<td class="px-4 py-3 align-top">
											<div class="font-semibold text-foreground">{client.codename}</div>
											<div class="text-xs text-muted-foreground">#{client.id}</div>
										</td>
										<td class="px-4 py-3 align-top">
											<div class="font-medium">{client.hostname}</div>
											<div class="text-xs text-muted-foreground">{client.ip}</div>
										</td>
										<td class="px-4 py-3 align-top">
											<Badge
												variant="outline"
												class={cn('px-2.5 py-1 text-xs font-medium', statusStyles[client.status])}
											>
												{statusLabels[client.status]}
											</Badge>
										</td>
										<td class="px-4 py-3 align-top text-sm font-medium">
											<div class="capitalize">{client.platform}</div>
											<div class="text-xs text-muted-foreground">{client.os}</div>
										</td>
										<td class="px-4 py-3 align-top text-sm">
											<div>{client.location}</div>
										</td>
										<td class="px-4 py-3 align-top text-sm">
											<div class="font-medium">{client.lastSeen}</div>
											<div class="text-xs text-muted-foreground">v{client.version}</div>
										</td>
										<td class="px-4 py-3 align-top">
											<div class="flex flex-wrap gap-1.5">
												{#each client.tags as tag}
													<Badge
														variant="outline"
														class="border-border/70 px-2 py-0.5 text-xs font-medium"
													>
														{tag}
													</Badge>
												{/each}
											</div>
										</td>
										<td class="px-4 py-3 align-top">
											<Badge
												variant="outline"
												class={cn('px-2.5 py-1 text-xs font-semibold', riskStyles[client.risk])}
											>
												{client.risk}
											</Badge>
											{#if client.notes}
												<p class="mt-2 max-w-[16rem] text-xs text-muted-foreground">
													{client.notes}
												</p>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</CardContent>
			</Card>
		{:else}
			<div class="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
				{#each filteredClients as client}
					<Card class="border-border/60">
						<CardHeader class="space-y-3">
							<div class="flex items-start justify-between gap-2">
								<div>
									<CardTitle class="text-base font-semibold">{client.codename}</CardTitle>
									<CardDescription class="text-xs tracking-wide text-muted-foreground uppercase">
										#{client.id}
									</CardDescription>
								</div>
								<Badge
									variant="outline"
									class={cn('px-2.5 py-1 text-xs font-medium', statusStyles[client.status])}
								>
									{statusLabels[client.status]}
								</Badge>
							</div>
							<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
								<span class="font-medium text-foreground">{client.hostname}</span>
								<span>•</span>
								<span>{client.ip}</span>
								<span>•</span>
								<span>{client.location}</span>
							</div>
						</CardHeader>
						<CardContent class="space-y-4 text-sm">
							<div class="flex items-center justify-between">
								<span class="text-muted-foreground">Platform</span>
								<span class="font-medium capitalize">{client.platform}</span>
							</div>
							<div class="flex items-center justify-between">
								<span class="text-muted-foreground">Operating system</span>
								<span class="text-right font-medium">{client.os}</span>
							</div>
							<div class="flex items-center justify-between">
								<span class="text-muted-foreground">Last seen</span>
								<span class="font-medium">{client.lastSeen}</span>
							</div>
							<div class="flex items-center justify-between">
								<span class="text-muted-foreground">Version</span>
								<span class="font-medium">v{client.version}</span>
							</div>
							<div>
								<span class="text-xs font-semibold tracking-wide text-muted-foreground uppercase"
									>Tags</span
								>
								<div class="mt-2 flex flex-wrap gap-1.5">
									{#each client.tags as tag}
										<Badge
											variant="outline"
											class="border-border/70 px-2 py-0.5 text-xs font-medium"
										>
											{tag}
										</Badge>
									{/each}
								</div>
							</div>
							<div class="flex items-center justify-between">
								<span class="text-muted-foreground">Risk</span>
								<Badge
									variant="outline"
									class={cn('px-2.5 py-1 text-xs font-semibold', riskStyles[client.risk])}
								>
									{client.risk}
								</Badge>
							</div>
							{#if client.notes}
								<p
									class="rounded-md border border-border/60 bg-muted/40 p-3 text-xs text-muted-foreground"
								>
									{client.notes}
								</p>
							{/if}
						</CardContent>
					</Card>
				{/each}
			</div>
		{/if}
	{:else}
		<Card class="border-dashed border-border/60 bg-muted/5">
			<CardContent class="flex flex-col items-center justify-center gap-3 py-16 text-center">
				<Users class="h-10 w-10 text-muted-foreground/40" />
				<div class="space-y-1">
					<h3 class="text-base font-semibold text-foreground">
						No clients match the current filters
					</h3>
					<p class="text-sm text-muted-foreground">
						Adjust your status, platform, or tag filters to reveal more agents.
					</p>
				</div>
				<Button
					type="button"
					variant="outline"
					onclick={() => {
						searchTerm = '';
						statusFilter = 'all';
						platformFilter = 'all';
						tagFilter = 'all';
					}}
				>
					Reset filters
				</Button>
			</CardContent>
		</Card>
	{/if}
</section>
