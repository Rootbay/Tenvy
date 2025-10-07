<script lang="ts">
        import { Button } from '$lib/components/ui/button/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
        import {
                Select,
                SelectContent,
                SelectItem,
                SelectTrigger
        } from '$lib/components/ui/select/index.js';
        import { Switch } from '$lib/components/ui/switch/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardFooter,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import ClientWorkspaceHero from '$lib/components/workspace/workspace-hero.svelte';
        import ActionLog from '$lib/components/workspace/action-log.svelte';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        type ProcessRow = {
                pid: number;
                name: string;
                cpu: number;
                memory: number;
                user: string;
        };

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('task-manager');

        let filter = $state('');
        let sort = $state<'cpu' | 'memory' | 'name'>('cpu');
        let autoRefresh = $state(true);
        let sampleInterval = $state(15);
        let log = $state<WorkspaceLogEntry[]>([]);
        let rows = $state<ProcessRow[]>([
                { pid: 4120, name: 'explorer.exe', cpu: 1.5, memory: 224, user: 'USER\\operator' },
                { pid: 5220, name: 'svchost.exe', cpu: 3.2, memory: 178, user: 'SYSTEM' },
                { pid: 872, name: 'powershell.exe', cpu: 0.4, memory: 96, user: 'USER\\operator' }
        ]);

        function pollSnapshot(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Task snapshot requested', `filter "${filter}" · sort ${sort} · interval ${sampleInterval}s`, status)
                );
        }

        const filteredRows = $derived(
                rows
                        .filter((row) => row.name.toLowerCase().includes(filter.toLowerCase()))
                        .sort((a, b) => {
                                if (sort === 'cpu') return b.cpu - a.cpu;
                                if (sort === 'memory') return b.memory - a.memory;
                                return a.name.localeCompare(b.name);
                        })
        );
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Auto refresh', value: autoRefresh ? 'Enabled' : 'Paused' },
                        { label: 'Sample interval', value: `${sampleInterval}s` }
                ]}
        >
                <p>
                        Outline remote process monitoring, including polling cadence and isolation actions. The live data is
                        simulated until the agent streams process snapshots.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Process filters</CardTitle>
                        <CardDescription>Configure how snapshots are collected and filtered.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-4 md:grid-cols-3">
                                <div class="grid gap-2">
                                        <Label for="task-filter">Process filter</Label>
                                        <Input id="task-filter" bind:value={filter} placeholder="powershell.exe" />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="task-sort">Sort by</Label>
                                        <Select
                                                type="single"
                                                value={sort}
                                                onValueChange={(value) => (sort = value as typeof sort)}
                                        >
                                                <SelectTrigger id="task-sort" class="w-full">
                                                        <span class="capitalize">{sort}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="cpu">CPU</SelectItem>
                                                        <SelectItem value="memory">Memory</SelectItem>
                                                        <SelectItem value="name">Name</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="task-interval">Sample interval (s)</Label>
                                        <Input
                                                id="task-interval"
                                                type="number"
                                                min={5}
                                                step={5}
                                                bind:value={sampleInterval}
                                        />
                                </div>
                        </div>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Auto refresh</p>
                                        <p class="text-xs text-muted-foreground">Pull a new snapshot automatically</p>
                                </div>
                                <Switch bind:checked={autoRefresh} />
                        </label>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => pollSnapshot('draft')}>Save configuration</Button>
                        <Button type="button" onclick={() => pollSnapshot('queued')}>Poll snapshot</Button>
                </CardFooter>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Simulated snapshot</CardTitle>
                        <CardDescription>Example data that mirrors the final table output.</CardDescription>
                </CardHeader>
                <CardContent class="overflow-hidden rounded-lg border border-border/60 text-sm">
                        <table class="w-full divide-y divide-border/60">
                                <thead class="bg-muted/30">
                                        <tr class="text-left">
                                                <th class="px-4 py-2 font-medium">PID</th>
                                                <th class="px-4 py-2 font-medium">Process</th>
                                                <th class="px-4 py-2 font-medium">CPU %</th>
                                                <th class="px-4 py-2 font-medium">Memory (MB)</th>
                                                <th class="px-4 py-2 font-medium">User</th>
                                        </tr>
                                </thead>
                                <tbody>
                                        {#each filteredRows as row (row.pid)}
                                                <tr class="odd:bg-muted/20">
                                                        <td class="px-4 py-2 font-mono">{row.pid}</td>
                                                        <td class="px-4 py-2">{row.name}</td>
                                                        <td class="px-4 py-2">{row.cpu.toFixed(1)}</td>
                                                        <td class="px-4 py-2">{row.memory}</td>
                                                        <td class="px-4 py-2">{row.user}</td>
                                                </tr>
                                        {/each}
                                </tbody>
                        </table>
                </CardContent>
        </Card>

        <ActionLog entries={log} />
</div>
