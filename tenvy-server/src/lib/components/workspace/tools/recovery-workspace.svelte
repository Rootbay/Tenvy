<script lang="ts">
        import { Button } from '$lib/components/ui/button/index.js';
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

        const tool = getClientTool('recovery');

        let captureBrowserSecrets = $state(true);
        let captureWifiProfiles = $state(true);
        let captureVaultCredentials = $state(false);
        let captureRdpHistory = $state(true);
        let offlineOnly = $state(false);
        let log = $state<WorkspaceLogEntry[]>([]);

        function describePlan(): string {
                const segments = [
                        captureBrowserSecrets ? 'browser secrets' : null,
                        captureWifiProfiles ? 'wifi profiles' : null,
                        captureVaultCredentials ? 'credential vault' : null,
                        captureRdpHistory ? 'RDP history' : null
                ]
                        .filter(Boolean)
                        .join(', ');
                return `${segments || 'no targets'} · ${offlineOnly ? 'offline staging' : 'immediate upload'}`;
        }

        function queue(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Recovery plan staged', describePlan(), status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Offline staging', value: offlineOnly ? 'Enabled' : 'Disabled' }
                ]}
        >
                <p>
                        Prototype credential and configuration recovery. Collection targets are simulated until a secure export
                        path is negotiated with the agent.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Recovery targets</CardTitle>
                        <CardDescription>Choose the data classes to extract.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Browser secrets</p>
                                        <p class="text-xs text-muted-foreground">Decrypt Chromium/Firefox password stores</p>
                                </div>
                                <Switch bind:checked={captureBrowserSecrets} />
                        </label>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Wi-Fi profiles</p>
                                        <p class="text-xs text-muted-foreground">Extract SSIDs and WPA credentials</p>
                                </div>
                                <Switch bind:checked={captureWifiProfiles} />
                        </label>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Credential vault</p>
                                        <p class="text-xs text-muted-foreground">Stage DPAPI master key workflow</p>
                                </div>
                                <Switch bind:checked={captureVaultCredentials} />
                        </label>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">RDP history</p>
                                        <p class="text-xs text-muted-foreground">Collect mstsc cache + JumpList items</p>
                                </div>
                                <Switch bind:checked={captureRdpHistory} />
                        </label>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Offline staging</p>
                                        <p class="text-xs text-muted-foreground">Store artefacts locally until manually exfiltrated</p>
                                </div>
                                <Switch bind:checked={offlineOnly} />
                        </label>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => queue('queued')}>Queue recovery</Button>
                </CardFooter>
        </Card>

        <ActionLog entries={log} />
</div>
