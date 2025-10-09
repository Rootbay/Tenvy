<script lang="ts">
        import { onDestroy } from 'svelte';
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
        import { Badge } from '$lib/components/ui/badge/index.js';
        import { Textarea } from '$lib/components/ui/textarea/index.js';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import {
                appendWorkspaceLog,
                createWorkspaceLogEntry,
                formatWorkspaceTimestamp
        } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';
        import type { AgentDetailResponse, AgentSnapshot } from '../../../../../../shared/types/agent';
        import type { CommandQueueResponse, CommandResult } from '../../../../../../shared/types/messages';

        type CommandDraft = {
                id: string;
                command: string;
                timeout: number | null;
                elevated: boolean;
                workingDirectory: string;
        };

        const { client, agent } = $props<{ client: Client; agent: AgentSnapshot }>();

        const tool = getClientTool('cmd');
        const defaultWorkingDirectory = getDefaultWorkingDirectory(agent);

        let command = $state('whoami');
        let timeout = $state<number | null>(null);
        let elevated = $state(false);
        let workingDirectory = $state(defaultWorkingDirectory);
        let drafts = $state<CommandDraft[]>([]);
        let log = $state<WorkspaceLogEntry[]>([]);
        const initialHistory = agent.recentResults?.slice(0, 5) ?? [];
        let history = $state<CommandResult[]>(initialHistory);
        let latestResult = $state<CommandResult | null>(initialHistory[0] ?? null);
        let dispatching = $state(false);
        let dispatchError = $state<string | null>(null);
        let agentSnapshot = $state(agent);
        let activePollController: AbortController | null = null;

        $effect(() => {
                elevated;
                if (dispatchError) {
                        dispatchError = null;
                }
        });

        const resultPollIntervalMs = 1500;
        const minimumResultWaitMs = 10_000;

        function getDefaultWorkingDirectory(snapshot: AgentSnapshot): string {
                const os = snapshot.metadata.os?.toLowerCase() ?? '';
                if (os.includes('win')) {
                        return 'C:/Windows/System32';
                }
                if (os.includes('mac')) {
                        return '/usr/bin';
                }
                return '/bin';
        }

        function describeDraft(draft: CommandDraft): string {
                const segments = [draft.command.trim() || '—'];
                if (draft.timeout) segments.push(`${draft.timeout}s timeout`);
                if (draft.elevated) segments.push('run as system');
                segments.push(`cwd ${draft.workingDirectory || defaultWorkingDirectory}`);
                return segments.join(' · ');
        }

        function createDraft(): CommandDraft {
                return {
                        id: `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
                        command,
                        timeout,
                        elevated,
                        workingDirectory: workingDirectory.trim() || defaultWorkingDirectory
                } satisfies CommandDraft;
        }

        function saveDraft() {
                const draft = createDraft();
                drafts = [draft, ...drafts];
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Shell command drafted', describeDraft(draft), 'draft')
                );
        }

        function updateLogEntry(id: string, updates: Partial<WorkspaceLogEntry>) {
                log = log.map((entry) => (entry.id === id ? { ...entry, ...updates } : entry));
        }

        function recordResult(result: CommandResult) {
                latestResult = result;
                const deduped = history.filter((item) => item.commandId !== result.commandId);
                history = [result, ...deduped].slice(0, 5);
        }

        function summarizeOutput(output?: string | null): string | null {
                if (!output) {
                        return null;
                }
                const trimmed = output.trim();
                if (!trimmed) {
                        return null;
                }
                if (trimmed.length <= 160) {
                        return trimmed;
                }
                return `${trimmed.slice(0, 160)}…`;
        }

        function formatResultDetail(result: CommandResult): string {
                const segments = [result.success ? 'Command succeeded' : 'Command failed'];
                if (!result.success && result.error) {
                        segments.push(result.error);
                }
                const snippet = summarizeOutput(result.output);
                if (snippet) {
                        segments.push(snippet);
                }
                return segments.join(' — ');
        }

        function getResultVariant(result: CommandResult | null): 'secondary' | 'destructive' {
                if (!result) {
                        return 'secondary';
                }
                return result.success ? 'secondary' : 'destructive';
        }

        function cancelActivePoll() {
                if (activePollController) {
                        activePollController.abort();
                        activePollController = null;
                }
        }

        async function fetchAgent(signal?: AbortSignal): Promise<AgentSnapshot | null> {
                try {
                        const response = await fetch(`/api/agents/${client.id}`, { signal });
                        if (!response.ok) {
                                console.error(`Failed to refresh agent snapshot: ${response.status}`);
                                return null;
                        }
                        const data = (await response.json()) as AgentDetailResponse;
                        return data.agent;
                } catch (err) {
                        if (err instanceof DOMException && err.name === 'AbortError') {
                                throw err;
                        }
                        console.error('Failed to refresh agent snapshot', err);
                        return null;
                }
        }

        async function waitForCommandResult(commandId: string, draft: CommandDraft): Promise<CommandResult> {
                cancelActivePoll();
                const controller = new AbortController();
                activePollController = controller;

                const start = Date.now();
                const maxWaitMs = Math.max(minimumResultWaitMs, ((draft.timeout ?? 30) + 5) * 1000);

                try {
                        while (!controller.signal.aborted) {
                                const snapshot = await fetchAgent(controller.signal);
                                if (snapshot) {
                                        agentSnapshot = snapshot;
                                        const match = snapshot.recentResults.find(
                                                (result) => result.commandId === commandId
                                        );
                                        if (match) {
                                                return match;
                                        }
                                }

                                if (Date.now() - start > maxWaitMs) {
                                        throw new Error('Timed out waiting for command result');
                                }

                                if (controller.signal.aborted) {
                                        break;
                                }

                                await new Promise((resolve) => setTimeout(resolve, resultPollIntervalMs));
                        }
                } finally {
                        cancelActivePoll();
                }

                if (controller.signal.aborted) {
                        throw new DOMException('Polling aborted', 'AbortError');
                }

                throw new Error('Command polling cancelled');
        }

        async function queueShell() {
                if (dispatching) {
                        return;
                }

                const trimmedCommand = command.trim();
                if (!trimmedCommand) {
                        dispatchError = 'Command is required';
                        return;
                }

                dispatchError = null;

                const draft = createDraft();
                drafts = [draft, ...drafts];

                const logEntry = createWorkspaceLogEntry(
                        'Shell command dispatched',
                        describeDraft(draft),
                        'queued'
                );
                log = appendWorkspaceLog(log, logEntry);

                dispatching = true;

                try {
                        const response = await fetch(`/api/agents/${client.id}/commands`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({
                                        name: 'shell',
                                        payload: {
                                                command: trimmedCommand,
                                                timeoutSeconds: draft.timeout ?? undefined,
                                                workingDirectory: draft.workingDirectory || undefined,
                                                elevated: draft.elevated || undefined
                                        }
                                })
                        });

                        if (!response.ok) {
                                const message = (await response.text()) || 'Failed to queue command';
                                updateLogEntry(logEntry.id, {
                                        status: 'complete',
                                        detail: message.trim()
                                });
                                throw new Error(message.trim());
                        }

                        const data = (await response.json()) as CommandQueueResponse;
                        updateLogEntry(logEntry.id, {
                                status: 'in-progress',
                                detail: 'Awaiting agent execution'
                        });

                        const result = await waitForCommandResult(data.command.id, draft);
                        recordResult(result);
                        updateLogEntry(logEntry.id, {
                                status: 'complete',
                                detail: formatResultDetail(result)
                        });
                        command = '';
                } catch (err) {
                        const message =
                                err instanceof DOMException && err.name === 'AbortError'
                                        ? 'Command polling cancelled'
                                        : err instanceof Error
                                                ? err.message
                                                : 'Failed to run command';
                        dispatchError = message;
                        updateLogEntry(logEntry.id, {
                                status: 'complete',
                                detail: message
                        });
                } finally {
                        dispatching = false;
                }
        }

        onDestroy(() => {
                cancelActivePoll();
        });
</script>

<div class="space-y-6">
        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Command template</CardTitle>
                        <CardDescription>Define the command, execution context, and guardrails before dispatch.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-2">
                                <Label for="cmd-working-directory">Working directory</Label>
                                <Input
                                        id="cmd-working-directory"
                                        bind:value={workingDirectory}
                                        oninput={() => (dispatchError = null)}
                                />
                        </div>
                        <div class="grid gap-2">
                                <Label for="cmd-command">Command</Label>
                                <Textarea
                                        id="cmd-command"
                                        class="min-h-32"
                                        bind:value={command}
                                        placeholder="Invoke-Command"
                                        oninput={() => (dispatchError = null)}
                                />
                        </div>
                        <div class="grid gap-4 sm:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="cmd-timeout">Timeout (seconds)</Label>
                                        <Input
                                                id="cmd-timeout"
                                                type="number"
                                                min={5}
                                                placeholder="Optional"
                                                oninput={(event) => {
                                                        const value = event.currentTarget.value.trim();
                                                        timeout = value === '' ? null : Number.parseInt(value, 10) || null;
                                                        dispatchError = null;
                                                }}
                                        />
                                </div>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Run with system token</p>
                                                <p class="text-xs text-muted-foreground">Escalate privileges when available</p>
                                        </div>
                                        <Switch bind:checked={elevated} />
                                </label>
                        </div>
                </CardContent>
                <CardFooter class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                        <div class="flex flex-wrap gap-3">
                                <Button type="button" variant="outline" onclick={saveDraft}>Save draft</Button>
                                <Button type="button" onclick={queueShell} disabled={dispatching}>
                                        {#if dispatching}
                                                Dispatching…
                                        {:else}
                                                Queue command
                                        {/if}
                                </Button>
                        </div>
                        {#if dispatchError}
                                <p class="text-sm text-destructive">{dispatchError}</p>
                        {/if}
                </CardFooter>
        </Card>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Latest command result</CardTitle>
                        <CardDescription>Review the most recent shell output returned by the agent.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                        {#if latestResult}
                                <div class="flex flex-wrap items-center justify-between gap-3">
                                        <Badge variant={getResultVariant(latestResult)}>
                                                {latestResult.success ? 'Success' : 'Failure'}
                                        </Badge>
                                        <span class="text-xs text-muted-foreground">
                                                Completed {formatWorkspaceTimestamp(latestResult.completedAt)}
                                        </span>
                                </div>
                                {#if latestResult.error && !latestResult.success}
                                        <p class="rounded-md border border-destructive/40 bg-destructive/10 p-3 text-sm text-destructive">
                                                {latestResult.error}
                                        </p>
                                {/if}
                                <pre class="max-h-72 overflow-auto rounded-md border border-border/60 bg-muted/40 p-3 text-xs leading-relaxed text-foreground">{latestResult.output?.trim() || '—'}</pre>
                        {:else}
                                <p class="text-sm text-muted-foreground">
                                        Dispatch a command to retrieve live output from the client.
                                </p>
                        {/if}

                        {#if history.length > 1}
                                <div class="space-y-2">
                                        <h3 class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                                Recent results
                                        </h3>
                                        <ul class="space-y-2">
                                                {#each history.slice(1) as result (result.commandId)}
                                                        <li class="rounded-lg border border-border/60 bg-muted/30 p-3">
                                                                <div class="flex flex-wrap items-center justify-between gap-2 text-xs">
                                                                        <span class="font-medium">
                                                                                {result.success ? 'Success' : 'Failure'}
                                                                        </span>
                                                                        <span class="text-muted-foreground">
                                                                                {formatWorkspaceTimestamp(result.completedAt)}
                                                                        </span>
                                                                </div>
                                                                {#if summarizeOutput(result.output)}
                                                                        <p class="mt-1 text-xs text-muted-foreground">
                                                                                {summarizeOutput(result.output)}
                                                                        </p>
                                                                {/if}
                                                        </li>
                                                {/each}
                                        </ul>
                                </div>
                        {/if}
                </CardContent>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Draft history</CardTitle>
                        <CardDescription>Recently staged commands stay local until you dispatch them.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-3 text-sm">
                        {#if drafts.length === 0}
                                <p class="text-muted-foreground">No commands staged yet.</p>
                        {:else}
                                <ul class="space-y-2">
                                        {#each drafts as draft (draft.id)}
                                                <li class="rounded-lg border border-border/60 bg-muted/40 p-3">
                                                        <p class="font-medium text-foreground">{draft.command}</p>
                                                        <p class="text-xs text-muted-foreground">{describeDraft(draft)}</p>
                                                </li>
                                        {/each}
                                </ul>
                        {/if}
                </CardContent>
        </Card>
</div>
