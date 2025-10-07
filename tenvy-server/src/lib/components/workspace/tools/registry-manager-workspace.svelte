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

        type RegistryAction = 'read' | 'set' | 'delete';

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('registry-manager');

        let hive = $state('HKEY_LOCAL_MACHINE');
        let path = $state('SOFTWARE/Policies');
        let valueName = $state('Example');
        let dataType = $state<'REG_SZ' | 'REG_DWORD' | 'REG_BINARY'>('REG_SZ');
        let action = $state<RegistryAction>('read');
        let valueData = $state('');
        let createBackup = $state(true);
        let propagateToWow64 = $state(false);
        let log = $state<WorkspaceLogEntry[]>([]);

        function describePlan(): string {
                const segments = [`${action.toUpperCase()} ${hive}\\${path}`, `value ${valueName}`, `type ${dataType}`];
                if (valueData.trim()) segments.push(`data: ${valueData.trim()}`);
                segments.push(createBackup ? 'backup enabled' : 'no backup');
                if (propagateToWow64) segments.push('WOW64 sync');
                return segments.join(' · ');
        }

        function queue(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Registry change staged', describePlan(), status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Hive', value: hive },
                        { label: 'Backup', value: createBackup ? 'Enabled' : 'Disabled' }
                ]}
        >
                <p>
                        Prepare registry manipulations with guardrails for backups and WOW64 mirroring. Operations execute locally
                        for now while the agent API is finalised.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Registry target</CardTitle>
                        <CardDescription>Specify the hive, key path, and intended action.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-4 md:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="registry-hive">Hive</Label>
                                        <Select type="single" value={hive} onValueChange={(value) => (hive = value)}>
                                                <SelectTrigger id="registry-hive" class="w-full">
                                                        <span class="truncate">{hive}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="HKEY_LOCAL_MACHINE">HKEY_LOCAL_MACHINE</SelectItem>
                                                        <SelectItem value="HKEY_CURRENT_USER">HKEY_CURRENT_USER</SelectItem>
                                                        <SelectItem value="HKEY_USERS">HKEY_USERS</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="registry-path">Path</Label>
                                        <Input id="registry-path" bind:value={path} placeholder="SOFTWARE/Policies" />
                                </div>
                        </div>
                        <div class="grid gap-4 md:grid-cols-3">
                                <div class="grid gap-2">
                                        <Label for="registry-action">Action</Label>
                                        <Select
                                                type="single"
                                                value={action}
                                                onValueChange={(value) => (action = value as RegistryAction)}
                                        >
                                                <SelectTrigger id="registry-action" class="w-full">
                                                        <span class="capitalize">{action}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="read">Read</SelectItem>
                                                        <SelectItem value="set">Set</SelectItem>
                                                        <SelectItem value="delete">Delete</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="registry-value">Value name</Label>
                                        <Input id="registry-value" bind:value={valueName} placeholder="Example" />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="registry-type">Data type</Label>
                                        <Select
                                                type="single"
                                                value={dataType}
                                                onValueChange={(value) => (dataType = value as typeof dataType)}
                                        >
                                                <SelectTrigger id="registry-type" class="w-full">
                                                        <span>{dataType}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="REG_SZ">REG_SZ</SelectItem>
                                                        <SelectItem value="REG_DWORD">REG_DWORD</SelectItem>
                                                        <SelectItem value="REG_BINARY">REG_BINARY</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                        </div>
                        {#if action !== 'read'}
                                <div class="grid gap-2">
                                        <Label for="registry-data">Data</Label>
                                        <textarea
                                                id="registry-data"
                                                class="min-h-20 w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus-visible:border-ring focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                                                bind:value={valueData}
                                                placeholder={dataType === 'REG_DWORD' ? '0x00000001' : 'Value'}
                                        ></textarea>
                                </div>
                        {/if}
                        <div class="grid gap-4 md:grid-cols-2">
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Create backup</p>
                                                <p class="text-xs text-muted-foreground">Export key before mutation</p>
                                        </div>
                                        <Switch bind:checked={createBackup} />
                                </label>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Mirror WOW64</p>
                                                <p class="text-xs text-muted-foreground">Apply change to 32-bit view</p>
                                        </div>
                                        <Switch bind:checked={propagateToWow64} />
                                </label>
                        </div>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => queue('queued')}>Queue change</Button>
                </CardFooter>
        </Card>

        <ActionLog entries={log} />
</div>
