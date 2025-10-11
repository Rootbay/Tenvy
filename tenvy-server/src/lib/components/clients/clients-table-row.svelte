<script lang="ts">
	import { ContextMenu as ContextMenuPrimitive } from 'bits-ui';
	import {
		ContextMenu,
		ContextMenuContent,
		ContextMenuItem,
		ContextMenuSeparator,
		ContextMenuSub,
		ContextMenuSubContent,
		ContextMenuSubTrigger,
		ContextMenuTrigger
	} from '$lib/components/ui/context-menu/index.js';
	import { TableCell, TableRow } from '$lib/components/ui/table/index.js';
	import OsLogo from '$lib/components/os-logo.svelte';
	import { cn } from '$lib/utils.js';
	import type { AgentSnapshot } from '../../../../../shared/types/agent';
	import type { SectionKey } from '$lib/client-sections';

	type TriggerChildProps = Parameters<NonNullable<ContextMenuPrimitive.TriggerProps['child']>>[0];

	export let agent: AgentSnapshot;
	export let openSection: (section: SectionKey, agent: AgentSnapshot) => void;
	export let copyAgentId: (agentId: string) => void;
	export let getAgentLocation: (agent: AgentSnapshot) => { label: string; flag: string };
	export let getAgentGroup: (agent: AgentSnapshot) => string;
	export let formatPing: (agent: AgentSnapshot) => string;
	export let formatDate: (value: string) => string;
</script>

{#snippet TriggerChild({ props }: TriggerChildProps)}
	{@const className = cn('cursor-context-menu', (props as { class?: string }).class)}
	<TableRow {...props} class={className} tabindex={0}>
		<TableCell>
			<div class="flex items-center gap-3">
				<span class="text-2xl" aria-hidden="true">{getAgentLocation(agent).flag}</span>
				<div class="flex flex-col">
					<span class="text-sm font-medium text-foreground">{getAgentLocation(agent).label}</span>
					{#if agent.metadata.hostname}
						<span class="text-xs text-muted-foreground">{agent.metadata.hostname}</span>
					{/if}
				</div>
			</div>
		</TableCell>
		<TableCell class="text-sm text-muted-foreground">
			{agent.metadata.publicIpAddress ?? agent.metadata.ipAddress ?? 'Unknown'}
		</TableCell>
		<TableCell class="text-sm text-muted-foreground">
			{agent.metadata.username}
		</TableCell>
		<TableCell class="text-sm text-muted-foreground">
			{getAgentGroup(agent)}
		</TableCell>
		<TableCell class="text-center">
			<OsLogo os={agent.metadata.os} />
		</TableCell>
		<TableCell class="text-sm text-muted-foreground">
			{formatPing(agent)}
		</TableCell>
		<TableCell class="text-sm text-muted-foreground">
			{agent.metadata.version ?? '—'}
		</TableCell>
		<TableCell class="text-sm text-muted-foreground">
			{formatDate(agent.connectedAt)}
		</TableCell>
	</TableRow>
{/snippet}

<ContextMenu>
	<ContextMenuTrigger child={TriggerChild} />
	<ContextMenuContent class="w-56">
		<ContextMenuItem onSelect={() => openSection('systemInfo', agent)}>System Info</ContextMenuItem>
		<ContextMenuItem onSelect={() => openSection('notes', agent)}>Notes</ContextMenuItem>

		<ContextMenuSeparator />

		<ContextMenuSub>
			<ContextMenuSubTrigger>Control</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem onSelect={() => openSection('hiddenVnc', agent)}>
					Hidden VNC
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('remoteDesktop', agent)}>
					Remote Desktop
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('webcamControl', agent)}>
					Webcam Control
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('audioControl', agent)}>
					Audio Control
				</ContextMenuItem>
				<ContextMenuSub>
					<ContextMenuSubTrigger>Keylogger</ContextMenuSubTrigger>
					<ContextMenuSubContent class="w-48">
						<ContextMenuItem onSelect={() => openSection('keyloggerOnline', agent)}>
							Online
						</ContextMenuItem>
						<ContextMenuItem onSelect={() => openSection('keyloggerOffline', agent)}>
							Offline
						</ContextMenuItem>
						<ContextMenuItem onSelect={() => openSection('keyloggerAdvanced', agent)}>
							Advanced Online
						</ContextMenuItem>
					</ContextMenuSubContent>
				</ContextMenuSub>
				<ContextMenuItem onSelect={() => openSection('cmd', agent)}>CMD</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>

		<ContextMenuSeparator />

		<ContextMenuSub>
			<ContextMenuSubTrigger>Management</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem onSelect={() => openSection('fileManager', agent)}>
					File Manager
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('taskManager', agent)}>
					Task Manager
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('registryManager', agent)}>
					Registry Manager
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('startupManager', agent)}>
					Startup Manager
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('clipboardManager', agent)}>
					Clipboard Manager
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('tcpConnections', agent)}>
					TCP Connections
				</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>

		<ContextMenuSeparator />

		<ContextMenuItem onSelect={() => openSection('recovery', agent)}>Recovery</ContextMenuItem>
		<ContextMenuItem onSelect={() => openSection('options', agent)}>Options</ContextMenuItem>

		<ContextMenuSeparator />

		<ContextMenuSub>
			<ContextMenuSubTrigger>Miscellaneous</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem onSelect={() => openSection('openUrl', agent)}>Open URL</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('messageBox', agent)}>
					Message Box
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('clientChat', agent)}>
					Client Chat
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('reportWindow', agent)}>
					Report Window
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('ipGeolocation', agent)}>
					IP Geolocation
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('environmentVariables', agent)}>
					Environment Variables
				</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>

		<ContextMenuSeparator />

		<ContextMenuSub>
			<ContextMenuSubTrigger>System Controls</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem onSelect={() => openSection('reconnect', agent)}>
					Reconnect
				</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('disconnect', agent)}>
					Disconnect
				</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>

		<ContextMenuSeparator />

		<ContextMenuSub>
			<ContextMenuSubTrigger>Power</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem onSelect={() => openSection('shutdown', agent)}>Shutdown</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('restart', agent)}>Restart</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('sleep', agent)}>Sleep</ContextMenuItem>
				<ContextMenuItem onSelect={() => openSection('logoff', agent)}>Logoff</ContextMenuItem>
			</ContextMenuSubContent>
		</ContextMenuSub>

		<ContextMenuSeparator />

		<ContextMenuItem onSelect={() => copyAgentId(agent.id)}>Copy agent ID</ContextMenuItem>
	</ContextMenuContent>
</ContextMenu>
