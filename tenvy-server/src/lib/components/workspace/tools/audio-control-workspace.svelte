<script lang="ts">
        import { onDestroy, onMount } from 'svelte';
        import { Button } from '$lib/components/ui/button/index.js';
        import { Badge } from '$lib/components/ui/badge/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardFooter,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import type {
                AudioDeviceDescriptor,
                AudioDeviceInventory,
                AudioSessionState,
                AudioStreamChunk
        } from '$lib/types/audio';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('audio-control');
        const isBrowser = typeof window !== 'undefined';

        let inventory = $state<AudioDeviceInventory | null>(null);
        let pendingInventory = $state(false);
        let refreshingInventory = $state(false);
        let inventoryError = $state<string | null>(null);

        let session = $state<AudioSessionState | null>(null);
        let listening = $state(false);
        let sessionError = $state<string | null>(null);
        let playbackError = $state<string | null>(null);

        let log = $state<WorkspaceLogEntry[]>([]);

        const heroMetadata = $derived([
                {
                        label: 'Inputs discovered',
                        value: inventory ? inventory.inputs.length.toString() : pendingInventory ? 'Pending' : '0'
                },
                {
                        label: 'Session state',
                        value: session ? (session.active ? 'Active' : 'Stopped') : 'Idle',
                        hint: listening ? 'Streaming live microphone audio to the controller.' : undefined
                }
        ]);

        let eventSource: EventSource | null = null;
        let audioContext: AudioContext | null = null;
        let playbackQueueTime = 0;

        function logAction(action: string, detail: string, status: WorkspaceLogEntry['status'] = 'complete') {
                log = appendWorkspaceLog(log, createWorkspaceLogEntry(action, detail, status));
        }

        async function loadInventory() {
                if (!isBrowser) {
                        return;
                }
                try {
                        const response = await fetch(`/api/agents/${client.id}/audio/devices`);
                        if (!response.ok) {
                                throw new Error(`Status ${response.status}`);
                        }
                        const body = (await response.json()) as { inventory?: AudioDeviceInventory | null; pending?: boolean };
                        inventory = body.inventory ?? null;
                        pendingInventory = Boolean(body.pending);
                        inventoryError = null;
                } catch (err) {
                        inventoryError = (err as Error).message ?? 'Failed to load audio inventory.';
                }
        }

        async function pollInventory(requestId: string) {
                const deadline = Date.now() + 15_000;
                while (Date.now() < deadline) {
                        await new Promise((resolve) => setTimeout(resolve, 1000));
                        await loadInventory();
                        if (inventory && (!inventory.requestId || inventory.requestId === requestId)) {
                                pendingInventory = false;
                                logAction('Audio inventory updated', inventory.capturedAt);
                                return;
                        }
                }
                logAction('Audio inventory refresh timed out', requestId, 'failed');
        }

        async function refreshInventory() {
                if (!isBrowser || refreshingInventory) {
                        return;
                }
                refreshingInventory = true;
                inventoryError = null;
                try {
                        const response = await fetch(`/api/agents/${client.id}/audio/devices/refresh`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' }
                        });
                        if (!response.ok) {
                                throw new Error(`Status ${response.status}`);
                        }
                        const body = (await response.json()) as { requestId: string };
                        pendingInventory = true;
                        logAction('Requested audio inventory refresh', body.requestId, 'pending');
                        await pollInventory(body.requestId);
                } catch (err) {
                        inventoryError = (err as Error).message ?? 'Failed to refresh audio inventory.';
                        logAction('Audio inventory refresh failed', inventoryError ?? '', 'failed');
                } finally {
                        refreshingInventory = false;
                }
        }

        async function loadSessionState() {
                if (!isBrowser) {
                        return;
                }
                try {
                        const response = await fetch(`/api/agents/${client.id}/audio/session`);
                        if (!response.ok) {
                                throw new Error(`Status ${response.status}`);
                        }
                        const body = (await response.json()) as { session: AudioSessionState | null };
                        session = body.session ?? null;
                        if (session?.active) {
                                listening = true;
                                openStream(session.sessionId);
                        }
                } catch (err) {
                        sessionError = (err as Error).message ?? 'Failed to load audio session state.';
                }
        }

        async function startListening(device: AudioDeviceDescriptor) {
                if (!isBrowser) {
                        return;
                }
                if (device.kind !== 'input') {
                        sessionError = 'Only input devices can be monitored currently.';
                        return;
                }
                const label = device.label ?? device.id;
                await stopListening(true);
                sessionError = null;
                playbackError = null;
                try {
                        const response = await fetch(`/api/agents/${client.id}/audio/session`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({
                                        deviceId: device.id,
                                        deviceLabel: device.label,
                                        direction: device.kind
                                })
                        });
                        if (!response.ok) {
                                throw new Error(`Status ${response.status}`);
                        }
                        const body = (await response.json()) as { session: AudioSessionState | null };
                        session = body.session;
                        if (!session) {
                                throw new Error('Session was not created.');
                        }
                        listening = true;
                        logAction('Audio session started', label);
                        openStream(session.sessionId);
                } catch (err) {
                        listening = false;
                        sessionError = (err as Error).message ?? 'Failed to start audio session.';
                        logAction('Audio session start failed', sessionError ?? '', 'failed');
                }
        }

        async function stopListening(silent = false) {
                if (!isBrowser) {
                        cleanupPlayback();
                        listening = false;
                        return;
                }
                const current = session;
                const label = current?.deviceLabel ?? current?.sessionId ?? 'session';
                if (!current) {
                        cleanupPlayback();
                        listening = false;
                        return;
                }
                try {
                        const response = await fetch(`/api/agents/${client.id}/audio/session`, {
                                method: 'DELETE',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({ sessionId: current.sessionId })
                        });
                        if (response.ok) {
                                const body = (await response.json()) as { session: AudioSessionState | null };
                                session = body.session ?? null;
                        }
                } catch (err) {
                        if (!silent) {
                                sessionError = (err as Error).message ?? 'Failed to stop audio session.';
                        }
                } finally {
                        cleanupPlayback();
                        listening = false;
                        if (!silent) {
                                logAction('Audio session stopped', label);
                        }
                }
        }

        function openStream(sessionId: string) {
                if (!isBrowser) {
                        return;
                }
                cleanupPlayback();
                const url = new URL(`/api/agents/${client.id}/audio/stream`, window.location.origin);
                url.searchParams.set('sessionId', sessionId);
                const source = new EventSource(url.toString());
                eventSource = source;
                source.addEventListener('session', (event) => {
                        const detail = JSON.parse((event as MessageEvent<string>).data) as AudioSessionState;
                        session = detail;
                });
                source.addEventListener('chunk', (event) => {
                        const detail = JSON.parse((event as MessageEvent<string>).data) as AudioStreamChunk;
                        void handleChunk(detail);
                });
                source.addEventListener('stopped', () => {
                        listening = false;
                        playbackError = 'Audio session ended.';
                        if (session) {
                                session = { ...session, active: false } satisfies AudioSessionState;
                        }
                        cleanupPlayback();
                });
                source.onerror = () => {
                        playbackError = 'Audio stream interrupted.';
                };
        }

        function cleanupPlayback() {
                if (eventSource) {
                        eventSource.close();
                        eventSource = null;
                }
                if (audioContext) {
                        audioContext.close().catch(() => {});
                        audioContext = null;
                }
                playbackQueueTime = 0;
        }

        async function ensureAudioContext(sampleRate: number): Promise<boolean> {
                if (!isBrowser) {
                        return false;
                }
                if (!audioContext) {
                        try {
                                audioContext = new AudioContext();
                        } catch {
                                playbackError = 'Audio playback is not supported in this environment.';
                                return false;
                        }
                        playbackQueueTime = audioContext.currentTime;
                }
                if (audioContext.state === 'suspended') {
                        try {
                                await audioContext.resume();
                        } catch {
                                // ignore resume errors
                        }
                }
                // AudioBuffer resamples automatically to the context sample rate.
                void sampleRate;
                return true;
        }

        function decodePcm(data: string): Int16Array | null {
                try {
                        const binary = atob(data);
                        if (binary.length % 2 !== 0) {
                                return null;
                        }
                        const buffer = new ArrayBuffer(binary.length);
                        const bytes = new Uint8Array(buffer);
                        for (let i = 0; i < binary.length; i += 1) {
                                bytes[i] = binary.charCodeAt(i);
                        }
                        return new Int16Array(buffer);
                } catch {
                        return null;
                }
        }

        function schedulePlayback(format: AudioStreamChunk['format'], pcm: Int16Array) {
                if (!audioContext) {
                        return;
                }
                const channels = Math.max(1, Math.min(2, format.channels ?? 1));
                const frameCount = Math.floor(pcm.length / channels);
                if (frameCount <= 0) {
                        return;
                }
                const buffer = audioContext.createBuffer(channels, frameCount, format.sampleRate);
                for (let channel = 0; channel < channels; channel += 1) {
                        const channelData = buffer.getChannelData(channel);
                        for (let frame = 0; frame < frameCount; frame += 1) {
                                const sampleIndex = frame * channels + channel;
                                const value = pcm[sampleIndex] / 32768;
                                channelData[frame] = Math.max(-1, Math.min(1, value));
                        }
                }
                const source = audioContext.createBufferSource();
                source.buffer = buffer;
                source.connect(audioContext.destination);
                const startAt = Math.max(audioContext.currentTime + 0.05, playbackQueueTime);
                source.start(startAt);
                playbackQueueTime = startAt + buffer.duration;
        }

        async function handleChunk(chunk: AudioStreamChunk) {
                if (!(await ensureAudioContext(chunk.format.sampleRate))) {
                        return;
                }
                const pcm = decodePcm(chunk.data);
                if (!pcm) {
                        playbackError = 'Received malformed audio data.';
                        return;
                }
                playbackError = null;
                if (session) {
                        session = {
                                ...session,
                                lastSequence: chunk.sequence,
                                lastUpdatedAt: chunk.timestamp
                        } satisfies AudioSessionState;
                }
                schedulePlayback(chunk.format, pcm);
        }

        onMount(async () => {
                if (!isBrowser) {
                        return;
                }
                await loadInventory();
                await loadSessionState();
        });

        onDestroy(() => {
                void stopListening(true);
        });
</script>

<div class="space-y-6">
        <div class="grid gap-6 lg:grid-cols-[2fr,1fr]">
                <Card>
                        <CardHeader class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                                <div>
                                        <CardTitle>Agent audio inventory</CardTitle>
                                        <CardDescription>
                                                Discover capture and playback devices reported by the client. Refresh to request a new hardware scan.
                                        </CardDescription>
                                </div>
                                <div class="flex items-center gap-2">
                                        {#if pendingInventory}
                                                <Badge variant="secondary">Pending update</Badge>
                                        {/if}
                                        <Button onclick={refreshInventory} disabled={refreshingInventory}>Refresh</Button>
                                </div>
                        </CardHeader>
                        <CardContent class="space-y-4">
                                {#if inventoryError}
                                        <p class="text-sm text-destructive">{inventoryError}</p>
                                {/if}
                                {#if !inventory}
                                        <p class="text-sm text-muted-foreground">
                                                No inventory is currently available. Request a refresh to collect the agent's audio hardware list.
                                        </p>
                                {:else}
                                        <div class="space-y-3">
                                                <div>
                                                        <h3 class="text-sm font-medium text-foreground/80">Input devices</h3>
                                                        {#if inventory.inputs.length === 0}
                                                                <p class="text-sm text-muted-foreground">No audio inputs were reported.</p>
                                                        {:else}
                                                                <div class="mt-2 space-y-2">
                                                                        {#each inventory.inputs as device (device.id)}
                                                                                <div class="flex flex-col gap-2 rounded-lg border border-border/60 p-3 sm:flex-row sm:items-center sm:justify-between">
                                                                                        <div class="space-y-1">
                                                                                                <div class="flex items-center gap-2">
                                                                                                        <span class="font-medium">{device.label}</span>
                                                                                                        {#if device.systemDefault}
                                                                                                                <Badge variant="outline">Default</Badge>
                                                                                                        {/if}
                                                                                                </div>
                                                                                                <p class="text-xs text-muted-foreground">{device.id}</p>
                                                                                        </div>
                                                                                        <div class="flex items-center gap-2">
                                                                                                <Button
                                                                                                        size="sm"
                                                                                                        variant="outline"
                                                                                                        disabled={refreshingInventory}
                                                                                                        onclick={() => startListening(device)}
                                                                                                >
                                                                                                        {listening && session?.deviceId === device.id ? 'Listening…' : 'Listen'}
                                                                                                </Button>
                                                                                        </div>
                                                                                </div>
                                                                        {/each}
                                                                </div>
                                                        {/if}
                                                </div>
                                                <div>
                                                        <h3 class="text-sm font-medium text-foreground/80">Output devices</h3>
                                                        {#if inventory.outputs.length === 0}
                                                                <p class="text-sm text-muted-foreground">No audio outputs were reported.</p>
                                                        {:else}
                                                                <p class="text-sm text-muted-foreground">
                                                                        Output capture is not yet available. These devices are listed for reference only.
                                                                </p>
                                                        {/if}
                                                </div>
                                        </div>
                                {/if}
                        </CardContent>
                </Card>

                <Card>
                        <CardHeader>
                                <CardTitle>Audio bridge session</CardTitle>
                                <CardDescription>
                                        Establish or release a live audio stream from the agent back to the controller.
                                </CardDescription>
                        </CardHeader>
                        <CardContent class="space-y-3">
                                {#if sessionError}
                                        <p class="text-sm text-destructive">{sessionError}</p>
                                {/if}
                                {#if playbackError}
                                        <p class="text-sm text-warning-foreground">{playbackError}</p>
                                {/if}
                                {#if session}
                                        <div class="space-y-2 text-sm">
                                                <div class="flex items-center gap-2">
                                                        <span class="font-medium text-foreground">{session.deviceLabel ?? 'Unknown device'}</span>
                                                        {#if listening && session.active}
                                                                <Badge>Streaming</Badge>
                                                        {:else if session.active}
                                                                <Badge variant="secondary">Pending</Badge>
                                                        {:else}
                                                                <Badge variant="outline">Stopped</Badge>
                                                        {/if}
                                                </div>
                                                <p class="text-muted-foreground">Session ID: {session.sessionId}</p>
                                                <p class="text-muted-foreground">Direction: {session.direction}</p>
                                                <p class="text-muted-foreground">
                                                        Format: {session.format.sampleRate} Hz · {session.format.channels} {session.format.channels === 1 ? 'channel' : 'channels'}
                                                </p>
                                                {#if session.lastUpdatedAt}
                                                        <p class="text-muted-foreground">Last update: {new Date(session.lastUpdatedAt).toLocaleString()}</p>
                                                {/if}
                                        </div>
                                {:else}
                                        <p class="text-sm text-muted-foreground">No active audio session.</p>
                                {/if}
                        </CardContent>
                        <CardFooter class="flex flex-col gap-2 sm:flex-row sm:justify-between">
                                <Button variant="outline" onclick={() => stopListening(false)} disabled={!session?.active && !listening}>
                                        Stop session
                                </Button>
                                {#if session}
                                        <Button variant="ghost" disabled>{session.direction === 'input' ? 'Input monitoring' : 'Output monitoring'}</Button>
                                {/if}
                        </CardFooter>
                </Card>
        </div>
</div>
