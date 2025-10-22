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
	import { Popover, PopoverContent, PopoverTrigger } from '$lib/components/ui/popover/index.js';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select/index.js';
        import ClientPresenceMap from '$lib/components/dashboard/client-presence-map.lazy.svelte';
	import { countryCodeToFlag } from '$lib/utils/location';
	import { derived, writable } from 'svelte/store';
	import {
		Activity,
		ArrowDownRight,
		ArrowUpRight,
		Gauge,
		Earth,
		ChevronDown,
		UserPlus,
		Users
	} from '@lucide/svelte';
	import type {
		DashboardClient,
		DashboardCountryStat,
		DashboardLogEntry
	} from '$lib/data/dashboard';
	import type { PageData } from './$types';

	const integerFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });
	const gbFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 });
	const percentageFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });
	const timeFormatter = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' });
	const relativeFormatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
	const latencyFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });

	let { data } = $props<{ data: PageData }>();

	const newClientRange = writable<'today' | 'week'>('today');
	const activeView = writable<'logs' | 'map'>('map');
	const selectedCountry = writable<string | null>(null);

	type TrendIcon = typeof ArrowUpRight | typeof ArrowDownRight;
	type TrendTone = 'positive' | 'negative' | 'neutral';
	type TrendDescriptor = { text: string; tone: TrendTone; icon: TrendIcon | null };

	const generatedAt = new Date(data.generatedAt);

	const newClientSnapshot = derived(
		newClientRange,
		($range): (typeof data.newClients)[keyof typeof data.newClients] => data.newClients[$range]
	);
	const newClientDelta = derived(
		newClientSnapshot,
		($snapshot): TrendDescriptor => describePercentDelta($snapshot.deltaPercent)
	);
	const filteredLogs = derived(selectedCountry, ($selectedCountry): DashboardLogEntry[] =>
		$selectedCountry
			? data.logs.filter((entry: DashboardLogEntry) => entry.countryCode === $selectedCountry)
			: data.logs
	);
	const filteredClients = derived(selectedCountry, ($selectedCountry): DashboardClient[] =>
		$selectedCountry
			? data.clients.filter(
					(entry: DashboardClient) => entry.location.countryCode === $selectedCountry
				)
			: data.clients
	);
	const countryStats: DashboardCountryStat[] = data.countries;
	const bandwidthDelta = describePercentDelta(data.bandwidth.deltaPercent);
	const latencyDelta = describeLatencyDelta(data.latency.deltaMs);

	const severityVariant: Record<
		DashboardLogEntry['severity'],
		'secondary' | 'outline' | 'destructive'
	> = {
		info: 'secondary',
		warning: 'outline',
		critical: 'destructive'
	};

	const severityTone: Record<DashboardLogEntry['severity'], string> = {
		info: 'text-sky-500',
		warning: 'text-amber-500',
		critical: 'text-destructive'
	};

	function describePercentDelta(delta: number | null): TrendDescriptor {
		if (delta === null) {
			return { text: 'No prior comparison', tone: 'neutral', icon: null };
		}
		if (Math.abs(delta) < 0.05) {
			return { text: 'Stable vs prior period', tone: 'neutral', icon: null };
		}
		const tone: TrendTone = delta > 0 ? 'positive' : 'negative';
		const icon: TrendIcon = delta > 0 ? ArrowUpRight : ArrowDownRight;
		const formatted = `${delta > 0 ? '+' : '−'}${percentageFormatter.format(Math.abs(delta))}%`;
		return { text: `${formatted} vs prior period`, tone, icon };
	}

	function describeLatencyDelta(delta: number): TrendDescriptor {
		if (Math.abs(delta) < 0.1) {
			return { text: 'Stable vs last window', tone: 'neutral', icon: null };
		}
		const tone: TrendTone = delta < 0 ? 'positive' : 'negative';
		const icon: TrendIcon = delta < 0 ? ArrowDownRight : ArrowUpRight;
		const formatted = `${delta > 0 ? '+' : '−'}${latencyFormatter.format(Math.abs(delta))} ms`;
		return { text: `${formatted} vs last window`, tone, icon };
	}

	function formatRelative(timestamp: string): string {
		const difference = new Date(timestamp).getTime() - generatedAt.getTime();
		const abs = Math.abs(difference);
		if (abs < 60_000) {
			return 'moments ago';
		}
		if (abs < 3_600_000) {
			return relativeFormatter.format(Math.round(difference / 60_000), 'minute');
		}
		if (abs < 86_400_000) {
			return relativeFormatter.format(Math.round(difference / 3_600_000), 'hour');
		}
		return relativeFormatter.format(Math.round(difference / 86_400_000), 'day');
	}

	function formatLogTime(timestamp: string): string {
		return timeFormatter.format(new Date(timestamp));
	}

	function toggleCountry(code: string) {
		selectedCountry.update((current) => (current === code ? null : code));
	}

	function resolveFlag(code: string | null): string {
		return code ? countryCodeToFlag(code) : '🌐';
	}

	const connectedCaption = `${data.totals.connected}`;
</script>

<div class="flex h-full min-h-0 flex-1 flex-col gap-6 overflow-hidden">
	<section class="grid flex-none gap-4 md:grid-cols-2 xl:grid-cols-4">
		<Card class="border-border/60">
			<CardHeader class="flex flex-col gap-3">
				<div class="flex items-center justify-between gap-3">
					<CardTitle class="text-sm font-semibold">Total clients</CardTitle>
					<span
						class="flex h-9 w-9 items-center justify-center rounded-full border border-border/60 bg-muted/40"
					>
						<Users class="h-4 w-4 text-muted-foreground" />
					</span>
				</div>
				<CardDescription>Live controller footprint across every uplink.</CardDescription>
			</CardHeader>
			<CardContent class="space-y-2">
				<div class="text-3xl font-semibold tracking-tight">
					{integerFormatter.format(data.totals.total)}
				</div>
				<p class="text-xs text-muted-foreground">
					Active: {connectedCaption}
				</p>
			</CardContent>
		</Card>

		<Card class="border-border/60">
			<CardHeader class="flex flex-col gap-3">
				<div class="flex items-center justify-between gap-3">
					<CardTitle class="text-sm font-semibold">New clients</CardTitle>
					<span
						class="flex h-9 w-9 items-center justify-center rounded-full border border-border/60 bg-muted/40"
					>
						<UserPlus class="h-4 w-4 text-muted-foreground" />
					</span>
					<div class="mx-6 w-[9rem]">
						<Select
							type="single"
							value={$newClientRange}
							onValueChange={(value) => {
								if (value === 'today' || value === 'week') {
									newClientRange.set(value);
								}
							}}
						>
							<SelectTrigger
								id="new-client-range"
								class="h-9 w-full justify-between border-border/60 bg-muted/40 px-3 text-xs font-medium"
							>
								<span>{$newClientRange === 'today' ? 'Today' : 'This week'}</span>
							</SelectTrigger>
							<SelectContent>
								<SelectItem value="today">Today</SelectItem>
								<SelectItem value="week">This week</SelectItem>
							</SelectContent>
						</Select>
					</div>
				</div>
				<CardDescription>Enrollment momentum for operators.</CardDescription>
			</CardHeader>
			<CardContent class="space-y-4">
				<div class="flex flex-wrap items-center justify-between gap-3">
					<div>
						<div class="text-3xl font-semibold tracking-tight">
							{integerFormatter.format($newClientSnapshot.total)}
						</div>
						{#if $newClientDelta.text}
							<div
								class={cn(
									'mt-1 flex items-center gap-1 text-xs',
									$newClientDelta.tone === 'positive'
										? 'text-emerald-500'
										: $newClientDelta.tone === 'negative'
											? 'text-rose-500'
											: 'text-muted-foreground'
								)}
							>
								{#if $newClientDelta.icon}
									{@const Icon = $newClientDelta.icon}
									<Icon class="h-3.5 w-3.5" />
								{/if}
								<span>{$newClientDelta.text}</span>
							</div>
						{/if}
					</div>
				</div>
			</CardContent>
		</Card>

		<Card class="border-border/60">
			<CardHeader class="flex flex-col gap-3">
				<div class="flex items-center justify-between gap-3">
					<CardTitle class="text-sm font-semibold">Bandwidth usage</CardTitle>
					<span
						class="flex h-9 w-9 items-center justify-center rounded-full border border-border/60 bg-muted/40"
					>
						<Activity class="h-4 w-4 text-muted-foreground" />
					</span>
				</div>
				<CardDescription>Aggregate transfer over the last 24 hours.</CardDescription>
			</CardHeader>
			<CardContent class="space-y-4">
				<div>
					<div class="text-3xl font-semibold tracking-tight">
						{gbFormatter.format(data.bandwidth.totalGb)}
						<span class="text-base font-normal text-muted-foreground">GB</span>
					</div>
					<div
						class={cn(
							'mt-1 flex items-center gap-1 text-xs',
							bandwidthDelta.tone === 'positive'
								? 'text-emerald-500'
								: bandwidthDelta.tone === 'negative'
									? 'text-rose-500'
									: 'text-muted-foreground'
						)}
					>
						{#if bandwidthDelta.icon}
							{@const Icon = bandwidthDelta.icon}
							<Icon class="h-3.5 w-3.5" />
						{/if}
						<span>{bandwidthDelta.text}</span>
					</div>
				</div>
			</CardContent>
		</Card>

		<Card class="border-border/60">
			<CardHeader class="flex flex-col gap-3">
				<div class="flex items-center justify-between gap-3">
					<CardTitle class="text-sm font-semibold">Latency</CardTitle>
					<span
						class="flex h-9 w-9 items-center justify-center rounded-full border border-border/60 bg-muted/40"
					>
						<Gauge class="h-4 w-4 text-muted-foreground" />
					</span>
				</div>
				<CardDescription>Heartbeat round-trip monitoring.</CardDescription>
			</CardHeader>
			<CardContent class="space-y-4">
				<div>
					<div class="text-3xl font-semibold tracking-tight">
						{latencyFormatter.format(data.latency.averageMs)}
						<span class="text-base font-normal text-muted-foreground">ms</span>
					</div>
					<div
						class={cn(
							'mt-1 flex items-center gap-1 text-xs',
							latencyDelta.tone === 'positive'
								? 'text-emerald-500'
								: latencyDelta.tone === 'negative'
									? 'text-rose-500'
									: 'text-muted-foreground'
						)}
					>
						{#if latencyDelta.icon}
							{@const Icon = latencyDelta.icon}
							<Icon class="h-3.5 w-3.5" />
						{/if}
						<span>{latencyDelta.text}</span>
					</div>
				</div>
			</CardContent>
		</Card>
	</section>

	<section
		class="grid h-full min-h-0 flex-1 auto-rows-[minmax(0,1fr)] gap-6 overflow-hidden lg:grid-cols-7"
	>
		<Card class="flex h-[32rem] flex-col border-border/60 lg:col-span-5">
			<CardContent class="relative flex min-h-0 flex-1 flex-col gap-4 overflow-hidden">
				<div class="pointer-events-none absolute top-4 right-4 z-10">
					<Popover>
						<PopoverTrigger
							type="button"
							class="pointer-events-auto flex h-9 w-9 items-center justify-center rounded-full border border-border/60 bg-background/95 text-muted-foreground shadow-sm transition hover:text-foreground focus-visible:ring-2 focus-visible:ring-primary/60 focus-visible:outline-none"
							aria-label="Open operations view menu"
						>
							<ChevronDown class="h-4 w-4" />
							<span class="sr-only">Toggle operations view menu</span>
						</PopoverTrigger>
						<PopoverContent align="end" sideOffset={12} class="w-36 space-y-2 p-3">
							<Button
								type="button"
								variant={$activeView === 'map' ? 'secondary' : 'ghost'}
								size="sm"
								class="w-full text-xs"
								onclick={() => activeView.set('map')}
							>
								Map
							</Button>
							<Button
								type="button"
								variant={$activeView === 'logs' ? 'secondary' : 'ghost'}
								size="sm"
								class="w-full text-xs"
								onclick={() => activeView.set('logs')}
							>
								Logs
							</Button>
						</PopoverContent>
					</Popover>
				</div>
				{#if $activeView === 'map'}
					<div class="min-h-0 flex-1 overflow-hidden">
						<ClientPresenceMap clients={$filteredClients} highlightCountry={$selectedCountry} />
					</div>
				{:else}
					<div class="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1">
						{#if $filteredLogs.length === 0}
							<div
								class="rounded-lg border border-dashed border-border/60 p-6 text-center text-sm text-muted-foreground"
							>
								No events matched this country filter.
							</div>
						{/if}
						{#each $filteredLogs as entry (entry.id)}
							<div
								class="flex flex-col gap-4 rounded-lg border border-border/60 p-4 md:flex-row md:items-center md:justify-between"
							>
								<div class="flex items-start gap-3">
									<span
										class="flex h-10 w-10 items-center justify-center rounded-md border border-border/60 bg-muted/40"
									>
										<Earth class="h-4 w-4 text-muted-foreground" />
									</span>
									<div class="space-y-1">
										<div class="flex items-center gap-2 text-sm font-semibold">
											<span>{entry.codename}</span>
											<span class="text-xs text-muted-foreground">
												{resolveFlag(entry.countryCode ?? null)}
											</span>
										</div>
										<p
											class="font-mono text-[0.65rem] tracking-[0.08em] text-muted-foreground uppercase"
										>
											{entry.action}
										</p>
										<p class="text-sm text-muted-foreground">{entry.description}</p>
									</div>
								</div>
								<div class="flex flex-col items-start gap-2 md:items-end">
									<div class="flex items-center gap-2 text-xs text-muted-foreground">
										<span>{formatLogTime(entry.timestamp)}</span>
										<span aria-hidden="true">•</span>
										<span>{formatRelative(entry.timestamp)}</span>
									</div>
									<Badge
										variant={severityVariant[entry.severity]}
										class={cn('tracking-wide uppercase', severityTone[entry.severity])}
									>
										{entry.severity}
									</Badge>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</CardContent>
		</Card>
		<Card class="flex h-[32rem] flex-col border-border/60 lg:col-span-2">
			<CardContent class="flex-1 overflow-hidden p-0">
				<div class="h-full flex-1 overflow-y-auto">
					<div class="divide-y divide-border/60">
						{#each countryStats as country (country.countryCode)}
							{@const countryCode = country.countryCode}
							{@const flagUrl =
								countryCode && countryCode.length > 0
									? `https://flagcdn.com/${countryCode.toLowerCase()}.svg`
									: null}
							<button
								type="button"
								class={cn(
									'flex w-full items-center justify-between gap-3 px-6 py-3 text-left transition-colors',
									$selectedCountry === country.countryCode ? 'bg-primary/10' : 'hover:bg-primary/5'
								)}
								onclick={() => toggleCountry(country.countryCode)}
							>
								<div class="flex items-center gap-3">
									{#if flagUrl}
										<img
											src={flagUrl}
											alt=""
											class="h-5 w-8 rounded-sm border border-border/60 object-cover"
											loading="lazy"
										/>
									{:else}
										<span class="text-lg leading-none" aria-hidden="true">{country.flag}</span>
									{/if}
									<div class="space-y-0.5">
										<p class="text-sm font-semibold text-foreground">{country.countryName}</p>
									</div>
								</div>
								<span
									class={cn(
										'rounded-md border px-2 py-0.5 text-xs font-medium',
										$selectedCountry === country.countryCode
											? 'border-primary/60 text-primary'
											: 'border-border/60 text-muted-foreground'
									)}
								>
									<p class="text-xs text-muted-foreground">
										{percentageFormatter.format(country.percentage)}%
									</p>
								</span>
							</button>
						{/each}
					</div>
				</div>
			</CardContent>
		</Card>
	</section>
</div>
