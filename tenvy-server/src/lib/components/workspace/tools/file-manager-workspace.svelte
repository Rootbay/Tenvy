<script lang="ts">
        import { Button } from '$lib/components/ui/button/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
        import { Switch } from '$lib/components/ui/switch/index.js';
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
        import ClientWorkspaceHero from '$lib/components/workspace/workspace-hero.svelte';
        import ActionLog from '$lib/components/workspace/action-log.svelte';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        type TransferDraft = {
                id: string;
                root: string;
                mode: 'download' | 'upload' | 'sync';
                includeHidden: boolean;
                concurrency: number;
        };

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('file-manager');

        let root = $state('C:/');
        let mode = $state<TransferDraft['mode']>('download');
        let includeHidden = $state(true);
        let concurrency = $state(3);
        let createIntegrityHashes = $state(true);
        let mirrorTimestamps = $state(true);
        let drafts = $state<TransferDraft[]>([]);
        let log = $state<WorkspaceLogEntry[]>([]);

        function describeDraft(draft: TransferDraft): string {
                return `${draft.mode} ${draft.root} · concurrency ${draft.concurrency} · hidden ${draft.includeHidden ? 'yes' : 'no'} · timestamps ${mirrorTimestamps ? 'preserved' : 'ignored'}`;
        }

        function stage(status: WorkspaceLogEntry['status']) {
                const draft: TransferDraft = {
                        id: `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
                        root,
                        mode,
                        includeHidden,
                        concurrency
                };
                drafts = [draft, ...drafts];
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('File transfer drafted', describeDraft(draft), status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Mirror timestamps', value: mirrorTimestamps ? 'Enabled' : 'Disabled' },
                        { label: 'Integrity hashes', value: createIntegrityHashes ? 'Enabled' : 'Disabled' }
                ]}
        >
                <p>
                        Plan remote file operations, including mirrored sync and staged transfers. Each draft will eventually map
                        to task executions once the agent exposes file APIs.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Transfer scope</CardTitle>
                        <CardDescription>Describe which directories are synced and how aggressively.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-2">
                                <Label for="file-root">Root path</Label>
                                <Input id="file-root" bind:value={root} placeholder="C:/Users/Public" />
                        </div>
                        <div class="grid gap-4 md:grid-cols-3">
                                <div class="grid gap-2">
                                        <Label for="file-mode">Mode</Label>
                                        <Select
                                                type="single"
                                                value={mode}
                                                onValueChange={(value) => (mode = value as TransferDraft['mode'])}
                                        >
                                                <SelectTrigger id="file-mode" class="w-full">
                                                        <span class="capitalize">{mode}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="download">Download</SelectItem>
                                                        <SelectItem value="upload">Upload</SelectItem>
                                                        <SelectItem value="sync">Two-way sync</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Include hidden entries</p>
                                                <p class="text-xs text-muted-foreground">Transfer hidden/system files</p>
                                        </div>
                                        <Switch bind:checked={includeHidden} />
                                </label>
                                <div class="grid gap-2">
                                        <Label for="file-concurrency">Concurrency</Label>
                                        <Input
                                                id="file-concurrency"
                                                type="number"
                                                min={1}
                                                max={8}
                                                bind:value={concurrency}
                                        />
                                </div>
                        </div>
                        <div class="grid gap-4 md:grid-cols-2">
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Mirror timestamps</p>
                                                <p class="text-xs text-muted-foreground">Apply original modified times on download</p>
                                        </div>
                                        <Switch bind:checked={mirrorTimestamps} />
                                </label>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Generate hashes</p>
                                                <p class="text-xs text-muted-foreground">Produce SHA-256 manifests post-transfer</p>
                                        </div>
                                        <Switch bind:checked={createIntegrityHashes} />
                                </label>
                        </div>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => stage('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => stage('queued')}>Queue transfer</Button>
                </CardFooter>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Draft queue</CardTitle>
                        <CardDescription>Operations are tracked locally until agent bindings are ready.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-3 text-sm">
                        {#if drafts.length === 0}
                                <p class="text-muted-foreground">No file operations staged.</p>
                        {:else}
                                <ul class="space-y-2">
                                        {#each drafts as draft (draft.id)}
                                                <li class="rounded-lg border border-border/60 bg-muted/40 p-3">
                                                        <p class="font-medium text-foreground">{draft.mode} · {draft.root}</p>
                                                        <p class="text-xs text-muted-foreground">{describeDraft(draft)}</p>
                                                </li>
                                        {/each}
                                </ul>
                        {/if}
                </CardContent>
        </Card>

        <ActionLog entries={log} />
</div>
