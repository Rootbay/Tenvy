<script lang="ts">
	import { cn } from '$lib/utils.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import type { IconComponent } from '$lib/types/navigation.js';

	import { Activity, TriangleAlert, CircleCheck, PlugZap, Terminal, Users } from '@lucide/svelte';
	import { Switch } from '$lib/components/ui/switch/index.js';

	type Stat = {
		title: string;
		value: string;
		delta?: string;
		icon: IconComponent;
		iconClass?: string;
	};

	const stats: Stat[] = [
		{
			title: 'Active clients',
			value: '18',
			delta: '+3.2% vs last hour',
			icon: Users,
			iconClass: 'text-emerald-500'
		},
		{
			title: 'Pending tasks',
			value: '42',
			delta: '6 scheduled for execution',
			icon: Terminal,
			iconClass: 'text-amber-500'
		},
		{
			title: 'Plugin status',
			value: '27 online',
			delta: '4 modules awaiting review',
			icon: PlugZap,
			iconClass: 'text-purple-500'
		},
		{
			title: 'Alerts',
			value: '5 open',
			delta: 'Updated moments ago',
			icon: TriangleAlert,
			iconClass: 'text-red-500'
		}
	];

	type LogEntry = {
		source: string;
		message: string;
		time: string;
		level: 'info' | 'warning' | 'critical';
		icon: IconComponent;
		accentClass: string;
	};

	const logEntries: LogEntry[] = [
		{
			source: 'vela/core',
			message: 'Beacon accepted, stage negotiation complete.',
			time: '00:02:14',
			level: 'info',
			icon: Activity,
			accentClass: 'bg-emerald-500/15 text-emerald-500'
		},
		{
			source: 'aurora/tasker',
			message: 'Workflow "Aurora Sweep" completed across 3 hosts.',
			time: '00:14:08',
			level: 'info',
			icon: CircleCheck,
			accentClass: 'bg-blue-500/15 text-blue-500'
		},
		{
			source: 'credentials/cache',
			message: 'New credential bundle awaiting analyst review.',
			time: '00:27:46',
			level: 'warning',
			icon: PlugZap,
			accentClass: 'bg-purple-500/15 text-purple-500'
		},
		{
			source: 'guardian/watch',
			message: 'Safeguard override requested for lateral move.',
			time: '00:41:59',
			level: 'critical',
			icon: TriangleAlert,
			accentClass: 'bg-red-500/15 text-red-500'
		}
	];

	type CommandEntry = {
		kind: 'command' | 'output' | 'system' | 'error';
		prompt?: string;
		text: string;
	};

	const commandStream: CommandEntry[] = [
		{
			kind: 'system',
			text: '[channel] connected to agent VELA :: latency 142ms'
		},
		{
			kind: 'command',
			prompt: 'vela@controller:~$',
			text: 'status --summary'
		},
		{
			kind: 'output',
			text: 'active clients: 18  |  pending tasks: 42  |  safeguards: 2 overrides pending'
		},
		{
			kind: 'command',
			prompt: 'vela@controller:~$',
			text: 'tail --follow=/var/log/vela/broker.log --lines=4'
		},
		{
			kind: 'output',
			text: ':: broker :: queue synced :: 14 new events buffered'
		},
		{
			kind: 'error',
			text: 'warning: safeguard escalation required for host-239'
		},
		{
			kind: 'command',
			prompt: 'vela@controller:~$',
			text: 'acknowledge --ticket=9124'
		},
		{
			kind: 'system',
			text: '[channel] awaiting operator input…'
		}
	];

	let showCommand = false;
</script>

<section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
	{#each stats as stat}
		<Card class="border-border/60">
			<CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
				<CardTitle class="text-sm font-medium">{stat.title}</CardTitle>
				<stat.icon class={cn('h-4 w-4 text-muted-foreground', stat.iconClass)} />
			</CardHeader>
			<CardContent class="space-y-1">
				<div class="text-2xl font-semibold">{stat.value}</div>
				{#if stat.delta}
					<p class="text-xs text-muted-foreground">{stat.delta}</p>
				{/if}
			</CardContent>
		</Card>
	{/each}
</section>

<section class="grid gap-6 lg:grid-cols-7">
	<Card class="lg:col-span-4">
		<CardHeader class="space-y-3">
			<div class="flex items-center gap-2 text-[0.7rem] font-semibold tracking-[0.08em] uppercase">
				<span
					class={cn('transition-colors', showCommand ? 'text-muted-foreground/70' : 'text-primary')}
				>
					Logs
				</span>
				<Switch
					bind:checked={showCommand}
					aria-label="Toggle between log stream and command console"
				/>
				<span
					class={cn('transition-colors', showCommand ? 'text-primary' : 'text-muted-foreground/70')}
				>
					Command
				</span>
			</div>
			<div class="space-y-1">
				<CardTitle>Operations console</CardTitle>
				<CardDescription>
					Monitor live log ingestion or drop into the embedded command interface.
				</CardDescription>
			</div>
		</CardHeader>
		<CardContent class="space-y-4">
			{#if !showCommand}
				<div class="space-y-3">
					{#each logEntries as entry}
						<div class="flex items-start gap-4 rounded-lg border border-border/60 p-4">
							<div
								class={cn(
									'mt-1 flex h-9 w-9 items-center justify-center rounded-md',
									entry.accentClass
								)}
							>
								<entry.icon class="h-4 w-4" />
							</div>
							<div class="flex-1 space-y-1">
								<p class="font-mono text-xs tracking-[0.08em] text-muted-foreground uppercase">
									{entry.source}
								</p>
								<p class="text-sm leading-tight font-medium">{entry.message}</p>
							</div>
							<div class="flex flex-col items-end gap-2 text-xs text-muted-foreground">
								<span>{entry.time}</span>
								<Badge
									variant={entry.level === 'critical'
										? 'destructive'
										: entry.level === 'warning'
											? 'outline'
											: 'secondary'}
								>
									{entry.level}
								</Badge>
							</div>
						</div>
					{/each}
				</div>
			{:else}
				<div
					class="space-y-3 rounded-lg border border-border/60 bg-background/95 p-4 font-mono text-xs"
				>
					{#each commandStream as line}
						<div
							class={cn(
								'flex flex-wrap gap-x-2 gap-y-1',
								line.kind === 'command' && 'text-emerald-400',
								line.kind === 'system' && 'text-sky-400/90',
								line.kind === 'error' && 'text-red-400',
								line.kind === 'output' && 'text-muted-foreground'
							)}
						>
							{#if line.prompt}
								<span class="select-none">{line.prompt}</span>
							{/if}
							<span class="whitespace-pre-wrap">{line.text}</span>
						</div>
					{/each}
					<div class="flex items-center gap-2 text-emerald-400">
						<span class="select-none">vela@controller:~$</span>
						<span class="animate-pulse text-muted-foreground/60">█</span>
					</div>
				</div>
			{/if}
		</CardContent>
	</Card>

	<Card class="lg:col-span-3">
		<CardHeader>
			<CardTitle>Operational health</CardTitle>
			<CardDescription>Signals from infrastructure, telemetry, and safeguards.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4">
			<div class="rounded-lg border border-border/60 p-4">
				<div class="flex items-center justify-between">
					<div>
						<p class="text-sm leading-tight font-semibold">Connectivity</p>
						<p class="text-xs text-muted-foreground">All relay nodes responding</p>
					</div>
					<Badge variant="secondary" class="bg-emerald-500/15 text-emerald-600">Stable</Badge>
				</div>
			</div>
			<div class="rounded-lg border border-border/60 p-4">
				<div class="flex items-center justify-between">
					<div>
						<p class="text-sm leading-tight font-semibold">Command queue</p>
						<p class="text-xs text-muted-foreground">Next dispatch in 38 seconds</p>
					</div>
					<Badge variant="outline" class="border-amber-500/40 text-amber-500">Balanced</Badge>
				</div>
			</div>
			<div class="rounded-lg border border-border/60 p-4">
				<div class="flex items-center justify-between">
					<div>
						<p class="text-sm leading-tight font-semibold">Safeguards</p>
						<p class="text-xs text-muted-foreground">2 overrides pending approval</p>
					</div>
					<Badge variant="outline" class="border-red-500/40 text-red-500">Review</Badge>
				</div>
			</div>
		</CardContent>
	</Card>
</section>
