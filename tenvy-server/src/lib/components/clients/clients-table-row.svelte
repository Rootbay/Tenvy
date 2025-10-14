<script lang="ts">
	import { browser } from '$app/environment';
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
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { TableCell, TableRow } from '$lib/components/ui/table/index.js';
	import OsLogo from '$lib/components/os-logo.svelte';
	import { cn } from '$lib/utils.js';
import { countryCodeToFlag } from '$lib/utils/location';
import type { AgentSnapshot } from '../../../../../shared/types/agent';
import type { SectionKey } from '$lib/client-sections';

type TriggerChildProps = Parameters<NonNullable<ContextMenuPrimitive.TriggerProps['child']>>[0];

type ResolvedLocation = {
	label: string;
	flagEmoji: string;
	flagUrl?: string;
	isVpn: boolean;
};

let {
	agent,
	openSection,
	copyAgentId,
	getAgentLocation,
	getAgentGroup,
	formatPing,
	formatDate
} = $props<{
	agent: AgentSnapshot;
	openSection: (section: SectionKey, agent: AgentSnapshot) => void;
	copyAgentId: (agentId: string) => void;
	getAgentLocation: (agent: AgentSnapshot) => { label: string; flag: string };
	getAgentGroup: (agent: AgentSnapshot) => string;
	formatPing: (agent: AgentSnapshot) => string;
	formatDate: (value: string) => string;
}>();

const globalRegistry = globalThis as Record<string, unknown>;

if (!globalRegistry.__tenvyIpLocationCache) {
	globalRegistry.__tenvyIpLocationCache = new Map<string, ResolvedLocation>();
}

if (!globalRegistry.__tenvyIpLocationPromises) {
	globalRegistry.__tenvyIpLocationPromises = new Map<string, Promise<ResolvedLocation>>();
}

const ipLocationCache = globalRegistry.__tenvyIpLocationCache as Map<string, ResolvedLocation>;
const ipLocationPromises = globalRegistry.__tenvyIpLocationPromises as Map<
	string,
	Promise<ResolvedLocation>
>;

function toResolvedLocation(base: { label: string; flag: string }): ResolvedLocation {
	return {
		label: base.label,
		flagEmoji: base.flag,
		flagUrl: undefined,
		isVpn: false
	};
}

let locationDisplay = $state<ResolvedLocation>(toResolvedLocation(getAgentLocation(agent)));

$effect(() => {
	const baseLocation = toResolvedLocation(getAgentLocation(agent));
	locationDisplay = baseLocation;

	const ip = agent.metadata.publicIpAddress?.trim();
	if (!ip || isLikelyPrivateIp(ip)) {
		return;
	}

	const cached = ipLocationCache.get(ip);
	if (cached) {
		locationDisplay = { ...cached };
		return;
	}

	if (!browser) {
		return;
	}

	const existingPromise = ipLocationPromises.get(ip);
	if (existingPromise) {
		return attachLocationPromise(ip, existingPromise, baseLocation);
	}

	const lookupPromise = fetchIpLocation(ip, baseLocation);
	ipLocationPromises.set(ip, lookupPromise);
	return attachLocationPromise(ip, lookupPromise, baseLocation);
});

	function attachLocationPromise(
		ip: string,
		promise: Promise<ResolvedLocation>,
		baseLocation: ResolvedLocation
	): () => void {
		let disposed = false;

		promise
			.then((result) => {
				const resolved = { ...result };
				ipLocationCache.set(ip, resolved);
				if (!disposed && agent.metadata.publicIpAddress?.trim() === ip) {
					locationDisplay = resolved;
				}
			})
			.catch(() => {
				if (!disposed && agent.metadata.publicIpAddress?.trim() === ip) {
					locationDisplay = { ...baseLocation };
				}
			})
			.finally(() => {
				if (ipLocationPromises.get(ip) === promise) {
					ipLocationPromises.delete(ip);
				}
			});

		return () => {
			disposed = true;
		};
	}

	function isLikelyPrivateIp(ip: string): boolean {
		const normalized = ip.toLowerCase();
		if (
			normalized === '::1' ||
			normalized.startsWith('fe80:') ||
			normalized.startsWith('fc') ||
			normalized.startsWith('fd')
		) {
			return true;
		}
		const ipv4Candidate = normalized.startsWith('::ffff:') ? normalized.slice(7) : normalized;
		return (
			ipv4Candidate.startsWith('10.') ||
			ipv4Candidate.startsWith('192.168.') ||
			/^172\.(1[6-9]|2\d|3[0-1])\./.test(ipv4Candidate) ||
			ipv4Candidate.startsWith('127.')
		);
	}

	async function fetchIpLocation(ip: string, baseLocation: ResolvedLocation): Promise<ResolvedLocation> {
		const url = new URL(`http://ip-api.com/json/${encodeURIComponent(ip)}`);
		url.searchParams.set('fields', 'status,message,country,countryCode,proxy,query');

		const response = await fetch(url.toString(), {
			headers: { Accept: 'application/json' }
		});

		if (!response.ok) {
			throw new Error('Failed to resolve IP location');
		}

		const data = (await response.json()) as {
			status?: 'success' | 'fail';
			message?: string;
			country?: string;
			countryCode?: string;
			proxy?: boolean;
		};

		if (data.status !== 'success') {
			throw new Error(data.message ?? 'Lookup error');
		}

		const countryName = data.country?.trim() || baseLocation.label;
		const countryCode = data.countryCode?.trim();
		const flagEmoji =
			countryCode && countryCode.length > 0
				? countryCodeToFlag(countryCode)
				: baseLocation.flagEmoji;
		const flagUrl =
			countryCode && countryCode.length > 0
				? `https://flagcdn.com/${countryCode.toLowerCase()}.svg`
				: baseLocation.flagUrl;

		return {
			label: countryName,
			flagEmoji: flagEmoji || baseLocation.flagEmoji,
			flagUrl,
			isVpn: data.proxy === true
		};
	}
</script>

{#snippet TriggerChild({ props }: TriggerChildProps)}
	{@const className = cn('cursor-context-menu', (props as { class?: string }).class)}
	<TableRow {...props} class={className} tabindex={0}>
		<TableCell>
			<div class="flex items-center gap-2">
				{#if locationDisplay.flagUrl}
					<img
						src={locationDisplay.flagUrl}
						alt=""
						class="h-4 w-6 rounded-sm border border-border/60 object-cover"
						loading="lazy"
					/>
				{:else}
					<span class="text-xl" aria-hidden="true">{locationDisplay.flagEmoji}</span>
				{/if}
				<span class="text-sm font-medium text-foreground">{locationDisplay.label}</span>
				{#if locationDisplay.isVpn}
					<Badge
						variant="outline"
						class="border-amber-500 bg-amber-500/10 text-amber-500"
					>
						VPN
					</Badge>
				{/if}
			</div>
		</TableCell>
		<TableCell class="text-sm text-muted-foreground text-center">
			{agent.metadata.publicIpAddress ?? agent.metadata.ipAddress ?? 'Unknown'}
		</TableCell>
		<TableCell class="text-sm text-muted-foreground text-center">
			{agent.metadata.username}
		</TableCell>
		<TableCell class="text-sm text-muted-foreground text-center">
			{getAgentGroup(agent)}
		</TableCell>
		<TableCell class="text-center">
			<OsLogo os={agent.metadata.os} />
		</TableCell>
		<TableCell class="text-sm text-muted-foreground text-center">
			{formatPing(agent)}
		</TableCell>
		<TableCell class="text-sm text-muted-foreground text-center">
			{agent.metadata.version ?? 'N/A'}
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
