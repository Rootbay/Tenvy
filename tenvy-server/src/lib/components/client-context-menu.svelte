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
	import type { Client } from '$lib/data/clients';

	export let client: Client;
</script>

<ContextMenuContent class="w-64">
	<ContextMenuGroup>
		<ContextMenuItem>System Info</ContextMenuItem>
		<ContextMenuItem>Notes</ContextMenuItem>
	</ContextMenuGroup>

	<ContextMenuSeparator />

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Control</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem>Hidden VNC</ContextMenuItem>
				<ContextMenuItem>Remote Desktop</ContextMenuItem>
				<ContextMenuItem>Webcam Control</ContextMenuItem>
				<ContextMenuItem>Audio Control</ContextMenuItem>
				<ContextMenuSub>
					<ContextMenuSubTrigger>Keylogger</ContextMenuSubTrigger>
					<ContextMenuSubContent class="w-48">
						<ContextMenuItem>Online</ContextMenuItem>
						<ContextMenuItem>Offline</ContextMenuItem>
						<ContextMenuItem>Advanced Online</ContextMenuItem>
					</ContextMenuSubContent>
				</ContextMenuSub>
				<ContextMenuItem>CMD</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Management</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem>File Manager</ContextMenuItem>
				<ContextMenuItem>Task Manager</ContextMenuItem>
				<ContextMenuItem>Registry Manager</ContextMenuItem>
				<ContextMenuItem>Startup Manager</ContextMenuItem>
				<ContextMenuItem>Clipboard Manager</ContextMenuItem>
				<ContextMenuItem>TCP Connections</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuSeparator />

	<ContextMenuGroup>
		<ContextMenuItem>Recovery</ContextMenuItem>
		<ContextMenuItem>Options</ContextMenuItem>
	</ContextMenuGroup>

	<ContextMenuSeparator />

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Miscellaneous</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem>Open URL</ContextMenuItem>
				<ContextMenuItem>Message Box</ContextMenuItem>
				<ContextMenuItem>Client Chat</ContextMenuItem>
				<ContextMenuItem>Report Window</ContextMenuItem>
				<ContextMenuItem>IP Geolocation</ContextMenuItem>
				<ContextMenuItem>Environment Variables</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>System Controls</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem>Reconnect</ContextMenuItem>
				<ContextMenuItem>Disconnect</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>

	<ContextMenuGroup>
		<ContextMenuSub>
			<ContextMenuSubTrigger>Power</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem>Shutdown</ContextMenuItem>
				<ContextMenuItem>Restart</ContextMenuItem>
				<ContextMenuItem>Sleep</ContextMenuItem>
				<ContextMenuItem>Logoff</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>
	</ContextMenuGroup>
</ContextMenuContent>
