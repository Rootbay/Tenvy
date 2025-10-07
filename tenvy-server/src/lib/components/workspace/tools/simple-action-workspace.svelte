<script lang="ts">
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
        import ClientWorkspaceHero from '$lib/components/workspace/workspace-hero.svelte';
        import ActionLog from '$lib/components/workspace/action-log.svelte';
        import { getClientTool, type ClientToolId } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        type Variant = 'system-control' | 'power';

        const {
                client,
                toolId,
                variant
        } = $props<{ client: Client; toolId: ClientToolId; variant: Variant }>();

        const tool = getClientTool(toolId);

        let delaySeconds = $state(variant === 'power' ? 10 : 3);
        let requireConfirmation = $state(variant !== 'power');
        let notifyUser = $state(false);
        let note = $state('');
        let log = $state<WorkspaceLogEntry[]>([]);

        function describeAction(): string {
                const segments = [`delay ${delaySeconds}s`, requireConfirmation ? 'confirmation required' : 'immediate'];
                if (notifyUser) segments.push('user notified');
                if (note.trim()) segments.push(`note: ${note.trim()}`);
                return segments.join(' · ');
        }

        function queue(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry(`${tool.title} planned`, describeAction(), status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        { label: 'Action scope', value: variant === 'power' ? 'Host power state' : 'Agent transport' },
                        { label: 'Confirmation', value: requireConfirmation ? 'Required' : 'Optional' }
                ]}
        >
                <p>
                        Prototype how {tool.title.toLowerCase()} should be queued and audited. The action is simulated locally until
                        the command dispatcher is bound to these intents.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Execution window</CardTitle>
                        <CardDescription>Control how quickly the action is dispatched to the agent.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-2 md:w-1/3">
                                <Label for={`${toolId}-delay`}>Delay (seconds)</Label>
                                <Input
                                        id={`${toolId}-delay`}
                                        type="number"
                                        min={0}
                                        step={1}
                                        bind:value={delaySeconds}
                                />
                        </div>
                        <div class="grid gap-4 md:grid-cols-2">
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Require confirmation</p>
                                                <p class="text-xs text-muted-foreground">Gate the action behind an additional prompt</p>
                                        </div>
                                        <Switch bind:checked={requireConfirmation} />
                                </label>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Notify user</p>
                                                <p class="text-xs text-muted-foreground">Surface a visible notification prior to execution</p>
                                        </div>
                                        <Switch bind:checked={notifyUser} />
                                </label>
                        </div>
                        <div class="grid gap-2">
                                <Label for={`${toolId}-note`}>Operator note</Label>
                                <textarea
                                        id={`${toolId}-note`}
                                        bind:value={note}
                                        class="min-h-24 w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus-visible:border-ring focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                                        placeholder="Document the reason or maintenance ticket reference."
                                ></textarea>
                        </div>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => queue('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => queue('queued')}>Queue action</Button>
                </CardFooter>
        </Card>

        <ActionLog entries={log} />
</div>
