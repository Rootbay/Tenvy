<script lang="ts">
        import { onMount } from 'svelte';
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
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import {
                fetchEnvironmentSnapshot,
                setEnvironmentVariable,
                removeEnvironmentVariable
        } from '$lib/data/environment';
        import type { EnvironmentVariable, EnvironmentMutationResult } from '$lib/types/environment';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';
        import { Trash2 } from '@lucide/svelte';

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('environment-variables');
        void tool;

        let variables = $state<EnvironmentVariable[]>([]);
        let filter = $state('');
        let newKey = $state('');
        let newValue = $state('');
        let newScope = $state<'machine' | 'user'>('user');
        let restartProcess = $state(false);
        let log = $state<WorkspaceLogEntry[]>([]);
        let loading = $state(true);
        let loadError = $state<string | null>(null);
        let mutationError = $state<string | null>(null);
        let saving = $state(false);
        let removingKey = $state<string | null>(null);
        let lastCapturedAt = $state<string | null>(null);

        const filteredVariables = $derived(
                variables.filter((item) => item.key.toLowerCase().includes(filter.toLowerCase()))
        );

        function updateVariablesFromSnapshot(snapshot: { variables: EnvironmentVariable[]; capturedAt: string }) {
                variables = snapshot.variables;
                lastCapturedAt = snapshot.capturedAt;
        }

        async function refreshEnvironment(signal?: AbortSignal) {
                loadError = null;
                loading = true;
                try {
                        const snapshot = await fetchEnvironmentSnapshot(client.id, { signal });
                        updateVariablesFromSnapshot(snapshot);
                } catch (err) {
                        loadError = (err as Error).message ?? 'Failed to load environment variables';
                } finally {
                        loading = false;
                }
        }

        function applyMutation(result: EnvironmentMutationResult) {
                const existing = variables.filter((variable) => variable.key !== result.key);
                if (result.operation === 'set' && result.value !== undefined) {
                        const entry: EnvironmentVariable = {
                                key: result.key,
                                value: result.value,
                                scope: result.scope,
                                length: result.value.length,
                                lastModifiedAt: result.mutatedAt
                        };
                        variables = [entry, ...existing].sort((a, b) => a.key.localeCompare(b.key));
                        return;
                }
                variables = existing;
        }

        async function queueVariable() {
                const key = newKey.trim();
                if (!key) {
                        mutationError = 'Environment variable key is required';
                        return;
                }
                mutationError = null;
                saving = true;
                const detail = `${key.toUpperCase()} (${newScope})`;
                try {
                        const result = await setEnvironmentVariable(client.id, {
                                key,
                                value: newValue,
                                scope: newScope,
                                restartProcesses: restartProcess
                        });
                        applyMutation(result);
                        log = appendWorkspaceLog(
                                log,
                                createWorkspaceLogEntry('Environment variable updated', detail, 'complete')
                        );
                        newKey = '';
                        newValue = '';
                } catch (err) {
                        const message = (err as Error).message ?? 'Failed to update variable';
                        mutationError = message;
                        log = appendWorkspaceLog(
                                log,
                                createWorkspaceLogEntry('Environment variable update failed', message, 'failed')
                        );
                } finally {
                        saving = false;
                }
        }

        async function removeVariable(variable: EnvironmentVariable) {
                removingKey = variable.key;
                mutationError = null;
                try {
                        const result = await removeEnvironmentVariable(client.id, {
                                key: variable.key,
                                scope: variable.scope
                        });
                        applyMutation(result);
                        log = appendWorkspaceLog(
                                log,
                                createWorkspaceLogEntry(
                                        'Environment variable removed',
                                        `${variable.key} (${variable.scope})`,
                                        'complete'
                                )
                        );
                } catch (err) {
                        const message = (err as Error).message ?? 'Failed to remove variable';
                        mutationError = message;
                        log = appendWorkspaceLog(
                                log,
                                createWorkspaceLogEntry('Environment variable remove failed', message, 'failed')
                        );
                } finally {
                        removingKey = null;
                }
        }

        function recordDraft() {
                const key = newKey.trim();
                if (!key) {
                        mutationError = 'Environment variable key is required';
                        return;
                }
                mutationError = null;
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry(
                                'Environment variable staged',
                                `${key.toUpperCase()} (${newScope})`,
                                'draft'
                        )
                );
        }

        onMount(() => {
                const controller = new AbortController();
                void refreshEnvironment(controller.signal);
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
                                <CardTitle class="text-base">Manage variables</CardTitle>
                                <CardDescription>
                                        Queue updates for environment variables and request agent restarts when needed.
                                </CardDescription>
                        </div>
                        <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                onclick={() => refreshEnvironment()}
                                disabled={loading}
                        >
                                Refresh
                        </Button>
                </CardHeader>
                <CardContent class="space-y-4">
                        {#if mutationError}
                                <p class="rounded-md border border-destructive/30 bg-destructive/10 p-2 text-sm text-destructive">
                                        {mutationError}
                                </p>
                        {/if}
                        <div class="grid gap-2 md:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="env-key">Key</Label>
                                        <Input
                                                id="env-key"
                                                bind:value={newKey}
                                                placeholder="LOG_LEVEL"
                                                disabled={saving}
                                        />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="env-scope">Scope</Label>
                                        <select
                                                id="env-scope"
                                                class="h-9 w-full rounded-md border border-border/60 bg-background px-3 text-sm"
                                                bind:value={newScope}
                                                disabled={saving}
                                        >
                                                <option value="user">User</option>
                                                <option value="machine">Machine</option>
                                        </select>
                                </div>
                        </div>
                        <div class="grid gap-2">
                                <Label for="env-value">Value</Label>
                                <Input
                                        id="env-value"
                                        bind:value={newValue}
                                        placeholder="debug"
                                        disabled={saving}
                                />
                        </div>
                        <label
                                class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2"
                        >
                                <div>
                                        <p class="text-sm font-medium text-foreground">Restart affected processes</p>
                                        <p class="text-xs text-muted-foreground">Trigger reload after applying the variable</p>
                                </div>
                                <Switch bind:checked={restartProcess} disabled={saving} />
                        </label>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={recordDraft} disabled={saving}
                                >Save draft</Button
                        >
                        <Button type="button" onclick={queueVariable} disabled={saving}
                                >{saving ? 'Queuing…' : 'Queue update'}</Button
                        >
                </CardFooter>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Existing variables</CardTitle>
                        <CardDescription>
                                Filter and manage tracked variables.
                                {#if lastCapturedAt}
                                        <span class="ml-2 text-xs text-muted-foreground">Snapshot: {lastCapturedAt}</span>
                                {/if}
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-4 text-sm">
                        <div class="grid gap-2 md:w-1/2">
                                <Label for="env-filter">Filter</Label>
                                <Input
                                        id="env-filter"
                                        bind:value={filter}
                                        placeholder="PATH"
                                        disabled={loading}
                                />
                        </div>
                        {#if loading}
                                <p class="rounded-lg border border-border/40 bg-muted/30 p-3 text-muted-foreground">
                                        Loading environment snapshot…
                                </p>
                        {:else if filteredVariables.length === 0}
                                <p class="rounded-lg border border-border/60 bg-muted/30 p-3 text-muted-foreground">
                                        No variables match your filter.
                                </p>
                        {:else}
                                <ul class="space-y-2">
                                        {#each filteredVariables as variable (variable.key)}
                                                <li class="rounded-lg border border-border/60 bg-muted/30 p-3">
                                                        <div class="flex items-start justify-between gap-3">
                                                                <div>
                                                                        <p class="font-medium text-foreground">{variable.key}</p>
                                                                        <p class="text-xs text-muted-foreground break-words">
                                                                                {variable.value}
                                                                        </p>
                                                                        <p class="text-xs text-muted-foreground">
                                                                                Scope: {variable.scope}
                                                                                {#if variable.lastModifiedAt}
                                                                                        · Updated {variable.lastModifiedAt}
                                                                                {/if}
                                                                        </p>
                                                                </div>
                                                                <Button
                                                                        type="button"
                                                                        size="icon"
                                                                        variant="ghost"
                                                                        aria-label={`Remove ${variable.key}`}
                                                                        onclick={() => removeVariable(variable)}
                                                                        disabled={removingKey === variable.key}
                                                                >
                                                                        <Trash2 class="h-4 w-4" />
                                                                </Button>
                                                        </div>
                                                </li>
                                        {/each}
                                </ul>
                        {/if}
                </CardContent>
        </Card>

        {#if log.length > 0}
                <Card>
                        <CardHeader>
                                <CardTitle class="text-base">Activity</CardTitle>
                                <CardDescription>Recent actions performed in this workspace.</CardDescription>
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
