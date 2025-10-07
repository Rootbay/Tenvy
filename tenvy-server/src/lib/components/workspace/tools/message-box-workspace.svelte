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

        const tool = getClientTool('message-box');

        let title = $state('System notice');
        let body = $state('');
        let style = $state<'info' | 'warning' | 'error'>('info');
        let requireAck = $state(true);
        let includeCountdown = $state(false);
        let log = $state<WorkspaceLogEntry[]>([]);

        function describePlan(): string {
                return `${title} · ${style} · ack ${requireAck ? 'required' : 'optional'}${includeCountdown ? ' · countdown' : ''}`;
        }

        function queue(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Message drafted', describePlan(), status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Delivery style', value: style },
                        { label: 'Acknowledgement', value: requireAck ? 'Required' : 'Optional' }
                ]}
        >
                <p>
                        Extend the message box dialog with richer delivery semantics. The request remains local until acknowledgem
                        ent capture is wired to the agent.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Message content</CardTitle>
                        <CardDescription>Compose the message and choose the presentation style.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-2">
                                <Label for="message-title">Title</Label>
                                <Input id="message-title" bind:value={title} />
                        </div>
                        <div class="grid gap-2">
                                <Label for="message-body">Body</Label>
                                <textarea
                                        id="message-body"
                                        class="min-h-32 w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus-visible:border-ring focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                                        bind:value={body}
                                        placeholder="Enter the message shown to the user"
                                ></textarea>
                        </div>
                        <div class="grid gap-4 md:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="message-style">Style</Label>
                                        <Select type="single" value={style} onValueChange={(value) => (style = value as typeof style)}>
                                                <SelectTrigger id="message-style" class="w-full">
                                                        <span class="capitalize">{style}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="info">Info</SelectItem>
                                                        <SelectItem value="warning">Warning</SelectItem>
                                                        <SelectItem value="error">Error</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Countdown timer</p>
                                                <p class="text-xs text-muted-foreground">Display a dismissal countdown</p>
                                        </div>
                                        <input
                                                type="checkbox"
                                                class="size-4 rounded border border-border/60"
                                                bind:checked={includeCountdown}
                                        />
                                </label>
                        </div>
                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Require acknowledgement</p>
                                        <p class="text-xs text-muted-foreground">Prevent dismissal without operator confirmation</p>
                                </div>
                                <input
                                        type="checkbox"
                                        class="size-4 rounded border border-border/60"
                                        bind:checked={requireAck}
                                />
                        </label>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => queue('queued')}>Queue message</Button>
                </CardFooter>
        </Card>

        <ActionLog entries={log} />
</div>
