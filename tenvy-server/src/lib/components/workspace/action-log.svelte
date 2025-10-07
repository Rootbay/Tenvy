<script lang="ts">
        import { Badge } from '$lib/components/ui/badge/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import type { WorkspaceLogEntry, WorkspaceLogStatus } from '$lib/workspace/types';
        import { formatWorkspaceTimestamp } from '$lib/workspace/utils';

        const {
                entries,
                title = 'Action log',
                description = 'Simulated execution trail for the planned automation.'
        } = $props<{
                entries: WorkspaceLogEntry[];
                title?: string;
                description?: string;
        }>();

        const statusVariant: Record<WorkspaceLogStatus, 'default' | 'secondary' | 'outline' | 'destructive'> = {
                queued: 'default',
                draft: 'outline',
                'in-progress': 'secondary',
                complete: 'secondary'
        };

        function getStatusVariant(status: WorkspaceLogStatus) {
                return statusVariant[status];
        }
</script>

<Card class="border-slate-200/60 bg-white/70 shadow-sm dark:border-slate-800/60 dark:bg-slate-900/60">
        <CardHeader>
                <CardTitle class="text-base font-semibold text-slate-900 dark:text-slate-100">{title}</CardTitle>
                <CardDescription class="text-slate-600 dark:text-slate-400">{description}</CardDescription>
        </CardHeader>
        <CardContent class="space-y-4">
                {#if entries.length === 0}
                        <p class="text-sm text-muted-foreground">
                                Log entries appear here once you stage actions from this workspace.
                        </p>
                {:else}
                        <ul class="space-y-3">
                                {#each entries as entry (entry.id)}
                                        <li class="rounded-lg border border-border/60 bg-muted/40 p-4">
                                                <div class="flex flex-wrap items-center justify-between gap-2">
                                                        <div class="flex items-center gap-2">
                                                                <Badge variant={getStatusVariant(entry.status)} class="uppercase">
                                                                        {entry.status.replace('-', ' ')}
                                                                </Badge>
                                                                <span class="text-sm font-medium text-foreground">{entry.action}</span>
                                                        </div>
                                                        <span class="text-xs font-medium text-muted-foreground">
                                                                {formatWorkspaceTimestamp(entry.timestamp)}
                                                        </span>
                                                </div>
                                                {#if entry.detail}
                                                        <p class="mt-2 text-sm leading-relaxed text-muted-foreground">{entry.detail}</p>
                                                {/if}
                                        </li>
                                {/each}
                        </ul>
                {/if}
        </CardContent>
</Card>
