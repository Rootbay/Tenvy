<script lang="ts">
        import { Badge } from '$lib/components/ui/badge/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { Separator } from '$lib/components/ui/separator/index.js';
        import type { Client } from '$lib/data/clients';
        import type { ClientToolDefinition } from '$lib/data/client-tools';
        import type { Snippet } from 'svelte';

        const {
                client,
                tool,
                segments = tool.segments,
                metadata = [],
                children
        } = $props<{
                client: Client;
                tool: ClientToolDefinition;
                segments?: string[];
                metadata?: { label: string; value: string; hint?: string }[];
                children?: Snippet;
        }>();

        const segmentPath = $derived(segments.join(' / '));
</script>

<Card class="border-slate-200/70 bg-white/70 shadow-sm dark:border-slate-800/70 dark:bg-slate-900/60">
        <CardHeader class="space-y-2">
                <CardTitle class="text-lg font-semibold tracking-tight text-slate-900 dark:text-slate-100">
                        {tool.title}
                </CardTitle>
                <CardDescription class="text-slate-600 dark:text-slate-400">
                        {tool.description}
                </CardDescription>
        </CardHeader>
        <CardContent class="space-y-6 text-sm text-slate-600 dark:text-slate-400">
                <section class="grid gap-4 sm:grid-cols-2">
                        <div class="space-y-2">
                                <h2 class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
                                        Client context
                                </h2>
                                <p class="text-sm text-slate-700 dark:text-slate-300">
                                        <span class="font-medium text-slate-900 dark:text-slate-100">{client.codename}</span>
                                        · {client.hostname}
                                </p>
                                <p>{client.location} · {client.os}</p>
                                <div class="flex flex-wrap gap-2 pt-1">
                                        <Badge variant="outline" class="uppercase">{client.platform}</Badge>
                                        <Badge variant="secondary">Status: {client.status}</Badge>
                                        <Badge
                                                variant={client.risk === 'High'
                                                        ? 'destructive'
                                                        : client.risk === 'Medium'
                                                                ? 'secondary'
                                                                : 'outline'}
                                        >
                                                Risk: {client.risk}
                                        </Badge>
                                </div>
                        </div>
                        <div class="space-y-3">
                                <h2 class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
                                        Metadata
                                </h2>
                                {#if metadata.length === 0}
                                        <p>Workspace metadata will be populated as integration artifacts are defined.</p>
                                {:else}
                                        <dl class="grid gap-2">
                                                {#each metadata as item (item.label)}
                                                        <div class="space-y-1 rounded-lg border border-border/60 bg-muted/40 p-3">
                                                                <dt class="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                                                        {item.label}
                                                                </dt>
                                                                <dd class="text-sm text-foreground">{item.value}</dd>
                                                                {#if item.hint}
                                                                        <p class="text-xs text-muted-foreground">{item.hint}</p>
                                                                {/if}
                                                        </div>
                                                {/each}
                                        </dl>
                                {/if}
                        </div>
                </section>

                <Separator />

                {@render children?.()}

                <section class="space-y-2">
                        <h2 class="text-xs font-semibold uppercase tracking-wide text-slate-500 dark:text-slate-400">
                                Route blueprint
                        </h2>
                        <p class="font-mono text-xs text-slate-500 dark:text-slate-400">modules / {segmentPath}</p>
                </section>
        </CardContent>
</Card>
