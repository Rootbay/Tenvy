<script lang="ts">
        import { invalidateAll } from '$app/navigation';
        import { Badge } from '$lib/components/ui/badge/index.js';
        import { Button } from '$lib/components/ui/button/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardFooter,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Separator } from '$lib/components/ui/separator/index.js';
        import type { AgentSnapshot } from '../../../../../shared/types/agent';

        const statusLabels: Record<AgentSnapshot['status'], string> = {
                online: 'Online',
                offline: 'Offline',
                error: 'Error'
        };

        const statusClasses: Record<AgentSnapshot['status'], string> = {
                online: 'border border-emerald-500/30 bg-emerald-500/10 text-emerald-500',
                offline: 'border border-slate-500/30 bg-slate-500/10 text-slate-400',
                error: 'border border-red-500/30 bg-red-500/10 text-red-500'
        };

        let { data } = $props<{ data: { agents: AgentSnapshot[] } }>();
        const agents = $derived(data.agents ?? []);

        let pingMessages = $state<Record<string, string>>({});
        let shellCommands = $state<Record<string, string>>({});
        let shellTimeouts = $state<Record<string, number | undefined>>({});
        let commandErrors = $state<Record<string, string | null>>({});
        let commandSuccess = $state<Record<string, string | null>>({});
        let commandPending = $state<Record<string, boolean>>({});

        function updateRecord<T>(records: Record<string, T>, key: string, value: T): Record<string, T> {
                return { ...records, [key]: value };
        }

        function formatDate(value: string): string {
                const date = new Date(value);
                if (Number.isNaN(date.getTime())) {
                        return 'Unknown';
                }
                return date.toLocaleString();
        }

        function formatRelative(value: string): string {
                const date = new Date(value);
                if (Number.isNaN(date.getTime())) {
                        return 'unknown';
                }
                const diffMs = Date.now() - date.getTime();
                if (diffMs <= 0) {
                        return 'just now';
                }
                const diffSeconds = Math.floor(diffMs / 1000);
                const units: [Intl.RelativeTimeFormatUnit, number][] = [
                        ['day', 86_400],
                        ['hour', 3_600],
                        ['minute', 60],
                        ['second', 1]
                ];
                for (const [unit, seconds] of units) {
                        if (diffSeconds >= seconds || unit === 'second') {
                                const value = Math.floor(diffSeconds / seconds) * -1;
                                return new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' }).format(value, unit);
                        }
                }
                return 'just now';
        }

        function formatBytes(value?: number): string {
                if (!value) {
                        return '—';
                }
                const units = ['B', 'KB', 'MB', 'GB', 'TB'];
                let idx = 0;
                let current = value;
                while (current >= 1024 && idx < units.length - 1) {
                        current /= 1024;
                        idx += 1;
                }
                return `${current.toFixed(idx === 0 ? 0 : 1)} ${units[idx]}`;
        }

        function formatDuration(seconds?: number): string {
                if (!seconds) {
                        return '—';
                }
                const hours = Math.floor(seconds / 3600);
                const minutes = Math.floor((seconds % 3600) / 60);
                const secs = Math.floor(seconds % 60);
                const parts = [];
                if (hours > 0) parts.push(`${hours}h`);
                if (minutes > 0 || hours > 0) parts.push(`${minutes}m`);
                parts.push(`${secs}s`);
                return parts.join(' ');
        }

        function getError(key: string): string | null {
                return commandErrors[key] ?? null;
        }

        function getSuccess(key: string): string | null {
                return commandSuccess[key] ?? null;
        }

        function isPending(key: string): boolean {
                return commandPending[key] ?? false;
        }

        async function queueCommand(
                agentId: string,
                body: unknown,
                key: string,
                successMessage: string
        ): Promise<boolean> {
                commandPending = updateRecord(commandPending, key, true);
                commandErrors = updateRecord(commandErrors, key, null);
                commandSuccess = updateRecord(commandSuccess, key, null);

                try {
                        const response = await fetch(`/api/agents/${agentId}/commands`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify(body)
                        });
                        if (!response.ok) {
                                const message = (await response.text()) || 'Failed to queue command';
                                commandErrors = updateRecord(commandErrors, key, message.trim());
                                return false;
                        }
                        commandSuccess = updateRecord(commandSuccess, key, successMessage);
                        await invalidateAll();
                        return true;
                } catch (err) {
                        commandErrors = updateRecord(
                                commandErrors,
                                key,
                                err instanceof Error ? err.message : 'Unknown error'
                        );
                        return false;
                } finally {
                        commandPending = updateRecord(commandPending, key, false);
                }
        }

        async function sendPing(agentId: string) {
                const key = `ping:${agentId}`;
                const message = pingMessages[agentId]?.trim();
                const success = await queueCommand(
                        agentId,
                        {
                                name: 'ping',
                                payload: message ? { message } : {}
                        },
                        key,
                        'Ping queued'
                );
                if (success) {
                        pingMessages = updateRecord(pingMessages, agentId, '');
                }
        }

        async function sendShell(agentId: string) {
                const key = `shell:${agentId}`;
                const command = shellCommands[agentId]?.trim();
                if (!command) {
                        commandErrors = updateRecord(commandErrors, key, 'Command is required');
                        return;
                }

                const timeout = shellTimeouts[agentId];
                const payload: { command: string; timeoutSeconds?: number } = { command };
                if (timeout && timeout > 0) {
                        payload.timeoutSeconds = timeout;
                }

                const success = await queueCommand(
                        agentId,
                        {
                                name: 'shell',
                                payload
                        },
                        key,
                        'Shell command queued'
                );

                if (success) {
                        shellCommands = updateRecord(shellCommands, agentId, '');
                }
        }
</script>

<svelte:head>
        <title>Clients · Tenvy</title>
</svelte:head>

<section class="space-y-6">
        <header class="space-y-2">
                <h1 class="text-2xl font-semibold tracking-tight">Clients</h1>
                <p class="text-sm text-muted-foreground">
                        Manage connected agents, dispatch commands and inspect recent activity.
                </p>
        </header>

        {#if agents.length === 0}
                <Card class="border-dashed border-border/60">
                        <CardHeader>
                                <CardTitle>No agents connected</CardTitle>
                                <CardDescription>
                                        Launch a client instance to have it register and appear here automatically.
                                </CardDescription>
                        </CardHeader>
                </Card>
        {/if}

        {#each agents as agent (agent.id)}
                <Card class="border-border/60">
                        <CardHeader class="gap-2">
                                <div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
                                        <div>
                                                <CardTitle class="text-xl font-semibold">
                                                        {agent.metadata.hostname}
                                                        <span class="ml-2 text-sm font-medium text-muted-foreground">
                                                                {agent.metadata.username}@{agent.metadata.os}
                                                        </span>
                                                </CardTitle>
                                                <CardDescription class="text-sm">
                                                        Agent ID: <code>{agent.id}</code>
                                                </CardDescription>
                                        </div>
                                        <Badge class={`rounded-md px-2 py-1 text-xs font-semibold ${statusClasses[agent.status]}`}>
                                                {statusLabels[agent.status]}
                                        </Badge>
                                </div>
                                <div class="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
                                        <span>Connected {formatDate(agent.connectedAt)}</span>
                                        <span aria-hidden="true">•</span>
                                        <span>Last seen {formatRelative(agent.lastSeen)}</span>
                                        {#if agent.metadata.ipAddress}
                                                <span aria-hidden="true">•</span>
                                                <span>IP {agent.metadata.ipAddress}</span>
                                        {/if}
                                        {#if agent.metadata.tags?.length}
                                                <span aria-hidden="true">•</span>
                                                <span>Tags: {agent.metadata.tags.join(', ')}</span>
                                        {/if}
                                </div>
                        </CardHeader>
                        <CardContent class="space-y-6">
                                <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
                                        <section class="space-y-4">
                                                <div>
                                                        <h3 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                                                                Metrics
                                                        </h3>
                                                        <Separator class="my-2" />
                                                        <dl class="grid gap-2 text-sm">
                                                                <div class="flex justify-between">
                                                                        <dt class="text-muted-foreground">Memory</dt>
                                                                        <dd class="font-medium">
                                                                                {formatBytes(agent.metrics?.memoryBytes)}
                                                                        </dd>
                                                                </div>
                                                                <div class="flex justify-between">
                                                                        <dt class="text-muted-foreground">Goroutines</dt>
                                                                        <dd class="font-medium">
                                                                                {agent.metrics?.goroutines ?? '—'}
                                                                        </dd>
                                                                </div>
                                                                <div class="flex justify-between">
                                                                        <dt class="text-muted-foreground">Uptime</dt>
                                                                        <dd class="font-medium">
                                                                                {formatDuration(agent.metrics?.uptimeSeconds)}
                                                                        </dd>
                                                                </div>
                                                        </dl>
                                                </div>

                                                <div class="space-y-2">
                                                        <h3 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                                                                Pending commands
                                                        </h3>
                                                        <Separator class="my-2" />
                                                        {#if agent.pendingCommands === 0}
                                                                <p class="text-sm text-muted-foreground">Queue is empty.</p>
                                                        {:else}
                                                                <p class="text-sm font-medium">{agent.pendingCommands} awaiting pickup</p>
                                                        {/if}
                                                </div>

                                                <div class="space-y-2">
                                                        <h3 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                                                                Recent results
                                                        </h3>
                                                        <Separator class="my-2" />
                                                        {#if agent.recentResults.length === 0}
                                                                <p class="text-sm text-muted-foreground">No results yet.</p>
                                                        {:else}
                                                                <ul class="space-y-3 text-sm">
                                                                        {#each agent.recentResults.slice(0, 5) as result (result.commandId)}
                                                                                <li class="rounded-md border border-border/60 p-3">
                                                                                        <div class="flex items-center justify-between text-xs text-muted-foreground">
                                                                                                <span>#{result.commandId}</span>
                                                                                                <span>{formatDate(result.completedAt)}</span>
                                                                                        </div>
                                                                                        <p class={`mt-2 font-medium ${result.success ? 'text-emerald-500' : 'text-red-500'}`}>
                                                                                                {result.success ? 'Success' : 'Failed'}
                                                                                        </p>
                                                                                        {#if result.output}
                                                                                                <pre class="mt-2 whitespace-pre-wrap rounded bg-muted/60 p-2 text-xs text-muted-foreground">
{result.output}
                                                                                                </pre>
                                                                                        {/if}
                                                                                        {#if result.error}
                                                                                                <p class="mt-2 text-xs text-red-500">{result.error}</p>
                                                                                        {/if}
                                                                                </li>
                                                                        {/each}
                                                                </ul>
                                                        {/if}
                                                </div>
                                        </section>

                                        <section class="space-y-6">
                                                <div class="space-y-3 rounded-lg border border-border/60 bg-background/60 p-4">
                                                        <div>
                                                                <h3 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                                                                        Ping
                                                                </h3>
                                                                <p class="text-sm text-muted-foreground">
                                                                        Sends a keep-alive message to verify the agent connection.
                                                                </p>
                                                        </div>
                                                        <div class="flex flex-col gap-3 sm:flex-row">
                                                                <Input
                                                                        placeholder="Optional message"
                                                                        value={pingMessages[agent.id] ?? ''}
                                                                        oninput={(event) =>
                                                                                (pingMessages = updateRecord(
                                                                                        pingMessages,
                                                                                        agent.id,
                                                                                        event.currentTarget.value
                                                                                ))
                                                                        }
                                                                />
                                                                <Button
                                                                        type="button"
                                                                        onclick={() => sendPing(agent.id)}
                                                                        disabled={isPending(`ping:${agent.id}`)}
                                                                >
                                                                        {#if isPending(`ping:${agent.id}`)}
                                                                                Sending…
                                                                        {:else}
                                                                                Send ping
                                                                        {/if}
                                                                </Button>
                                                        </div>
                                                        {#if getError(`ping:${agent.id}`)}
                                                                <p class="text-sm text-red-500">{getError(`ping:${agent.id}`)}</p>
                                                        {:else if getSuccess(`ping:${agent.id}`)}
                                                                <p class="text-sm text-emerald-500">{getSuccess(`ping:${agent.id}`)}</p>
                                                        {/if}
                                                </div>

                                                <div class="space-y-3 rounded-lg border border-border/60 bg-background/60 p-4">
                                                        <div>
                                                                <h3 class="text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                                                                        Shell command
                                                                </h3>
                                                                <p class="text-sm text-muted-foreground">
                                                                        Execute a command on the remote system via the agent shell module.
                                                                </p>
                                                        </div>
                                                        <div class="space-y-3">
                                                                <textarea
                                                                        class="w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary"
                                                                        rows={3}
                                                                        placeholder="whoami"
                                                                        value={shellCommands[agent.id] ?? ''}
                                                                        oninput={(event) =>
                                                                                (shellCommands = updateRecord(
                                                                                        shellCommands,
                                                                                        agent.id,
                                                                                        event.currentTarget.value
                                                                                ))
                                                                        }
                                                                ></textarea>
                                                                <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
                                                                        <div class="flex items-center gap-2">
                                                                                <Input
                                                                                        type="number"
                                                                                        min={1}
                                                                                        placeholder="Timeout (s)"
                                                                                        value={shellTimeouts[agent.id] ?? ''}
                                                                                        oninput={(event) => {
                                                                                                const raw = event.currentTarget.value;
                                                                                                const value = Number.parseInt(raw, 10);
                                                                                                shellTimeouts = updateRecord(
                                                                                                        shellTimeouts,
                                                                                                        agent.id,
                                                                                                        raw.trim() === '' || Number.isNaN(value)
                                                                                                                ? undefined
                                                                                                                : value
                                                                                                );
                                                                                        }}
                                                                                />
                                                                                <span class="text-xs text-muted-foreground">Optional</span>
                                                                        </div>
                                                                        <Button
                                                                                type="button"
                                                                                onclick={() => sendShell(agent.id)}
                                                                                disabled={isPending(`shell:${agent.id}`)}
                                                                        >
                                                                                {#if isPending(`shell:${agent.id}`)}
                                                                                        Dispatching…
                                                                                {:else}
                                                                                        Execute command
                                                                                {/if}
                                                                        </Button>
                                                                </div>
                                                        </div>
                                                        {#if getError(`shell:${agent.id}`)}
                                                                <p class="text-sm text-red-500">{getError(`shell:${agent.id}`)}</p>
                                                        {:else if getSuccess(`shell:${agent.id}`)}
                                                                <p class="text-sm text-emerald-500">{getSuccess(`shell:${agent.id}`)}</p>
                                                        {/if}
                                                </div>
                                        </section>
                                </div>
                        </CardContent>
                        <CardFooter class="justify-between text-xs text-muted-foreground">
                                <span>Total results tracked: {agent.recentResults.length}</span>
                                <span>Last updated {formatDate(agent.lastSeen)}</span>
                        </CardFooter>
                </Card>
        {/each}
</section>
