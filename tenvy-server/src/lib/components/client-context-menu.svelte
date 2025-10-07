<script lang="ts">
	import {
		ContextMenuContent,
		ContextMenuGroup,
		ContextMenuItem,
		ContextMenuSeparator,
		ContextMenuSub,
		ContextMenuSubContent,
		ContextMenuSubTrigger
	} from '$lib/components/ui/context-menu/index.js';
	import { goto } from '$app/navigation';
	import { browser } from '$app/environment';
	import type { Client } from '$lib/data/clients';
	import ClientToolDialog from '$lib/components/client-tool-dialog.svelte';
	import {
		buildClientToolUrl,
		getClientTool,
		isDialogTool,
		type ClientToolId,
		type DialogToolId
	} from '$lib/data/client-tools';

	const { client } = $props<{ client: Client }>();

        let dialogTool = $state<DialogToolId | null>(null);

	function openTool(toolId: ClientToolId) {
		const tool = getClientTool(toolId);
		const target = tool.target ?? '_blank';

		if (target === 'dialog') {
			dialogTool = isDialogTool(toolId) ? toolId : (toolId as DialogToolId);
			return;
		}

		dialogTool = null;

		const url = buildClientToolUrl(client.id, tool);

		if (!browser) return;

		if (target === '_self') {
			goto(url);
			return;
		}

		window.open(url, target, 'noopener,noreferrer');
	}

	function handleDialogClose() {
		dialogTool = null;
	}
</script>

<ContextMenuContent class="w-64">
	<ContextMenuGroup>
		<ContextMenuItem on:select={() => openTool('system-info')}>System Info</ContextMenuItem>
		<ContextMenuItem on:select={() => openTool('notes')}>Notes</ContextMenuItem>
	</ContextMenuGroup>

	<ContextMenuSeparator />

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Control</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem on:select={() => openTool('hidden-vnc')}>Hidden VNC</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('remote-desktop')}>
					Remote Desktop
				</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('webcam-control')}>
					Webcam Control
				</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('audio-control')}>Audio Control</ContextMenuItem>
				<ContextMenuSub>
					<ContextMenuSubTrigger>Keylogger</ContextMenuSubTrigger>
					<ContextMenuSubContent class="w-48">
						<ContextMenuItem on:select={() => openTool('keylogger-online')}>Online</ContextMenuItem>
						<ContextMenuItem on:select={() => openTool('keylogger-offline')}>
							Offline
						</ContextMenuItem>
						<ContextMenuItem on:select={() => openTool('keylogger-advanced-online')}>
							Advanced Online
						</ContextMenuItem>
					</ContextMenuSubContent>
				</ContextMenuSub>
				<ContextMenuItem on:select={() => openTool('cmd')}>CMD</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Management</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem on:select={() => openTool('file-manager')}>File Manager</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('task-manager')}>Task Manager</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('registry-manager')}>
					Registry Manager
				</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('startup-manager')}>
					Startup Manager
				</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('clipboard-manager')}>
					Clipboard Manager
				</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('tcp-connections')}>
					TCP Connections
				</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuSeparator />

	<ContextMenuGroup>
		<ContextMenuItem on:select={() => openTool('recovery')}>Recovery</ContextMenuItem>
		<ContextMenuItem on:select={() => openTool('options')}>Options</ContextMenuItem>
	</ContextMenuGroup>

	<ContextMenuSeparator />

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Miscellaneous</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem on:select={() => openTool('open-url')}>Open URL</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('message-box')}>Message Box</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('client-chat')}>Client Chat</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('report-window')}>Report Window</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('ip-geolocation')}>
					IP Geolocation
				</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('environment-variables')}>
					Environment Variables
				</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>System Controls</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem on:select={() => openTool('reconnect')}>Reconnect</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('disconnect')}>Disconnect</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Power</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem on:select={() => openTool('shutdown')}>Shutdown</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('restart')}>Restart</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('sleep')}>Sleep</ContextMenuItem>
				<ContextMenuItem on:select={() => openTool('logoff')}>Logoff</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>
</ContextMenuContent>

{#if dialogTool}
	{#key dialogTool}
		<ClientToolDialog {client} toolId={dialogTool} on:close={handleDialogClose} />
	{/key}
{/if}
