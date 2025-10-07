<script lang="ts">
        import { Button } from '$lib/components/ui/button/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
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

        type StartupEntry = {
                id: string;
                name: string;
                path: string;
                enabled: boolean;
                scope: 'machine' | 'user';
        };

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('startup-manager');

        let entries = $state<StartupEntry[]>([
                {
                        id: 'svc-1',
                        name: 'TelemetryBridge',
                        path: 'C:/ProgramData/telemetry/bridge.exe',
                        enabled: true,
                        scope: 'machine'
                },
                {
                        id: 'svc-2',
                        name: 'UpdateMonitor',
                        path: 'C:/Users/operator/AppData/Roaming/updatemonitor.exe',
                        enabled: false,
                        scope: 'user'
                }
        ]);
        let newName = $state('');
        let newPath = $state('');
        let newScope = $state<'machine' | 'user'>('machine');
        let log = $state<WorkspaceLogEntry[]>([]);

        function toggleEntry(entry: StartupEntry, enabled: boolean) {
                entries = entries.map((item) => (item.id === entry.id ? { ...item, enabled } : item));
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Startup entry toggled', `${entry.name} → ${enabled ? 'enabled' : 'disabled'}`)
                );
        }

        function addEntry(status: WorkspaceLogEntry['status']) {
                if (!newName.trim() || !newPath.trim()) {
                        return;
                }
                const entry: StartupEntry = {
                        id: `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
                        name: newName.trim(),
                        path: newPath.trim(),
                        enabled: true,
                        scope: newScope
                };
                entries = [entry, ...entries];
                newName = '';
                newPath = '';
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Startup entry drafted', `${entry.name} (${entry.scope})`, status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Tracked entries', value: entries.length.toString() },
                        {
                                label: 'User scope entries',
                                value: entries.filter((item) => item.scope === 'user').length.toString()
                        }
                ]}
        >
                <p>
                        Prototype startup persistence orchestration. Entries are simulated locally and will synchronise with the
                        agent once registry and scheduled task bindings are exposed.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Add startup entry</CardTitle>
                        <CardDescription>Define an executable to run when the client session begins.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                        <div class="grid gap-2">
                                <Label for="startup-name">Name</Label>
                                <Input id="startup-name" bind:value={newName} placeholder="TelemetryBridge" />
                        </div>
                        <div class="grid gap-2">
                                <Label for="startup-path">Executable path</Label>
                                <Input id="startup-path" bind:value={newPath} placeholder="C:/Program Files/App/app.exe" />
                        </div>
                        <div class="grid gap-2 md:w-1/3">
                                <Label for="startup-scope">Scope</Label>
                                <select
                                        id="startup-scope"
                                        class="h-9 w-full rounded-md border border-border/60 bg-background px-3 text-sm"
                                        bind:value={newScope}
                                >
                                        <option value="machine">Machine</option>
                                        <option value="user">User</option>
                                </select>
                        </div>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => addEntry('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => addEntry('queued')}>Queue addition</Button>
                </CardFooter>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Tracked entries</CardTitle>
                        <CardDescription>Toggle entries to simulate enabling or disabling persistence.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-3 text-sm">
                        {#if entries.length === 0}
                                <p class="text-muted-foreground">No startup entries defined.</p>
                        {:else}
                                <ul class="space-y-2">
                                        {#each entries as entry (entry.id)}
                                                <li class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/40 p-3">
                                                        <div class="space-y-1">
                                                                <p class="font-medium text-foreground">{entry.name}</p>
                                                                <p class="text-xs text-muted-foreground">{entry.path}</p>
                                                                <p class="text-xs text-muted-foreground">Scope: {entry.scope}</p>
                                                        </div>
                                                        <Switch
                                                                checked={entry.enabled}
                                                                onCheckedChange={(value) => toggleEntry(entry, value)}
                                                                aria-label={`Toggle ${entry.name}`}
                                                        />
                                                </li>
                                        {/each}
                                </ul>
                        {/if}
                </CardContent>
        </Card>

        <ActionLog entries={log} />
</div>
