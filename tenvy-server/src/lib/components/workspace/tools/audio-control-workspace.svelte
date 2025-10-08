<script lang="ts">
        import { onDestroy, onMount } from 'svelte';
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
        import * as Dialog from '$lib/components/ui/dialog/index.js';
        import { Badge } from '$lib/components/ui/badge/index.js';
        import ClientWorkspaceHero from '$lib/components/workspace/workspace-hero.svelte';
        import ActionLog from '$lib/components/workspace/action-log.svelte';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        type AudioDirection = 'input' | 'output';

        type AudioDevice = {
                id: string;
                deviceId: string;
                label: string;
                kind: AudioDirection;
                groupId: string;
                systemDefault: boolean;
                communicationsDefault: boolean;
                lastSeen: string;
        };

        type DeviceControlState = {
                volume: number;
                muted: boolean;
                balance: number;
        };

        type DeviceDiagnostics = {
                sampleRate: number | null;
                channelCount: number | null;
                latency: number | null;
                echoCancellation: boolean | null;
                noiseSuppression: boolean | null;
                autoGainControl: boolean | null;
        };

        type PlaybackState = 'idle' | 'playing' | 'paused';

        const TEST_TONE_URL = '/audio-test-tone.wav';
        const DEFAULT_INPUT_VOLUME = 80;
        const DEFAULT_OUTPUT_VOLUME = 70;

        const isBrowser = typeof window !== 'undefined' && typeof navigator !== 'undefined';

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

        let mediaSupported = $state(false);
        let permissionState = $state<'unknown' | 'granted' | 'denied'>('unknown');
        let enumeratingDevices = $state(false);
        let enumerateError = $state<string | null>(null);
        let inputs = $state<AudioDevice[]>([]);
        let outputs = $state<AudioDevice[]>([]);
        let selectedInputId = $state<string | null>(null);
        let selectedOutputId = $state<string | null>(null);
        let deviceControls = $state<Record<string, DeviceControlState>>({});
        let deviceDiagnostics = $state<Record<string, DeviceDiagnostics>>({});
        let detailDialogOpen = $state(false);
        let detailDevice = $state<AudioDevice | null>(null);
        let detailLoading = $state(false);
        let detailError = $state<string | null>(null);

        let playbackState = $state<PlaybackState>('idle');
        let playbackPosition = $state(0);
        let playbackDuration = $state(0);
        let playbackError = $state<string | null>(null);

        let playbackTimer: ReturnType<typeof setInterval> | null = null;
        let audioElement: HTMLAudioElement | null = null;
        let audioContext: AudioContext | null = null;
        let mediaSource: MediaElementAudioSourceNode | null = null;
        let gainNode: GainNode | null = null;
        let stereoPanner: StereoPannerNode | null = null;
        let audioGraphInitialised = false;
        let deviceChangeListener: (() => void) | null = null;

        function describePlan(): string {
                return `${pipeline} · ${codec.toUpperCase()} · ${sampleRate / 1000}kHz · ${bitrate}kbps · PTT ${
                        pushToTalk ? 'on' : 'off'
                } · NS ${noiseSuppression ? 'on' : 'off'}`;
        }

        function queueCapture(status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Audio capture pipeline updated', describePlan(), status)
                );
        }

        function logAction(
                action: string,
                detail: string,
                status: WorkspaceLogEntry['status'] = 'complete'
        ) {
                log = appendWorkspaceLog(log, createWorkspaceLogEntry(action, detail, status));
        }

        function clamp(value: number, min: number, max: number): number {
                if (!Number.isFinite(value)) {
                        return min;
                }
                if (value < min) {
                        return min;
                }
                if (value > max) {
                        return max;
                }
                return value;
        }

        function createDefaultControl(kind: AudioDirection): DeviceControlState {
                return {
                        volume: kind === 'input' ? DEFAULT_INPUT_VOLUME : DEFAULT_OUTPUT_VOLUME,
                        muted: false,
                        balance: 0
                } satisfies DeviceControlState;
        }

        function normaliseDeviceId(device: MediaDeviceInfo): string {
                if (!device.deviceId || device.deviceId.length === 0) {
                        const random = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
                                ? crypto.randomUUID()
                                : Math.random().toString(36).slice(2, 10);
                        return `${device.kind}-${random}`;
                }
                if (device.deviceId === 'default') {
                        return `${device.kind}-default`;
                }
                if (device.deviceId === 'communications') {
                        return `${device.kind}-communications`;
                }
                return device.deviceId;
        }

        function selectedInputLabel(): string {
                if (!selectedInputId) {
                        return inputs.length > 0 ? 'Auto select' : 'None';
                }
                const match = inputs.find((device) => device.id === selectedInputId);
                return match?.label ?? 'None';
        }

        function selectedOutputLabel(): string {
                if (!selectedOutputId) {
                        return outputs.length > 0 ? 'Auto select' : 'None';
                }
                const match = outputs.find((device) => device.id === selectedOutputId);
                return match?.label ?? 'None';
        }

        function permissionLabel(): string {
                switch (permissionState) {
                        case 'granted':
                                return 'Granted';
                        case 'denied':
                                return 'Denied';
                        default:
                                return 'Unknown';
                }
        }

        function deviceStatus(device: AudioDevice): string {
                const control = deviceControls[device.id];
                if (control?.muted) {
                        return 'Muted';
                }
                if (
                        (device.kind === 'input' && selectedInputId === device.id) ||
                        (device.kind === 'output' && selectedOutputId === device.id)
                ) {
                        return 'Active';
                }
                return 'Available';
        }

        function formatTimestamp(value: string): string {
                const date = new Date(value);
                if (Number.isNaN(date.getTime())) {
                        return value;
                }
                return date.toLocaleString();
        }

        function formatBalance(value: number): string {
                const percent = Math.round(Math.abs(value) * 100);
                if (percent === 0) {
                        return 'Centered';
                }
                return value < 0 ? `Left ${percent}%` : `Right ${percent}%`;
        }

        function playbackProgressPercent(): number {
                if (!Number.isFinite(playbackDuration) || playbackDuration <= 0) {
                        return 0;
                }
                const percent = (playbackPosition / playbackDuration) * 100;
                return clamp(Number.isFinite(percent) ? percent : 0, 0, 100);
        }

        async function refreshPermissionState() {
                if (!isBrowser || !('permissions' in navigator)) {
                        return;
                }
                try {
                        const status = await (navigator.permissions as unknown as {
                                query: (descriptor: { name: 'microphone' }) => Promise<PermissionStatus>;
                        }).query({ name: 'microphone' });
                        if (status.state === 'granted') {
                                permissionState = 'granted';
                        } else if (status.state === 'denied') {
                                permissionState = 'denied';
                        } else {
                                permissionState = 'unknown';
                        }
                } catch {
                        // Permission API is optional; ignore failures.
                }
        }

        async function enumerateDevices(requestAccess = false) {
                if (!mediaSupported || !navigator.mediaDevices?.enumerateDevices) {
                        enumerateError = 'Media devices API is not available in this environment.';
                        return;
                }
                enumeratingDevices = true;
                enumerateError = null;
                try {
                        if (requestAccess && navigator.mediaDevices.getUserMedia) {
                                try {
                                        const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
                                        stream.getTracks().forEach((track) => track.stop());
                                        permissionState = 'granted';
                                } catch (error) {
                                        permissionState = 'denied';
                                        throw error;
                                }
                        } else if (permissionState === 'unknown') {
                                await refreshPermissionState();
                        }

                        const list = await navigator.mediaDevices.enumerateDevices();
                        const nextControls: Record<string, DeviceControlState> = { ...deviceControls };
                        const nextInputs: AudioDevice[] = [];
                        const nextOutputs: AudioDevice[] = [];
                        const seen = new Set<string>();
                        let inputFallback = 1;
                        let outputFallback = 1;

                        for (const device of list) {
                                if (device.kind !== 'audioinput' && device.kind !== 'audiooutput') {
                                        continue;
                                }
                                const kind: AudioDirection = device.kind === 'audioinput' ? 'input' : 'output';
                                const id = normaliseDeviceId(device);
                                if (seen.has(id)) {
                                        continue;
                                }
                                seen.add(id);
                                const label =
                                        device.label && device.label.trim().length > 0
                                                ? device.label
                                                : kind === 'input'
                                                        ? `Microphone ${inputFallback++}`
                                                        : `Output ${outputFallback++}`;
                                const entry: AudioDevice = {
                                        id,
                                        deviceId: device.deviceId,
                                        label,
                                        kind,
                                        groupId: device.groupId ?? '',
                                        systemDefault: device.deviceId === 'default',
                                        communicationsDefault: device.deviceId === 'communications',
                                        lastSeen: new Date().toISOString()
                                };
                                if (!nextControls[id]) {
                                        nextControls[id] = createDefaultControl(kind);
                                }
                                if (kind === 'input') {
                                        nextInputs.push(entry);
                                } else {
                                        nextOutputs.push(entry);
                                }
                        }

                        nextInputs.sort((a, b) => a.label.localeCompare(b.label));
                        nextOutputs.sort((a, b) => a.label.localeCompare(b.label));

                        const validIds = new Set([...nextInputs, ...nextOutputs].map((device) => device.id));
                        for (const id of Object.keys(nextControls)) {
                                if (!validIds.has(id)) {
                                        delete nextControls[id];
                                }
                        }

                        inputs = nextInputs;
                        outputs = nextOutputs;
                        deviceControls = nextControls;

                        syncSelection();
                        syncDetailDevice();
                        updateAudioGraphControls();
                } catch (error) {
                        if (error instanceof DOMException && error.name === 'NotAllowedError') {
                                enumerateError =
                                        'Microphone access was denied. Device names may be limited until permission is granted.';
                        } else if (error instanceof DOMException && error.name === 'NotFoundError') {
                                enumerateError = 'No audio hardware is currently available.';
                        } else {
                                enumerateError = (error as Error).message ?? 'Failed to enumerate audio devices.';
                        }
                } finally {
                        enumeratingDevices = false;
                }
        }

        function syncSelection() {
                if (inputs.length === 0) {
                        selectedInputId = null;
                } else if (!selectedInputId || !inputs.some((device) => device.id === selectedInputId)) {
                        const preferred =
                                inputs.find((device) => device.systemDefault) ??
                                inputs.find((device) => device.communicationsDefault) ??
                                inputs[0];
                        selectedInputId = preferred?.id ?? null;
                }

                let outputChanged = false;
                if (outputs.length === 0) {
                        outputChanged = selectedOutputId !== null;
                        selectedOutputId = null;
                } else {
                        const preferred =
                                outputs.find((device) => device.systemDefault) ??
                                outputs.find((device) => device.communicationsDefault) ??
                                outputs[0];
                        const nextOutputId = preferred?.id ?? null;
                        outputChanged = selectedOutputId !== nextOutputId;
                        selectedOutputId = nextOutputId;
                }

                if (outputChanged && selectedOutputId) {
                        void applyOutputSink(selectedOutputId);
                }
        }

        function syncDetailDevice() {
                if (!detailDevice) {
                        return;
                }
                const next = [...inputs, ...outputs].find((device) => device.id === detailDevice?.id) ?? null;
                detailDevice = next;
        }

        function ensureAudioGraph(): boolean {
                if (!isBrowser || typeof AudioContext === 'undefined') {
                        playbackError = 'Audio playback is not supported in this environment.';
                        return false;
                }
                if (!audioContext) {
                        audioContext = new AudioContext();
                }
                if (!audioElement) {
                        audioElement = new Audio(TEST_TONE_URL);
                        audioElement.preload = 'auto';
                        audioElement.crossOrigin = 'anonymous';
                        audioElement.addEventListener('ended', handlePlaybackEnded);
                }
                if (!mediaSource && audioContext && audioElement) {
                        mediaSource = audioContext.createMediaElementSource(audioElement);
                }
                if (!gainNode && audioContext) {
                        gainNode = audioContext.createGain();
                }
                if (!stereoPanner && audioContext) {
                        stereoPanner = audioContext.createStereoPanner();
                }
                if (mediaSource && gainNode && stereoPanner && audioContext && !audioGraphInitialised) {
                        mediaSource.connect(gainNode);
                        gainNode.connect(stereoPanner);
                        stereoPanner.connect(audioContext.destination);
                        audioGraphInitialised = true;
                }
                updateAudioGraphControls();
                return true;
        }

        function cleanupAudioGraph() {
                stopPlaybackTimer();
                playbackState = 'idle';
                playbackPosition = 0;
                playbackDuration = 0;
                if (audioElement) {
                        audioElement.pause();
                        audioElement.removeEventListener('ended', handlePlaybackEnded);
                        audioElement.src = '';
                        audioElement.load();
                        audioElement = null;
                }
                if (mediaSource) {
                        try {
                                mediaSource.disconnect();
                        } catch {
                                // ignore
                        }
                        mediaSource = null;
                }
                if (gainNode) {
                        try {
                                gainNode.disconnect();
                        } catch {
                                // ignore
                        }
                        gainNode = null;
                }
                if (stereoPanner) {
                        try {
                                stereoPanner.disconnect();
                        } catch {
                                // ignore
                        }
                        stereoPanner = null;
                }
                if (audioContext) {
                        audioContext.close().catch(() => {});
                        audioContext = null;
                }
                audioGraphInitialised = false;
        }

        function updateAudioGraphControls() {
                if (!audioElement) {
                        return;
                }
                const deviceId = selectedOutputId;
                if (!deviceId) {
                        return;
                }
                const control = deviceControls[deviceId];
                if (!control) {
                        return;
                }
                const volume = control.muted ? 0 : clamp(control.volume / 100, 0, 1);
                audioElement.muted = control.muted;
                audioElement.volume = volume;
                if (gainNode) {
                        gainNode.gain.value = volume;
                }
                if (stereoPanner) {
                        stereoPanner.pan.value = clamp(control.balance, -1, 1);
                }
        }

        async function applyOutputSink(deviceId: string) {
                if (!audioElement) {
                        return;
                }
                const device = outputs.find((item) => item.id === deviceId);
                if (!device) {
                        return;
                }
                const element = audioElement as HTMLAudioElement & { setSinkId?: (sinkId: string) => Promise<void> };
                if (typeof element.setSinkId === 'function') {
                        try {
                                await element.setSinkId(device.deviceId);
                        } catch (error) {
                                playbackError =
                                        (error as Error).message ?? 'Failed to route audio to the selected output device.';
                        }
                }
        }

        function updateVolume(device: AudioDevice, rawValue: number, commit: boolean) {
                const normalized = clamp(Math.round(rawValue), 0, 100);
                const existing = deviceControls[device.id] ?? createDefaultControl(device.kind);
                deviceControls = {
                        ...deviceControls,
                        [device.id]: { ...existing, volume: normalized }
                };
                if (device.kind === 'output' && device.id === selectedOutputId) {
                        updateAudioGraphControls();
                }
                if (commit) {
                        const action = device.kind === 'input' ? 'Input gain adjusted' : 'Output volume adjusted';
                        logAction(action, `${device.label} · ${normalized}%`);
                }
        }

        function toggleMute(device: AudioDevice, muted: boolean) {
                const existing = deviceControls[device.id] ?? createDefaultControl(device.kind);
                if (existing.muted === muted) {
                        return;
                }
                deviceControls = {
                        ...deviceControls,
                        [device.id]: { ...existing, muted }
                };
                if (device.kind === 'output' && device.id === selectedOutputId) {
                        updateAudioGraphControls();
                }
                logAction(
                        `${device.kind === 'input' ? 'Input' : 'Output'} ${muted ? 'muted' : 'unmuted'}`,
                        device.label
                );
        }

        function updateBalance(device: AudioDevice, rawValue: number, commit: boolean) {
                const normalized = clamp(rawValue, -100, 100) / 100;
                const existing = deviceControls[device.id] ?? createDefaultControl(device.kind);
                deviceControls = {
                        ...deviceControls,
                        [device.id]: { ...existing, balance: normalized }
                };
                if (device.kind === 'output' && device.id === selectedOutputId) {
                        updateAudioGraphControls();
                }
                if (commit) {
                        logAction('Output balance adjusted', `${device.label} · ${formatBalance(normalized)}`);
                }
        }

        function setDefaultDevice(device: AudioDevice) {
                if (device.kind === 'input') {
                        if (selectedInputId === device.id) {
                                return;
                        }
                        selectedInputId = device.id;
                        logAction('Default input updated', device.label);
                        return;
                }
                if (selectedOutputId === device.id) {
                        return;
                }
                selectedOutputId = device.id;
                void applyOutputSink(device.id);
                updateAudioGraphControls();
                logAction('Default output updated', device.label);
        }

        async function playTestSound() {
                playbackError = null;
                if (!ensureAudioGraph() || !audioElement) {
                        return;
                }
                try {
                        if (audioContext?.state === 'suspended') {
                                await audioContext.resume();
                        }
                        audioElement.currentTime = 0;
                        await audioElement.play();
                        playbackState = 'playing';
                        playbackDuration = Number.isFinite(audioElement.duration) ? audioElement.duration : 0;
                        startPlaybackTimer();
                        logAction('Test tone started', selectedOutputLabel());
                } catch (error) {
                        playbackError = (error as Error).message ?? 'Unable to start playback.';
                }
        }

        function pauseTestSound() {
                if (!audioElement || playbackState !== 'playing') {
                        return;
                }
                audioElement.pause();
                playbackState = 'paused';
                stopPlaybackTimer(false);
                logAction('Test tone paused', selectedOutputLabel());
        }

        function stopTestSound() {
                if (!audioElement || playbackState === 'idle') {
                        return;
                }
                audioElement.pause();
                audioElement.currentTime = 0;
                playbackState = 'idle';
                stopPlaybackTimer();
                logAction('Test tone stopped', selectedOutputLabel());
        }

        function startPlaybackTimer() {
                stopPlaybackTimer(false);
                playbackTimer = setInterval(() => {
                        if (!audioElement) {
                                return;
                        }
                        playbackPosition = audioElement.currentTime;
                        if (Number.isFinite(audioElement.duration)) {
                                playbackDuration = audioElement.duration;
                        }
                        if (audioElement.ended) {
                                playbackState = 'idle';
                                stopPlaybackTimer();
                        }
                }, 200);
        }

        function stopPlaybackTimer(reset = true) {
                if (playbackTimer) {
                        clearInterval(playbackTimer);
                        playbackTimer = null;
                }
                if (reset) {
                        playbackPosition = 0;
                }
        }

        function handlePlaybackEnded() {
                playbackState = 'idle';
                stopPlaybackTimer();
                logAction('Test tone completed', selectedOutputLabel());
        }

        async function handleRequestAccess() {
                await enumerateDevices(true);
        }

        async function handleRefresh() {
                await enumerateDevices(false);
        }

        async function inspectInputDevice(device: AudioDevice): Promise<DeviceDiagnostics | null> {
                if (!navigator.mediaDevices?.getUserMedia) {
                        return null;
                }
                const constraints: MediaStreamConstraints = {
                        audio:
                                device.deviceId === 'default' || device.deviceId === ''
                                        ? true
                                        : { deviceId: { exact: device.deviceId } },
                        video: false
                };
                const stream = await navigator.mediaDevices.getUserMedia(constraints);
                permissionState = 'granted';
                try {
                        const [track] = stream.getAudioTracks();
                        const settings = (track?.getSettings?.() ?? {}) as MediaTrackSettings & {
                                latency?: number;
                        };
                        return {
                                sampleRate:
                                        typeof settings.sampleRate === 'number' ? Math.round(settings.sampleRate) : null,
                                channelCount:
                                        typeof settings.channelCount === 'number' ? Math.round(settings.channelCount) : null,
                                latency: typeof settings.latency === 'number' ? settings.latency : null,
                                echoCancellation:
                                        typeof settings.echoCancellation === 'boolean'
                                                ? settings.echoCancellation
                                                : null,
                                noiseSuppression:
                                        typeof settings.noiseSuppression === 'boolean'
                                                ? settings.noiseSuppression
                                                : null,
                                autoGainControl:
                                        typeof settings.autoGainControl === 'boolean'
                                                ? settings.autoGainControl
                                                : null
                        } satisfies DeviceDiagnostics;
                } finally {
                        stream.getTracks().forEach((track) => {
                                try {
                                        track.stop();
                                } catch {
                                        // ignore
                                }
                        });
                }
        }

        async function inspectOutputDevice(): Promise<DeviceDiagnostics | null> {
                if (!ensureAudioGraph()) {
                        return null;
                }
                return {
                        sampleRate: audioContext?.sampleRate ?? null,
                        channelCount: audioContext?.destination?.maxChannelCount ?? null,
                        latency: audioContext?.baseLatency ?? null,
                        echoCancellation: null,
                        noiseSuppression: null,
                        autoGainControl: null
                } satisfies DeviceDiagnostics;
        }

        async function openDeviceDetails(device: AudioDevice) {
                detailDevice = device;
                detailDialogOpen = true;
                detailError = null;
                if (deviceDiagnostics[device.id]) {
                        detailLoading = false;
                        return;
                }
                detailLoading = true;
                try {
                        const diagnostics =
                                device.kind === 'input'
                                        ? await inspectInputDevice(device)
                                        : await inspectOutputDevice();
                        if (diagnostics) {
                                deviceDiagnostics = { ...deviceDiagnostics, [device.id]: diagnostics };
                        }
                        detailLoading = false;
                        logAction('Device diagnostics viewed', device.label);
                } catch (error) {
                        detailError = (error as Error).message ?? 'Unable to load device diagnostics.';
                        detailLoading = false;
                }
        }

        $effect(() => {
                if (!detailDialogOpen) {
                        detailDevice = null;
                        detailError = null;
                        detailLoading = false;
                }
        });

        onMount(() => {
                mediaSupported =
                        isBrowser && typeof navigator.mediaDevices?.enumerateDevices === 'function';
                if (!mediaSupported) {
                        enumerateError = 'Media devices API is not available in this environment.';
                        return;
                }
                void refreshPermissionState();
                void enumerateDevices(false);
                deviceChangeListener = () => {
                        void enumerateDevices(false);
                };
                navigator.mediaDevices.addEventListener('devicechange', deviceChangeListener);
        });

        onDestroy(() => {
                if (deviceChangeListener && navigator.mediaDevices?.removeEventListener) {
                        navigator.mediaDevices.removeEventListener('devicechange', deviceChangeListener);
                }
                cleanupAudioGraph();
        });
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
                        },
                        {
                                label: 'Input device',
                                value: selectedInputLabel(),
                                hint: `${inputs.length} detected`
                        },
                        {
                                label: 'Output device',
                                value: selectedOutputLabel(),
                                hint: `${outputs.length} detected`
                        }
                ]}
        >
                <p>
                        Prepare capture, monitoring, and playback flows while staying in control of every channel.
                        Enumerate microphones, loopback buses, and speakers, adjust live gain and balance, and stage
                        transport plans before wiring them into the agent runtime.
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
                        <Button type="button" variant="outline" onclick={() => queueCapture('draft')}>
                                Save draft
                        </Button>
                        <Button type="button" onclick={() => queueCapture('queued')}>
                                Queue capture
                        </Button>
                </CardFooter>
        </Card>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Device inventory</CardTitle>
                        <CardDescription>
                                Monitor microphones, loopback feeds, and playback channels. Adjust volume, mute state, and
                                defaults in real time.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="flex flex-wrap items-start justify-between gap-4">
                                <div class="space-y-2">
                                        <p class="text-sm font-medium text-foreground">Discovery status</p>
                                        <p class="text-xs text-muted-foreground">
                                                {#if !mediaSupported}
                                                        Media device APIs are unavailable in this environment.
                                                {:else if enumeratingDevices}
                                                        Enumerating devices…
                                                {:else if inputs.length + outputs.length === 0}
                                                        No audio hardware detected. Connect a device or request access to reveal inventory.
                                                {:else}
                                                        Tracking {inputs.length} input{inputs.length === 1 ? '' : 's'} and {outputs.length}
                                                        output{outputs.length === 1 ? '' : 's'}.
                                                {/if}
                                        </p>
                                        <div class="flex flex-wrap gap-2">
                                                <Badge variant="outline">Permission: {permissionLabel()}</Badge>
                                                <Badge variant="outline">Inputs: {inputs.length}</Badge>
                                                <Badge variant="outline">Outputs: {outputs.length}</Badge>
                                        </div>
                                </div>
                                <div class="flex flex-wrap gap-2">
                                        <Button
                                                type="button"
                                                variant="outline"
                                                onclick={handleRefresh}
                                                disabled={!mediaSupported || enumeratingDevices}
                                        >
                                                Refresh
                                        </Button>
                                        <Button
                                                type="button"
                                                onclick={handleRequestAccess}
                                                disabled={!mediaSupported || enumeratingDevices || permissionState === 'granted'}
                                        >
                                                Request access
                                        </Button>
                                </div>
                        </div>

                        {#if enumerateError}
                                <p class="text-sm text-destructive">{enumerateError}</p>
                        {/if}

                        <div class="grid gap-6 xl:grid-cols-2">
                                <section class="space-y-4">
                                        <div>
                                                <h3 class="text-sm font-semibold text-foreground">Input devices</h3>
                                                <p class="text-xs text-muted-foreground">
                                                        Configure capture gain, mute state, and metadata for microphones and loopback sources.
                                                </p>
                                        </div>
                                        {#if inputs.length === 0}
                                                <p class="rounded-lg border border-border/60 bg-muted/40 p-4 text-sm text-muted-foreground">
                                                        No audio inputs detected. Connect a microphone or grant permission to reveal available
                                                        devices.
                                                </p>
                                        {:else}
                                                <ul class="space-y-4">
                                                        {#each inputs as device (device.id)}
                                                                {@const control = deviceControls[device.id] ?? createDefaultControl('input')}
                                                                <li class="space-y-4 rounded-lg border border-border/60 bg-muted/30 p-4">
                                                                        <div class="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                                                                                <div class="space-y-1">
                                                                                        <p class="font-medium text-foreground">{device.label}</p>
                                                                                        <p class="text-xs text-muted-foreground">
                                                                                                Status: {deviceStatus(device)} · Last seen {formatTimestamp(device.lastSeen)}
                                                                                        </p>
                                                                                        {#if device.groupId}
                                                                                                <p class="text-xs text-muted-foreground">Group: {device.groupId}</p>
                                                                                        {/if}
                                                                                </div>
                                                                                <div class="flex flex-wrap justify-end gap-2">
                                                                                        {#if device.systemDefault}
                                                                                                <Badge variant="outline">System default</Badge>
                                                                                        {/if}
                                                                                        {#if device.communicationsDefault}
                                                                                                <Badge variant="outline">Communications</Badge>
                                                                                        {/if}
                                                                                        {#if selectedInputId === device.id}
                                                                                                <Badge variant="secondary">Workspace default</Badge>
                                                                                        {/if}
                                                                                </div>
                                                                        </div>
                                                                        <div class="space-y-3">
                                                                                <div class="space-y-1">
                                                                                        <div class="flex items-center justify-between text-xs uppercase tracking-wide text-muted-foreground">
                                                                                                <span>Gain</span>
                                                                                                <span>{control.muted ? 'Muted' : `${control.volume}%`}</span>
                                                                                        </div>
                                                                                        <input
                                                                                                type="range"
                                                                                                min="0"
                                                                                                max="100"
                                                                                                step="1"
                                                                                                value={control.volume}
                                                                                                oninput={(event) =>
                                                                                                        updateVolume(
                                                                                                                device,
                                                                                                                Number(
                                                                                                                        (event.currentTarget as HTMLInputElement).value
                                                                                                                ),
                                                                                                                false
                                                                                                        )
                                                                                                }
                                                                                                onchange={(event) =>
                                                                                                        updateVolume(
                                                                                                                device,
                                                                                                                Number(
                                                                                                                        (event.currentTarget as HTMLInputElement).value
                                                                                                                ),
                                                                                                                true
                                                                                                        )
                                                                                                }
                                                                                                class="h-1.5 w-full cursor-pointer appearance-none rounded-full bg-muted accent-primary"
                                                                                                aria-label={`Adjust gain for ${device.label}`}
                                                                                        />
                                                                                </div>
                                                                                <div class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-background/60 p-3">
                                                                                        <div>
                                                                                                <p class="text-xs font-medium uppercase tracking-wide text-muted-foreground">Mute</p>
                                                                                                <p class="text-xs text-muted-foreground">
                                                                                                        Toggle to suspend this capture source without removing it from the pipeline.
                                                                                                </p>
                                                                                        </div>
                                                                                        <Switch
                                                                                                checked={control.muted}
                                                                                                onclick={() => toggleMute(device, !control.muted)}
                                                                                                aria-label={`Toggle mute for ${device.label}`}
                                                                                        />
                                                                                </div>
                                                                        </div>
                                                                        <div class="flex flex-wrap gap-2">
                                                                                <Button
                                                                                        type="button"
                                                                                        variant="outline"
                                                                                        size="sm"
                                                                                        onclick={() => setDefaultDevice(device)}
                                                                                        disabled={selectedInputId === device.id}
                                                                                >
                                                                                        {selectedInputId === device.id ? 'Active input' : 'Use as default input'}
                                                                                </Button>
                                                                                <Button
                                                                                        type="button"
                                                                                        variant="ghost"
                                                                                        size="sm"
                                                                                        onclick={() => openDeviceDetails(device)}
                                                                                >
                                                                                        View details
                                                                                </Button>
                                                                        </div>
                                                                </li>
                                                        {/each}
                                                </ul>
                                        {/if}
                                </section>
                                <section class="space-y-4">
                                        <div>
                                                <h3 class="text-sm font-semibold text-foreground">Output devices</h3>
                                                <p class="text-xs text-muted-foreground">
                                                        Route playback, adjust mix balance, and choose the default speaker for operator diagnostics.
                                                </p>
                                        </div>
                                        {#if outputs.length === 0}
                                                <p class="rounded-lg border border-border/60 bg-muted/40 p-4 text-sm text-muted-foreground">
                                                        No audio outputs detected. Attach a playback device or request permission to view existing
                                                        routes.
                                                </p>
                                        {:else}
                                                <ul class="space-y-4">
                                                        {#each outputs as device (device.id)}
                                                                {@const control = deviceControls[device.id] ?? createDefaultControl('output')}
                                                                <li class="space-y-4 rounded-lg border border-border/60 bg-muted/30 p-4">
                                                                        <div class="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                                                                                <div class="space-y-1">
                                                                                        <p class="font-medium text-foreground">{device.label}</p>
                                                                                        <p class="text-xs text-muted-foreground">
                                                                                                Status: {deviceStatus(device)} · Last seen {formatTimestamp(device.lastSeen)}
                                                                                        </p>
                                                                                        {#if device.groupId}
                                                                                                <p class="text-xs text-muted-foreground">Group: {device.groupId}</p>
                                                                                        {/if}
                                                                                </div>
                                                                                <div class="flex flex-wrap justify-end gap-2">
                                                                                        {#if device.systemDefault}
                                                                                                <Badge variant="outline">System default</Badge>
                                                                                        {/if}
                                                                                        {#if device.communicationsDefault}
                                                                                                <Badge variant="outline">Communications</Badge>
                                                                                        {/if}
                                                                                        {#if selectedOutputId === device.id}
                                                                                                <Badge variant="secondary">Workspace default</Badge>
                                                                                        {/if}
                                                                                </div>
                                                                        </div>
                                                                        <div class="space-y-3">
                                                                                <div class="space-y-1">
                                                                                        <div class="flex items-center justify-between text-xs uppercase tracking-wide text-muted-foreground">
                                                                                                <span>Volume</span>
                                                                                                <span>{control.muted ? 'Muted' : `${control.volume}%`}</span>
                                                                                        </div>
                                                                                        <input
                                                                                                type="range"
                                                                                                min="0"
                                                                                                max="100"
                                                                                                step="1"
                                                                                                value={control.volume}
                                                                                                oninput={(event) =>
                                                                                                        updateVolume(
                                                                                                                device,
                                                                                                                Number(
                                                                                                                        (event.currentTarget as HTMLInputElement).value
                                                                                                                ),
                                                                                                                false
                                                                                                        )
                                                                                                }
                                                                                                onchange={(event) =>
                                                                                                        updateVolume(
                                                                                                                device,
                                                                                                                Number(
                                                                                                                        (event.currentTarget as HTMLInputElement).value
                                                                                                                ),
                                                                                                                true
                                                                                                        )
                                                                                                }
                                                                                                class="h-1.5 w-full cursor-pointer appearance-none rounded-full bg-muted accent-primary"
                                                                                                aria-label={`Adjust volume for ${device.label}`}
                                                                                        />
                                                                                </div>
                                                                                <div class="space-y-1">
                                                                                        <div class="flex items-center justify-between text-xs uppercase tracking-wide text-muted-foreground">
                                                                                                <span>Balance</span>
                                                                                                <span>{formatBalance(control.balance)}</span>
                                                                                        </div>
                                                                                        <input
                                                                                                type="range"
                                                                                                min="-100"
                                                                                                max="100"
                                                                                                step="1"
                                                                                                value={Math.round(control.balance * 100)}
                                                                                                oninput={(event) =>
                                                                                                        updateBalance(
                                                                                                                device,
                                                                                                                Number(
                                                                                                                        (event.currentTarget as HTMLInputElement).value
                                                                                                                ),
                                                                                                                false
                                                                                                        )
                                                                                                }
                                                                                                onchange={(event) =>
                                                                                                        updateBalance(
                                                                                                                device,
                                                                                                                Number(
                                                                                                                        (event.currentTarget as HTMLInputElement).value
                                                                                                                ),
                                                                                                                true
                                                                                                        )
                                                                                                }
                                                                                                class="h-1.5 w-full cursor-pointer appearance-none rounded-full bg-muted accent-primary"
                                                                                                aria-label={`Adjust balance for ${device.label}`}
                                                                                        />
                                                                                </div>
                                                                                <div class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-background/60 p-3">
                                                                                        <div>
                                                                                                <p class="text-xs font-medium uppercase tracking-wide text-muted-foreground">Mute</p>
                                                                                                <p class="text-xs text-muted-foreground">
                                                                                                        Temporarily silence playback while keeping the channel configured.
                                                                                                </p>
                                                                                        </div>
                                                                                        <Switch
                                                                                                checked={control.muted}
                                                                                                onclick={() => toggleMute(device, !control.muted)}
                                                                                                aria-label={`Toggle mute for ${device.label}`}
                                                                                        />
                                                                                </div>
                                                                        </div>
                                                                        <div class="flex flex-wrap gap-2">
                                                                                <Button
                                                                                        type="button"
                                                                                        variant="outline"
                                                                                        size="sm"
                                                                                        onclick={() => setDefaultDevice(device)}
                                                                                        disabled={selectedOutputId === device.id}
                                                                                >
                                                                                        {selectedOutputId === device.id ? 'Active output' : 'Use as default output'}
                                                                                </Button>
                                                                                <Button
                                                                                        type="button"
                                                                                        variant="ghost"
                                                                                        size="sm"
                                                                                        onclick={() => openDeviceDetails(device)}
                                                                                >
                                                                                        View details
                                                                                </Button>
                                                                        </div>
                                                                </li>
                                                        {/each}
                                                </ul>
                                        {/if}
                                </section>
                        </div>
                </CardContent>
        </Card>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Playback diagnostics</CardTitle>
                        <CardDescription>
                                Generate reference tones to verify speaker routing and balance adjustments without leaving the
                                workspace.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-4">
                        <div class="flex flex-wrap gap-2">
                                <Button type="button" onclick={playTestSound} disabled={playbackState === 'playing'}>
                                        Play
                                </Button>
                                <Button
                                        type="button"
                                        variant="outline"
                                        onclick={pauseTestSound}
                                        disabled={playbackState !== 'playing'}
                                >
                                        Pause
                                </Button>
                                <Button
                                        type="button"
                                        variant="outline"
                                        onclick={stopTestSound}
                                        disabled={playbackState === 'idle'}
                                >
                                        Stop
                                </Button>
                        </div>
                        <div class="space-y-1">
                                <div class="flex items-center justify-between text-xs uppercase tracking-wide text-muted-foreground">
                                        <span>Progress</span>
                                        <span>{playbackState === 'playing' || playbackState === 'paused' ? `${playbackPosition.toFixed(1)}s` : '0.0s'} / {playbackDuration.toFixed(1)}s</span>
                                </div>
                                <div class="h-2 rounded-full bg-muted">
                                        <div
                                                class="h-full rounded-full bg-primary"
                                                style={`width: ${Math.round(playbackProgressPercent())}%`}
                                        ></div>
                                </div>
                        </div>
                        <p class="text-xs text-muted-foreground">
                                Audio routes through {selectedOutputLabel()}. Use the controls above to confirm volume and
                                balance changes before going live.
                        </p>
                        {#if playbackError}
                                <p class="text-sm text-destructive">{playbackError}</p>
                        {/if}
                </CardContent>
        </Card>

        <Card class="border-dashed">
                <CardHeader>
                        <CardTitle class="text-base">Streaming checklist</CardTitle>
                        <CardDescription>Items to confirm before moving an audio bridge into production.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-2 text-sm text-muted-foreground">
                        <ul class="list-disc space-y-1 pl-5">
                                <li>Verify default input and output selections align with the operator's scenario.</li>
                                <li>Calibrate capture gain and playback volume to avoid clipping or low signal.</li>
                                <li>Exercise the test tone to confirm balance, routing, and mute automation.</li>
                        </ul>
                </CardContent>
        </Card>

        <ActionLog entries={log} />
</div>

<Dialog.Root bind:open={detailDialogOpen}>
        <Dialog.Content class="sm:max-w-lg">
                {#if detailDevice}
                        <Dialog.Header>
                                <Dialog.Title>{detailDevice.label}</Dialog.Title>
                                <Dialog.Description>
                                        {detailDevice.kind === 'input' ? 'Audio input device' : 'Audio output device'}
                                </Dialog.Description>
                        </Dialog.Header>
                        <div class="space-y-4 text-sm">
                                <div class="grid gap-1">
                                        <p>
                                                <span class="font-medium text-foreground">Status:</span> {deviceStatus(detailDevice)}
                                        </p>
                                        <p>
                                                <span class="font-medium text-foreground">Device ID:</span>
                                                <code class="break-all">{detailDevice.deviceId || 'Unavailable'}</code>
                                        </p>
                                        <p>
                                                <span class="font-medium text-foreground">Group ID:</span>
                                                {detailDevice.groupId || 'Unknown'}
                                        </p>
                                        <p>
                                                <span class="font-medium text-foreground">Last seen:</span>
                                                {formatTimestamp(detailDevice.lastSeen)}
                                        </p>
                                        <p>
                                                <span class="font-medium text-foreground">Muted:</span>
                                                {deviceControls[detailDevice.id]?.muted ? 'Yes' : 'No'}
                                        </p>
                                        <p>
                                                <span class="font-medium text-foreground">Volume:</span>
                                                {deviceControls[detailDevice.id]?.muted
                                                        ? 'Muted'
                                                        : `${deviceControls[detailDevice.id]?.volume ?? 0}%`}
                                        </p>
                                        {#if detailDevice.kind === 'output'}
                                                <p>
                                                        <span class="font-medium text-foreground">Balance:</span>
                                                        {formatBalance(deviceControls[detailDevice.id]?.balance ?? 0)}
                                                </p>
                                        {/if}
                                </div>
                                {#if detailLoading}
                                        <p class="text-xs text-muted-foreground">Gathering diagnostics…</p>
                                {:else if detailError}
                                        <p class="text-sm text-destructive">{detailError}</p>
                                {:else if deviceDiagnostics[detailDevice.id]}
                                        {@const diagnostics = deviceDiagnostics[detailDevice.id]}
                                        <div class="space-y-1">
                                                <p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                                                        Diagnostics
                                                </p>
                                                <ul class="space-y-1 text-sm text-foreground">
                                                        {#if diagnostics.sampleRate !== null}
                                                                <li>Sample rate: {diagnostics.sampleRate} Hz</li>
                                                        {/if}
                                                        {#if diagnostics.channelCount !== null}
                                                                <li>Channels: {diagnostics.channelCount}</li>
                                                        {/if}
                                                        {#if diagnostics.latency !== null}
                                                                <li>Reported latency: {diagnostics.latency.toFixed(3)} s</li>
                                                        {/if}
                                                        {#if diagnostics.echoCancellation !== null}
                                                                <li>Echo cancellation: {diagnostics.echoCancellation ? 'Enabled' : 'Disabled'}</li>
                                                        {/if}
                                                        {#if diagnostics.noiseSuppression !== null}
                                                                <li>Noise suppression: {diagnostics.noiseSuppression ? 'Enabled' : 'Disabled'}</li>
                                                        {/if}
                                                        {#if diagnostics.autoGainControl !== null}
                                                                <li>Auto gain control: {diagnostics.autoGainControl ? 'Enabled' : 'Disabled'}</li>
                                                        {/if}
                                                </ul>
                                        </div>
                                {:else}
                                        <p class="text-xs text-muted-foreground">
                                                No additional diagnostics are available for this device.
                                        </p>
                                {/if}
                        </div>
                        <Dialog.Footer>
                                <Dialog.Close>
                                        {#snippet child({ props })}
                                                <Button {...props} type="button" variant="outline">Close</Button>
                                        {/snippet}
                                </Dialog.Close>
                        </Dialog.Footer>
                {/if}
        </Dialog.Content>
</Dialog.Root>