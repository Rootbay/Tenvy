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
	import { Progress } from '$lib/components/ui/progress/index.js';
	import {
		ChartContainer,
		ChartTooltip,
		type ChartConfig
	} from '$lib/components/ui/chart/index.js';
	import { LineChart } from 'layerchart';
	import ClientPresenceMap from '$lib/components/dashboard/client-presence-map.svelte';
	import { countryCodeToFlag } from '$lib/utils/location';
	import { derived, writable } from 'svelte/store';
	import {
		Activity,
		ArrowDownRight,
		ArrowUpRight,
		Gauge,
		Globe2,
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
	const dayFormatter = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' });
	const relativeFormatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });
	const latencyFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });

	let { data } = $props<{ data: PageData }>();

	const newClientRange = writable<'today' | 'week'>('today');
	const activeView = writable<'logs' | 'map'>('logs');
	const selectedCountry = writable<string | null>(null);

	type TrendIcon = typeof ArrowUpRight | typeof ArrowDownRight;
	type TrendTone = 'positive' | 'negative' | 'neutral';
	type TrendDescriptor = { text: string; tone: TrendTone; icon: TrendIcon | null };

	const generatedAt = new Date(data.generatedAt);

	const countryNameMap = new Map<string, string>(
		data.countries.map(
			(entry: DashboardCountryStat) => [entry.countryCode, entry.countryName] as const
		)
	);

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
	type SelectedCountrySummary = { flag: string; name: string; total: number };
	const selectedCountrySummary = derived(
		[selectedCountry, filteredClients],
		([$selectedCountry, $filteredClients]): SelectedCountrySummary | null => {
			if (!$selectedCountry) {
				return null;
			}
			const name = countryNameMap.get($selectedCountry) ?? $selectedCountry;
			return {
				flag: resolveFlag($selectedCountry),
				name,
				total: $filteredClients.length
			};
		}
	);
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

	const newClientsChartConfig = {
		count: {
			label: 'New clients',
			theme: {
				light: 'var(--chart-1)',
				dark: 'var(--chart-1)'
			}
		}
	} satisfies ChartConfig;

	const newClientsSeries = [
		{
			key: 'count',
			label: newClientsChartConfig.count.label,
			value: (point: (typeof data.newClients.today.series)[number]) => point.count,
			color: 'var(--chart-1)'
		}
	];

	const bandwidthChartConfig = {
		total: {
			label: 'Total transfer (MB)',
			theme: {
				light: 'var(--chart-2)',
				dark: 'var(--chart-2)'
			}
		}
	} satisfies ChartConfig;

	const bandwidthSeries = [
		{
			key: 'totalMb',
			label: bandwidthChartConfig.total.label,
			value: (point: (typeof data.bandwidth.series)[number]) => point.totalMb,
			color: 'var(--chart-2)'
		}
	];

	const latencyChartConfig = {
		latency: {
			label: 'Average latency (ms)',
			theme: {
				light: 'var(--chart-3)',
				dark: 'var(--chart-3)'
			}
		}
	} satisfies ChartConfig;

	const latencySeries = [
		{
			key: 'latencyMs',
			label: latencyChartConfig.latency.label,
			value: (point: (typeof data.latency.series)[number]) => point.latencyMs,
			color: 'var(--chart-3)'
		}
	];

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

	const connectedCaption = `${data.totals.connected} active links`;
	const offlineCaption = `${data.totals.offline} offline`;
</script>

<section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
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
				🟢 {connectedCaption} · 🔴 {offlineCaption}
			</p>
			<p class="text-xs text-muted-foreground">
				Idle + dormant sleepers: {integerFormatter.format(data.totals.idle + data.totals.dormant)}
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
				<div class="flex items-center gap-1 rounded-md border border-border/60 bg-muted/40 p-1">
					<Button
						type="button"
						variant={$newClientRange === 'today' ? 'secondary' : 'ghost'}
						size="sm"
						class="px-3 text-xs"
						onclick={() => newClientRange.set('today')}
					>
						Today
					</Button>
					<Button
						type="button"
						variant={$newClientRange === 'week' ? 'secondary' : 'ghost'}
						size="sm"
						class="px-3 text-xs"
						onclick={() => newClientRange.set('week')}
					>
						This week
					</Button>
				</div>
			</div>
			<ChartContainer config={newClientsChartConfig} class="h-28 w-full">
				<LineChart
					data={$newClientSnapshot.series}
					x={(point) => new Date(point.timestamp)}
					series={newClientsSeries}
					props={{
						xAxis: {
							format: (value) =>
								value instanceof Date
									? $newClientRange === 'today'
										? timeFormatter.format(value)
										: dayFormatter.format(value)
									: ''
						},
						yAxis: {
							format: (value) => `${integerFormatter.format(Number(value ?? 0))}`
						}
					}}
				>
					{#snippet tooltip()}
						<ChartTooltip indicator="line" />
					{/snippet}
				</LineChart>
			</ChartContainer>
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
			<div class="space-y-2">
				<div class="flex items-center justify-between text-xs text-muted-foreground">
					<span>Capacity {gbFormatter.format(data.bandwidth.capacityMb / 1024)} GB</span>
					<span>{percentageFormatter.format(data.bandwidth.usagePercent)}% utilised</span>
				</div>
				<Progress value={data.bandwidth.usagePercent} />
			</div>
			<ChartContainer config={bandwidthChartConfig} class="h-24 w-full">
				<LineChart
					data={data.bandwidth.series}
					x={(point) => new Date(point.timestamp)}
					series={bandwidthSeries}
					props={{
						xAxis: {
							format: (value) => (value instanceof Date ? timeFormatter.format(value) : '')
						},
						yAxis: {
							format: (value) => `${integerFormatter.format(Number(value ?? 0))} MB`
						}
					}}
				>
					{#snippet tooltip()}
						<ChartTooltip indicator="line" />
					{/snippet}
				</LineChart>
			</ChartContainer>
		</CardContent>
	</Card>

	<Card class="border-border/60">
		<CardHeader class="flex flex-col gap-3">
			<div class="flex items-center justify-between gap-3">
				<CardTitle class="text-sm font-semibold">C2 server latency</CardTitle>
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
			<ChartContainer config={latencyChartConfig} class="h-24 w-full">
				<LineChart
					data={data.latency.series}
					x={(point) => new Date(point.timestamp)}
					series={latencySeries}
					props={{
						xAxis: {
							format: (value) => (value instanceof Date ? timeFormatter.format(value) : '')
						},
						yAxis: {
							format: (value) => `${latencyFormatter.format(Number(value ?? 0))} ms`
						}
					}}
				>
					{#snippet tooltip()}
						<ChartTooltip indicator="line" />
					{/snippet}
				</LineChart>
			</ChartContainer>
		</CardContent>
	</Card>
</section>

<section class="grid gap-6 lg:grid-cols-7">
	<Card class="border-border/60 lg:col-span-5">
		<CardHeader class="flex flex-col gap-3">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="space-y-1">
					<CardTitle>Operations stream</CardTitle>
					<CardDescription>
						Inspect live log traffic or map uplink distribution in real time.
					</CardDescription>
				</div>
				<div class="flex items-center gap-1 rounded-md border border-border/60 bg-muted/40 p-1">
					<Button
						type="button"
						variant={$activeView === 'logs' ? 'secondary' : 'ghost'}
						size="sm"
						class="px-3 text-xs"
						onclick={() => activeView.set('logs')}
					>
						Logs
					</Button>
					<Button
						type="button"
						variant={$activeView === 'map' ? 'secondary' : 'ghost'}
						size="sm"
						class="px-3 text-xs"
						onclick={() => activeView.set('map')}
					>
						Map
					</Button>
				</div>
			</div>
			{#if $selectedCountrySummary}
				<Badge variant="outline" class="w-fit gap-2 text-xs uppercase">
					<span>{$selectedCountrySummary.flag}</span>
					<span>{$selectedCountrySummary.name}</span>
					<span class="text-muted-foreground">
						· {integerFormatter.format($selectedCountrySummary.total)}
						{$selectedCountrySummary.total === 1 ? ' client' : ' clients'}
					</span>
				</Badge>
			{/if}
		</CardHeader>
		<CardContent class="space-y-4">
			{#if $activeView === 'logs'}
				<div class="space-y-3">
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
									<Globe2 class="h-4 w-4 text-muted-foreground" />
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
			{:else}
				<ClientPresenceMap clients={$filteredClients} highlightCountry={$selectedCountry} />
			{/if}
		</CardContent>
	</Card>

	<Card class="border-border/60 lg:col-span-2">
		<CardHeader class="flex flex-col gap-2">
			<CardTitle>Country distribution</CardTitle>
			<CardDescription>Click to focus the map and event feed.</CardDescription>
			<div class="flex flex-wrap items-center gap-2">
				<Badge variant="secondary" class="font-mono text-[0.65rem]">
					{integerFormatter.format(data.clients.length)} clients tracked
				</Badge>
				<Button
					type="button"
					variant="ghost"
					size="sm"
					class="h-7 px-2 text-xs"
					onclick={() => selectedCountry.set(null)}
					disabled={!$selectedCountry}
				>
					Reset
				</Button>
			</div>
		</CardHeader>
		<CardContent class="space-y-3">
			{#each countryStats as country (country.countryCode)}
				<button
					type="button"
					class={cn(
						'w-full rounded-lg border px-3 py-2 text-left transition-colors',
						$selectedCountry === country.countryCode
							? 'border-primary/70 bg-primary/10'
							: 'border-border/60 hover:border-primary/50 hover:bg-primary/5'
					)}
					onclick={() => toggleCountry(country.countryCode)}
				>
					<div class="flex items-center justify-between gap-3">
						<div class="flex items-center gap-3">
							<span class="text-lg leading-none">{country.flag}</span>
							<div class="space-y-0.5">
								<p class="text-sm font-semibold text-foreground">{country.countryName}</p>
								<p class="text-xs text-muted-foreground">
									{integerFormatter.format(country.count)} clients · {integerFormatter.format(
										country.onlineCount
									)} active
								</p>
							</div>
						</div>
						<span class="text-sm font-semibold text-muted-foreground">
							{percentageFormatter.format(country.percentage)}%
						</span>
					</div>
					<div class="mt-3 h-2 w-full overflow-hidden rounded-full bg-muted/50">
						<div
							class="h-full rounded-full bg-primary/80"
							style={`width: ${Math.min(country.percentage, 100)}%;`}
						></div>
					</div>
				</button>
			{/each}
		</CardContent>
	</Card>
</section>
