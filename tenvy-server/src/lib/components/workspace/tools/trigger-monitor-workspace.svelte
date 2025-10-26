<script lang="ts">
        import { onMount } from 'svelte';
        import { Button } from '$lib/components/ui/button/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
        import {
                Select,
                SelectContent,
                SelectItem,
                SelectTrigger
        } from '$lib/components/ui/select/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardFooter,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { Switch } from '$lib/components/ui/switch/index.js';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import {
                fetchTriggerMonitorStatus,
                updateTriggerMonitorConfig
        } from '$lib/data/trigger-monitor';
        import type { TriggerMonitorMetric } from '$lib/types/trigger-monitor';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('trigger-monitor');
        void tool;

        let feed = $state<'live' | 'batch'>('live');
        let refreshSeconds = $state(5);
        let includeScreenshots = $state(false);
        let includeCommands = $state(true);
        let metrics = $state<TriggerMonitorMetric[]>([]);
        let generatedAt = $state<string | null>(null);
        let log = $state<WorkspaceLogEntry[]>([]);
        let loading = $state(true);
        let loadError = $state<string | null>(null);
        let saving = $state(false);

        function describePlan(): string {
                return `${feed} feed · refresh ${refreshSeconds}s · screenshots ${includeScreenshots ? 'on' : 'off'} · commands${
                        includeCommands ? 'included' : 'excluded'
                }`;
        }

        async function refreshStatus(signal?: AbortSignal) {
                loadError = null;
                loading = true;
                try {
                        const status = await fetchTriggerMonitorStatus(client.id, { signal });
                        feed = status.config.feed;
                        refreshSeconds = status.config.refreshSeconds;
                        includeScreenshots = status.config.includeScreenshots;
                        includeCommands = status.config.includeCommands;
                        metrics = status.metrics;
                        generatedAt = status.generatedAt;
                } catch (err) {
                        loadError = (err as Error).message ?? 'Failed to load trigger monitor status';
                } finally {
                        loading = false;
                }
        }

        async function queue(status: WorkspaceLogEntry['status']) {
                if (status === 'draft') {
                        log = appendWorkspaceLog(
                                log,
                                createWorkspaceLogEntry('Trigger monitor staged', describePlan(), status)
                        );
                        return;
                }

                saving = true;
                loadError = null;
                const detail = describePlan();
                try {
                        const updated = await updateTriggerMonitorConfig(client.id, {
                                feed,
                                refreshSeconds,
                                includeScreenshots,
                                includeCommands
                        });
                        feed = updated.config.feed;
                        refreshSeconds = updated.config.refreshSeconds;
                        includeScreenshots = updated.config.includeScreenshots;
                        includeCommands = updated.config.includeCommands;
                        metrics = updated.metrics;
                        generatedAt = updated.generatedAt;
                        log = appendWorkspaceLog(
                                log,
                                createWorkspaceLogEntry('Trigger monitor updated', detail, 'complete')
                        );
                } catch (err) {
                        const message = (err as Error).message ?? 'Failed to update trigger monitor';
                        loadError = message;
                        log = appendWorkspaceLog(
                                log,
                                createWorkspaceLogEntry('Trigger monitor update failed', message, 'failed')
                        );
                } finally {
                        saving = false;
                }
        }

        onMount(() => {
                const controller = new AbortController();
                void refreshStatus(controller.signal);
                return () => controller.abort();
        });
</script>

<div class="space-y-6">
        {#if loadError}
                <p class="rounded-lg border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
                        {loadError}
                </p>
        {/if}

        <Card>
                <CardHeader class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                        <div>
                                <CardTitle class="text-base">Feed configuration</CardTitle>
                                <CardDescription>
                                        Adjust how frequently telemetry is collected and which channels are enabled.
                                </CardDescription>
                        </div>
                        <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onclick={() => refreshStatus()}
                                disabled={loading || saving}
                        >
                                Refresh
                        </Button>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-4 md:grid-cols-3">
                                <div class="grid gap-2">
                                        <Label for="report-feed">Feed type</Label>
                                        <Select
                                                type="single"
                                                value={feed}
                                                onValueChange={(value) => (feed = value as typeof feed)}
                                                disabled={saving}
                                        >
                                                <SelectTrigger id="report-feed" class="w-full">
                                                        <span class="capitalize">{feed}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="live">Live</SelectItem>
                                                        <SelectItem value="batch">Batch</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="report-refresh">Refresh (seconds)</Label>
                                        <Input
                                                id="report-refresh"
                                                type="number"
                                                min={feed === 'live' ? 2 : 30}
                                                step={feed === 'live' ? 1 : 30}
                                                bind:value={refreshSeconds}
                                                disabled={saving}
                                        />
                                </div>
                                <label
                                        class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3"
                                >
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Include command stream</p>
                                                <p class="text-xs text-muted-foreground">Show queued and completed commands</p>
                                        </div>
                                        <Switch bind:checked={includeCommands} disabled={saving} />
                                </label>
                        </div>
                        <label
                                class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2"
                        >
                                <div>
                                        <p class="text-sm font-medium text-foreground">Embed screenshots</p>
                                        <p class="text-xs text-muted-foreground">Attach periodic mini screenshots to the feed</p>
                                </div>
                                <Switch bind:checked={includeScreenshots} disabled={saving} />
                        </label>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => queue('draft')} disabled={saving}
                                >Save draft</Button
                        >
                        <Button type="button" onclick={() => queue('queued')} disabled={saving}
                                >{saving ? 'Updating…' : 'Queue workspace'}</Button
                        >
                </CardFooter>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Telemetry metrics</CardTitle>
                        <CardDescription>
                                Latest metrics reported by the agent.
                                {#if generatedAt}
                                        <span class="ml-2 text-xs text-muted-foreground">Generated {generatedAt}</span>
                                {/if}
                        </CardDescription>
                </CardHeader>
                <CardContent class="grid gap-4 md:grid-cols-3">
                        {#if loading}
                                <p class="col-span-full rounded-lg border border-border/40 bg-muted/30 p-3 text-muted-foreground">
                                        Loading telemetry…
                                </p>
                        {:else if metrics.length === 0}
                                <p class="col-span-full rounded-lg border border-border/60 bg-muted/30 p-3 text-muted-foreground">
                                        No telemetry has been reported yet.
                                </p>
                        {:else}
                                {#each metrics as metric (metric.id)}
                                        <div class="rounded-lg border border-border/60 bg-muted/30 p-4">
                                                <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                                                        {metric.label}
                                                </p>
                                                <p class="mt-2 text-lg font-semibold text-foreground">{metric.value}</p>
                                        </div>
                                {/each}
                        {/if}
                </CardContent>
        </Card>

        {#if log.length > 0}
                <Card>
                        <CardHeader>
                                <CardTitle class="text-base">Activity</CardTitle>
                                <CardDescription>Recent trigger monitor actions.</CardDescription>
                        </CardHeader>
                        <CardContent class="space-y-2 text-sm">
                                <ul class="space-y-2">
                                        {#each log as entry (entry.id)}
                                                <li class="rounded-lg border border-border/60 bg-muted/30 p-3">
                                                        <p class="font-medium text-foreground">{entry.action}</p>
                                                        <p class="text-xs text-muted-foreground">
                                                                Status: {entry.status} · {entry.timestamp}
                                                        </p>
                                                        {#if entry.detail}
                                                                <p class="text-xs text-muted-foreground">{entry.detail}</p>
                                                        {/if}
                                                </li>
                                        {/each}
                                </ul>
                        </CardContent>
                </Card>
        {/if}
</div>
