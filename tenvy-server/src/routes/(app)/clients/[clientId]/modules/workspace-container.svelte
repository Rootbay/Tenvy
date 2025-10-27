<script lang="ts">
        import { goto } from '$app/navigation';
        import { resolve } from '$app/paths';
        import { Button } from '$lib/components/ui/button/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { Separator } from '$lib/components/ui/separator/index.js';
        import { cn } from '$lib/utils.js';
        import type { Client } from '$lib/data/clients';
        import { buildClientToolUrl, type ClientToolDefinition } from '$lib/data/client-tools';
        import ClientToolWorkspace from '$lib/components/workspace/client-tool-workspace.svelte';
        import { isWorkspaceTool } from '$lib/data/client-tool-workspaces';
        import type { AgentSnapshot } from '../../../../../../shared/types/agent';
        import { ArrowLeft, X } from '@lucide/svelte';

        const { client, agent = null, tools, activeTool = null, segments = [] } = $props<{
                client: Client;
                agent?: AgentSnapshot | null;
                tools: ClientToolDefinition[];
                activeTool?: ClientToolDefinition | null;
                segments?: string[];
        }>();

        const baseModulesUrl = `/clients/${client.id}/modules`;

        const categoryLabels: Record<string, string> = {
                overview: 'Overview',
                control: 'Control',
                management: 'Management',
                operations: 'Operations',
                misc: 'Miscellaneous',
                'system-controls': 'System Controls',
                power: 'Power'
        };

        type Group = { key: string; label: string; items: ClientToolDefinition[] };

        const groupedTools = $derived(() => {
                const order: Group[] = [];
                const index = new Map<string, Group>();

                for (const tool of tools) {
                        const key = tool.segments[0] ?? 'misc';
                        let group = index.get(key);
                        if (!group) {
                                group = {
                                        key,
                                        label: categoryLabels[key] ?? key.replace(/-/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase()),
                                        items: []
                                } satisfies Group;
                                index.set(key, group);
                                order.push(group);
                        }
                        group.items.push(tool);
                }

                return order.map((group) => ({
                        ...group,
                        items: group.items.slice()
                }));
        });

        const activeToolId = $derived(activeTool?.id ?? null);

        function toWorkspaceUrl(tool: ClientToolDefinition) {
                        return resolve(buildClientToolUrl(client.id, tool));
        }

        function closeWorkspace() {
                goto(baseModulesUrl);
        }

        function returnToClients() {
                goto('/clients');
        }
</script>

<section class="space-y-6">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <div>
                        <h1 class="text-2xl font-semibold tracking-tight">{client.codename}</h1>
                        <p class="text-sm text-muted-foreground">
                                Manage {client.codename}&rsquo;s capabilities without leaving the client workspace.
                        </p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                        <Button variant="outline" onclick={returnToClients} class="gap-2">
                                <ArrowLeft class="h-4 w-4" />
                                <span>Client overview</span>
                        </Button>
                        {#if activeTool}
                                <Button variant="secondary" onclick={closeWorkspace} class="gap-2">
                                        <X class="h-4 w-4" />
                                        <span>Close workspace</span>
                                </Button>
                        {/if}
                </div>
        </div>

        <div class="grid gap-6 lg:grid-cols-[260px_minmax(0,1fr)]">
                <aside class="space-y-6 rounded-lg border border-border/60 bg-background/40 p-4">
                        {#each groupedTools as group (group.key)}
                                <div class="space-y-2">
                                        <p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                                {group.label}
                                        </p>
                                        <div class="flex flex-col gap-1">
                                                {#each group.items as item (item.id)}
                                                        {@const isActive = activeToolId === item.id}
                                                        <a
                                                                class={cn(
                                                                        'flex items-center justify-between rounded-md border border-transparent px-3 py-2 text-sm transition hover:border-primary/40 hover:bg-primary/5',
                                                                        isActive
                                                                                ? 'border-primary/60 bg-primary/10 text-primary'
                                                                                : 'text-muted-foreground'
                                                                )}
                                                                href={toWorkspaceUrl(item)}
                                                        >
                                                                <span class="truncate">{item.title}</span>
                                                                {#if isWorkspaceTool(item.id)}
                                                                        <span class={cn('text-[0.65rem] font-medium uppercase tracking-wide', isActive ? 'text-primary' : 'text-muted-foreground/70')}>
                                                                                Workspace
                                                                        </span>
                                                                {/if}
                                                        </a>
                                                {/each}
                                        </div>
                                </div>
                                {#if group !== groupedTools.at(-1)}
                                        <Separator />
                                {/if}
                        {/each}
                </aside>

                <div class="space-y-4">
                        {#if activeTool}
                                <Card class="border-border/60 bg-background/60 shadow-sm">
                                        <CardHeader class="space-y-1">
                                                <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                                                        <div class="space-y-1">
                                                                <CardTitle>{activeTool.title}</CardTitle>
                                                                {#if segments.length > 0}
                                                                        <CardDescription>{segments.join(' / ')}</CardDescription>
                                                                {/if}
                                                        </div>
                                                        <div class="flex items-center gap-2">
                                                                <Button variant="outline" size="sm" onclick={closeWorkspace} class="gap-2">
                                                                        <X class="h-4 w-4" />
                                                                        <span>Close</span>
                                                                </Button>
                                                        </div>
                                                </div>
                                        </CardHeader>
                                        <CardContent class="space-y-4">
                                                {#key `${client.id}-${activeTool.id}`}
                                                        <ClientToolWorkspace {client} {agent} tool={activeTool} />
                                                {/key}
                                        </CardContent>
                                </Card>
                        {:else}
                                <slot name="empty">
                                        <Card class="border-dashed">
                                                <CardHeader>
                                                        <CardTitle>Select a module</CardTitle>
                                                        <CardDescription>
                                                                Choose a capability to launch its dedicated workspace for {client.codename}.
                                                        </CardDescription>
                                                </CardHeader>
                                                <CardContent class="space-y-3 text-sm text-muted-foreground">
                                                        <p>
                                                                Workspaces preserve each tool&rsquo;s state while you evaluate remote workflows.
                                                        </p>
                                                        <p>
                                                                Use the navigation panel to switch between modules or close the workspace when you&rsquo;re done.
                                                        </p>
                                                </CardContent>
                                        </Card>
                                </slot>
                        {/if}
                </div>
        </div>
</section>
