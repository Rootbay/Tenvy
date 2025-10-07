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

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('options');

        let beaconInterval = $state(30);
        let jitterPercent = $state(15);
        let autoUpdate = $state(true);
        let safeMode = $state(false);
        let allowPlugins = $state(true);
        let restrictToVpn = $state(false);
        let log = $state<WorkspaceLogEntry[]>([]);

        function describePlan(): string {
                return `beacon ${beaconInterval}s ±${jitterPercent}% · auto-update ${autoUpdate ? 'on' : 'off'} · safe mode ${safeMode ? 'on' : 'off'} · plugins ${allowPlugins ? 'enabled' : 'disabled'}`;
        }

        function queue(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Client options staged', describePlan(), status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Beacon interval', value: `${beaconInterval}s` },
                        { label: 'Safe mode', value: safeMode ? 'Enabled' : 'Disabled' }
                ]}
        >
                <p>
                        Draft runtime option changes such as beacon cadence, plugin permissions, and safety switches. Updates are
                        simulated locally until the control channel supports live mutations.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Heartbeat</CardTitle>
                        <CardDescription>Adjust connection intervals and jitter ranges.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                        <div class="grid gap-4 md:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="options-beacon">Beacon interval (seconds)</Label>
                                        <Input
                                                id="options-beacon"
                                                type="number"
                                                min={5}
                                                step={5}
                                                bind:value={beaconInterval}
                                        />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="options-jitter">Jitter (%)</Label>
                                        <Input
                                                id="options-jitter"
                                                type="number"
                                                min={0}
                                                max={100}
                                                bind:value={jitterPercent}
                                        />
                                </div>
                        </div>
                </CardContent>
        </Card>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Behavior</CardTitle>
                        <CardDescription>Toggle runtime behavior flags.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Automatic updates</p>
                                        <p class="text-xs text-muted-foreground">Allow background binary refreshes</p>
                                </div>
                                <Switch bind:checked={autoUpdate} />
                        </label>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Safe mode</p>
                                        <p class="text-xs text-muted-foreground">Disable destructive capabilities</p>
                                </div>
                                <Switch bind:checked={safeMode} />
                        </label>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Allow plugins</p>
                                        <p class="text-xs text-muted-foreground">Permit runtime plugin activation</p>
                                </div>
                                <Switch bind:checked={allowPlugins} />
                        </label>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Restrict to VPN</p>
                                        <p class="text-xs text-muted-foreground">Only operate when controller is on VPN subnets</p>
                                </div>
                                <Switch bind:checked={restrictToVpn} />
                        </label>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => queue('queued')}>Queue update</Button>
                </CardFooter>
        </Card>

        <ActionLog entries={log} />
</div>
