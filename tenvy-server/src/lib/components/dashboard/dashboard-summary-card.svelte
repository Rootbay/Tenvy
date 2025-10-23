<script lang="ts">
	import { cn } from '$lib/utils.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select/index.js';
	import { derived, writable } from 'svelte/store';
	import { Activity, ArrowDownRight, ArrowUpRight, Gauge, UserPlus, Users } from '@lucide/svelte';
	import type {
		DashboardBandwidthSnapshot,
		DashboardLatencySnapshot,
		DashboardNewClientSnapshot
	} from '$lib/data/dashboard';

	const props = $props<{
		totals: { total: number; connected: number };
		newClients: { today: DashboardNewClientSnapshot; week: DashboardNewClientSnapshot };
		bandwidth: DashboardBandwidthSnapshot;
		latency: DashboardLatencySnapshot;
		percentageFormatter: Intl.NumberFormat;
	}>();

	const integerFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });
	const gbFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 });
	const latencyFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });

	type TrendIcon = typeof ArrowUpRight | typeof ArrowDownRight;
	type TrendTone = 'positive' | 'negative' | 'neutral';
	type TrendDescriptor = { text: string; tone: TrendTone; icon: TrendIcon | null };

	const newClientRange = writable<'today' | 'week'>('today');
	const newClientSnapshot = derived(
		newClientRange,
		($range): DashboardNewClientSnapshot => props.newClients[$range]
	);
	const newClientDelta = derived(
		newClientSnapshot,
		($snapshot): TrendDescriptor => describePercentDelta($snapshot.deltaPercent)
	);

	const bandwidthDelta = describePercentDelta(props.bandwidth.deltaPercent);
	const latencyDelta = describeLatencyDelta(props.latency.deltaMs);
	const connectedCaption = `${props.totals.connected}`;

	function describePercentDelta(delta: number | null): TrendDescriptor {
		if (delta === null) {
			return { text: 'No prior comparison', tone: 'neutral', icon: null };
		}
		if (Math.abs(delta) < 0.05) {
			return { text: 'Stable vs prior period', tone: 'neutral', icon: null };
		}
		const tone: TrendTone = delta > 0 ? 'positive' : 'negative';
		const icon: TrendIcon = delta > 0 ? ArrowUpRight : ArrowDownRight;
		const formatted = `${delta > 0 ? '+' : '−'}${props.percentageFormatter.format(Math.abs(delta))}%`;
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
</script>

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
				{integerFormatter.format(props.totals.total)}
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
				<div class="mx-6 w-36">
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
					{gbFormatter.format(props.bandwidth.totalGb)}
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
					{latencyFormatter.format(props.latency.averageMs)}
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
