<script lang="ts">
        import {
                Card,
                CardContent,
                CardDescription,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { buildClientToolUrl, type ClientToolDefinition } from '$lib/data/client-tools';
        import type { PageData } from './$types';

        let { data } = $props<{ data: PageData }>();
        const client = $derived(data.client);
        const tools = $derived((data.tools ?? []) as ClientToolDefinition[]);
</script>

<div class="space-y-6">
        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle>Select a module</CardTitle>
                        <CardDescription>
                                Choose an item from the client context menu to start configuring a capability.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-4 text-sm text-slate-600 dark:text-slate-400">
                        <p>
                                Each module opens in a dedicated workspace so you can prototype workflows without interrupting
                                the main client overview.
                        </p>
                        <p>
                                When features are ready, the Go agent will reuse these routes to negotiate data exchange and
                                streaming sessions.
                        </p>
                </CardContent>
        </Card>

        <Card class="border-slate-200/80 dark:border-slate-800/80">
                <CardHeader>
                        <CardTitle class="text-base">Available modules</CardTitle>
                        <CardDescription>
                                Launch a workspace in a new tab to begin outlining integrations for {client.codename}.
                        </CardDescription>
                </CardHeader>
                <CardContent>
                        <div class="grid gap-3 md:grid-cols-2">
                                {#each tools as item (item.id)}
                                        <a
                                                class="group flex flex-col rounded-lg border border-slate-200/70 bg-white/60 p-4 transition hover:border-sky-400 hover:shadow-sm dark:border-slate-800/70 dark:bg-slate-900/60 dark:hover:border-sky-500"
                                                href={buildClientToolUrl(client.id, item)}
                                                target={item.target === '_blank' ? '_blank' : undefined}
                                                rel={item.target === '_blank' ? 'noopener noreferrer' : undefined}
                                        >
                                                <span class="text-sm font-semibold text-slate-900 transition group-hover:text-sky-600 dark:text-slate-100 dark:group-hover:text-sky-400">
                                                        {item.title}
                                                </span>
                                                <span class="mt-1 line-clamp-2 text-xs text-slate-600 dark:text-slate-400">
                                                        {item.description}
                                                </span>
                                        </a>
                                {/each}
                        </div>
                </CardContent>
        </Card>
</div>
