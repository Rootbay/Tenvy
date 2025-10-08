<script lang="ts">
        import {
                Card,
                CardContent,
                CardDescription,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { Badge } from '$lib/components/ui/badge/index.js';
        import { ChartContainer, ChartTooltip, type ChartConfig } from '$lib/components/ui/chart/index.js';
        import { AreaChart, BarChart, LineChart } from 'layerchart';

        type SummaryMetric = {
                label: string;
                value: string;
                delta: string;
                tone: 'positive' | 'warning' | 'neutral';
        };

        const summaryMetrics: SummaryMetric[] = [
                {
                        label: 'Live beacons',
                        value: '24',
                        delta: '+4 vs last hour',
                        tone: 'positive'
                },
                {
                        label: 'Tasks dispatched',
                        value: '138',
                        delta: '32 queued downstream',
                        tone: 'neutral'
                },
                {
                        label: 'Escalations',
                        value: '7 open',
                        delta: '2 awaiting analyst review',
                        tone: 'warning'
                },
                {
                        label: 'New clients today',
                        value: '11',
                        delta: '86% provisioned via vouchers',
                        tone: 'neutral'
                }
        ];

        const summaryToneClasses: Record<SummaryMetric['tone'], string> = {
                positive: 'text-emerald-500',
                warning: 'text-amber-500',
                neutral: 'text-muted-foreground'
        };

        type ActivityPoint = {
                timestamp: Date;
                active: number;
                idle: number;
                suppressed: number;
        };

        const timeFormatter = new Intl.DateTimeFormat('en-US', {
                hour: '2-digit',
                minute: '2-digit'
        });

        const numberFormatter = new Intl.NumberFormat('en-US', {
                maximumFractionDigits: 0
        });

        const activityTimeline: ActivityPoint[] = [
                { timestamp: new Date('2024-12-01T08:00:00Z'), active: 9, idle: 4, suppressed: 1 },
                { timestamp: new Date('2024-12-01T09:00:00Z'), active: 11, idle: 5, suppressed: 2 },
                { timestamp: new Date('2024-12-01T10:00:00Z'), active: 14, idle: 4, suppressed: 2 },
                { timestamp: new Date('2024-12-01T11:00:00Z'), active: 18, idle: 6, suppressed: 3 },
                { timestamp: new Date('2024-12-01T12:00:00Z'), active: 22, idle: 7, suppressed: 3 },
                { timestamp: new Date('2024-12-01T13:00:00Z'), active: 19, idle: 6, suppressed: 3 },
                { timestamp: new Date('2024-12-01T14:00:00Z'), active: 17, idle: 6, suppressed: 2 },
                { timestamp: new Date('2024-12-01T15:00:00Z'), active: 21, idle: 5, suppressed: 2 }
        ];

        const activityChartConfig = {
                active: {
                        label: 'Active beacons',
                        theme: {
                                light: 'var(--chart-1)',
                                dark: 'var(--chart-1)'
                        }
                },
                idle: {
                        label: 'Idle sleepers',
                        theme: {
                                light: 'var(--chart-2)',
                                dark: 'var(--chart-2)'
                        }
                },
                suppressed: {
                        label: 'Suppressed links',
                        theme: {
                                light: 'var(--chart-3)',
                                dark: 'var(--chart-3)'
                        }
                }
        } satisfies ChartConfig;

        const activitySeries = [
                {
                        key: 'active',
                        label: activityChartConfig.active.label,
                        value: (point: ActivityPoint) => point.active,
                        color: 'var(--color-active)'
                },
                {
                        key: 'idle',
                        label: activityChartConfig.idle.label,
                        value: (point: ActivityPoint) => point.idle,
                        color: 'var(--color-idle)'
                },
                {
                        key: 'suppressed',
                        label: activityChartConfig.suppressed.label,
                        value: (point: ActivityPoint) => point.suppressed,
                        color: 'var(--color-suppressed)'
                }
        ];

        const moduleActivity = [
                { module: 'Reconnaissance', executed: 42, queued: 8 },
                { module: 'Credential access', executed: 31, queued: 6 },
                { module: 'Persistence', executed: 26, queued: 5 },
                { module: 'Collection', executed: 18, queued: 7 },
                { module: 'Exfiltration', executed: 11, queued: 6 }
        ];

        const moduleChartConfig = {
                executed: {
                        label: 'Executed tasks',
                        theme: {
                                light: 'var(--chart-4)',
                                dark: 'var(--chart-4)'
                        }
                },
                queued: {
                        label: 'Queued tasks',
                        theme: {
                                light: 'var(--chart-5)',
                                dark: 'var(--chart-5)'
                        }
                }
        } satisfies ChartConfig;

        const moduleSeries = [
                {
                        key: 'executed',
                        label: moduleChartConfig.executed.label,
                        value: (entry: (typeof moduleActivity)[number]) => entry.executed,
                        color: 'var(--color-executed)'
                },
                {
                        key: 'queued',
                        label: moduleChartConfig.queued.label,
                        value: (entry: (typeof moduleActivity)[number]) => entry.queued,
                        color: 'var(--color-queued)'
                }
        ];

        type LatencyPoint = {
                timestamp: Date;
                p50: number;
                p95: number;
        };

        const latencyTrend: LatencyPoint[] = [
                { timestamp: new Date('2024-12-01T08:00:00Z'), p50: 148, p95: 392 },
                { timestamp: new Date('2024-12-01T09:00:00Z'), p50: 162, p95: 418 },
                { timestamp: new Date('2024-12-01T10:00:00Z'), p50: 171, p95: 441 },
                { timestamp: new Date('2024-12-01T11:00:00Z'), p50: 186, p95: 467 },
                { timestamp: new Date('2024-12-01T12:00:00Z'), p50: 178, p95: 452 },
                { timestamp: new Date('2024-12-01T13:00:00Z'), p50: 169, p95: 423 },
                { timestamp: new Date('2024-12-01T14:00:00Z'), p50: 162, p95: 398 },
                { timestamp: new Date('2024-12-01T15:00:00Z'), p50: 158, p95: 376 }
        ];

        const latencyChartConfig = {
                p50: {
                        label: 'P50 latency',
                        theme: {
                                light: 'var(--chart-2)',
                                dark: 'var(--chart-2)'
                        }
                },
                p95: {
                        label: 'P95 latency',
                        theme: {
                                light: 'var(--chart-1)',
                                dark: 'var(--chart-1)'
                        }
                }
        } satisfies ChartConfig;

        const latencySeries = [
                {
                        key: 'p50',
                        label: latencyChartConfig.p50.label,
                        value: (entry: LatencyPoint) => entry.p50,
                        color: 'var(--color-p50)'
                },
                {
                        key: 'p95',
                        label: latencyChartConfig.p95.label,
                        value: (entry: LatencyPoint) => entry.p95,
                        color: 'var(--color-p95)'
                }
        ];

        const flaggedSessions = [
                {
                        client: 'vela-239',
                        reason: 'Command flood throttled by safeguard',
                        region: 'AMS • 54.210.90.12',
                        interactions: 47
                },
                {
                        client: 'lyra-082',
                        reason: 'Credential cache extracted',
                        region: 'FRA • 185.54.32.77',
                        interactions: 29
                },
                {
                        client: 'solace-441',
                        reason: 'Multiple privilege escalations',
                        region: 'SFO • 34.90.221.14',
                        interactions: 22
                },
                {
                        client: 'nadir-116',
                        reason: 'Dormant beacon rehydrated',
                        region: 'SIN • 103.6.46.220',
                        interactions: 18
                }
        ] as const;
</script>

<section class="space-y-6">
        <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
                {#each summaryMetrics as metric}
                        <Card class="border-border/60">
                                <CardHeader class="space-y-2">
                                        <CardTitle class="text-sm font-medium text-muted-foreground">{metric.label}</CardTitle>
                                        <div class="text-2xl font-semibold">{metric.value}</div>
                                        <p class={`text-xs ${summaryToneClasses[metric.tone]}`}>{metric.delta}</p>
                                </CardHeader>
                        </Card>
                {/each}
        </div>

        <div class="grid gap-6 xl:grid-cols-7">
                <Card class="xl:col-span-4">
                        <CardHeader class="space-y-1.5">
                                <CardTitle>Beacon activity over time</CardTitle>
                                <CardDescription>
                                        Aggregated connection states for the most active clients in the last eight windows.
                                </CardDescription>
                        </CardHeader>
                        <CardContent>
                                <ChartContainer config={activityChartConfig} class="w-full">
                                        <AreaChart
                                                data={activityTimeline}
                                                x={(point) => point.timestamp}
                                                series={activitySeries}
                                                seriesLayout="stack"
                                                props={{
                                                        xAxis: {
                                                                format: (value) =>
                                                                        value instanceof Date ? timeFormatter.format(value) : ''
                                                        },
                                                        yAxis: {
                                                                format: (value) => numberFormatter.format(Number(value ?? 0))
                                                        },
                                                        legend: {
                                                                placement: 'top-right'
                                                        }
                                                }}
                                        >
                                                {#snippet tooltip()}
                                                        <ChartTooltip labelKey="label" indicator="line" />
                                                {/snippet}
                                        </AreaChart>
                                </ChartContainer>
                        </CardContent>
                </Card>

                <Card class="xl:col-span-3">
                        <CardHeader class="flex flex-row items-start justify-between space-y-0">
                                <div class="space-y-1">
                                        <CardTitle>Command latency percentiles</CardTitle>
                                        <CardDescription>
                                                Monitors round-trip times captured from controller to clients.
                                        </CardDescription>
                                </div>
                                <Badge variant="secondary" class="font-mono text-[0.65rem]">
                                        Last 8 intervals
                                </Badge>
                        </CardHeader>
                        <CardContent>
                                <ChartContainer config={latencyChartConfig} class="w-full">
                                        <LineChart
                                                data={latencyTrend}
                                                x={(entry) => entry.timestamp}
                                                series={latencySeries}
                                                props={{
                                                        xAxis: {
                                                                format: (value) =>
                                                                        value instanceof Date ? timeFormatter.format(value) : ''
                                                        },
                                                        yAxis: {
                                                                format: (value) =>
                                                                        `${numberFormatter.format(Number(value ?? 0))} ms`
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
        </div>

        <div class="grid gap-6 lg:grid-cols-7">
                <Card class="lg:col-span-4">
                        <CardHeader class="space-y-1.5">
                                <CardTitle>Module execution mix</CardTitle>
                                <CardDescription>
                                        Compares executed jobs against queued follow-ups for the busiest operator pipelines.
                                </CardDescription>
                        </CardHeader>
                        <CardContent>
                                <ChartContainer config={moduleChartConfig} class="w-full">
                                        <BarChart
                                                data={moduleActivity}
                                                x={(entry) => entry.module}
                                                series={moduleSeries}
                                                seriesLayout="stack"
                                                bandPadding={0.3}
                                                props={{
                                                        yAxis: {
                                                                format: (value) => numberFormatter.format(Number(value ?? 0))
                                                        }
                                                }}
                                        >
                                                {#snippet tooltip()}
                                                        <ChartTooltip />
                                                {/snippet}
                                        </BarChart>
                                </ChartContainer>
                        </CardContent>
                </Card>

                <Card class="lg:col-span-3">
                        <CardHeader class="space-y-1">
                                <CardTitle>Clients needing attention</CardTitle>
                                <CardDescription>
                                        Signals escalations and high-volume sessions that may require intervention.
                                </CardDescription>
                        </CardHeader>
                        <CardContent class="space-y-4">
                                {#each flaggedSessions as session}
                                        <div class="rounded-lg border border-border/60 p-4">
                                                <div class="flex items-start justify-between gap-4">
                                                        <div class="space-y-1">
                                                                <p class="font-medium text-sm uppercase tracking-[0.08em] text-muted-foreground">
                                                                        {session.client}
                                                                </p>
                                                                <p class="text-sm text-foreground">{session.reason}</p>
                                                                <p class="text-xs text-muted-foreground">{session.region}</p>
                                                        </div>
                                                        <Badge variant="outline" class="font-mono text-xs">
                                                                {numberFormatter.format(session.interactions)} ops
                                                        </Badge>
                                                </div>
                                        </div>
                                {/each}
                        </CardContent>
                </Card>
        </div>
</section>
