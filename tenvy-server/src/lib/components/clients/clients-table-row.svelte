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
	import {
		Tooltip,
		TooltipContent,
		TooltipTrigger
	} from '$lib/components/ui/tooltip/index.js';
	import { TableCell } from '$lib/components/ui/table/index.js';
	import OsLogo from '$lib/components/os-logo.svelte';
	import { cn } from '$lib/utils.js';
	import { toast } from 'svelte-sonner';
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
	openManageTags,
	onTagClick,
	copyAgentId,
	getAgentLocation,
	getAgentTags,
	formatPing,
	formatDate
} = $props<{
	agent: AgentSnapshot;
	openSection: (section: SectionKey, agent: AgentSnapshot) => void;
	openManageTags: (agent: AgentSnapshot) => void;
	copyAgentId: (agentId: string) => void;
	onTagClick: (tag: string) => void;
	getAgentLocation: (agent: AgentSnapshot) => { label: string; flag: string };
	getAgentTags: (agent: AgentSnapshot) => string[];
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

function resolvePublicIpValue(agent: AgentSnapshot): string {
	return agent.metadata.publicIpAddress?.trim() || agent.metadata.ipAddress?.trim() || '';
}

function resolveUsernameValue(agent: AgentSnapshot): string {
	return agent.metadata.username?.trim() ?? '';
}

async function handleCopyValue(event: MouseEvent, rawValue: string, label: string) {
	event.stopPropagation();

	const value = rawValue.trim();
	if (!value) {
		toast.error(`No ${label} available to copy`, { position: 'bottom-right' });
		return;
	}

	if (!browser) {
		toast.error('Clipboard unavailable in this environment', { position: 'bottom-right' });
		return;
	}

	const clipboard = navigator.clipboard;
	if (!clipboard?.writeText) {
		toast.error('Clipboard API is not accessible', { position: 'bottom-right' });
		return;
	}

	try {
		await clipboard.writeText(value);
		toast.success(`${label} copied`, {
			description: value,
			position: 'bottom-right'
		});
	} catch (error) {
		console.error(`Failed to copy ${label}`, error);
		toast.error(`Failed to copy ${label}`, { position: 'bottom-right' });
	}
}

function buildStatusMeta(agent: AgentSnapshot): {
	label: string;
	className: string;
	indicatorClass: string;
	tooltip: string;
} {
	const connectedLabel = formatDate(agent.connectedAt);
	const lastSeenLabel = formatDate(agent.lastSeen);
	if (agent.status === 'online') {
		return {
			label: 'Online',
			className: 'text-emerald-500',
			indicatorClass: 'bg-emerald-500',
			tooltip: `Connected since ${connectedLabel}`
		};
	}
	if (agent.status === 'offline') {
		return {
			label: 'Offline',
			className: 'text-muted-foreground',
			indicatorClass: 'bg-muted-foreground',
			tooltip: `Last seen ${lastSeenLabel}`
		};
	}
	const referenceLabel = agent.lastSeen ? lastSeenLabel : connectedLabel;
	return {
		label: 'Error',
		className: 'text-rose-500',
		indicatorClass: 'bg-rose-500',
		tooltip: `Last seen ${referenceLabel}`
	};
}

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
	{@const className = cn(
		'cursor-context-menu border-b transition-colors hover:bg-muted/50 data-[state=selected]:bg-muted',
		(props as { class?: string }).class
	)}
	{@const tags = getAgentTags(agent)}
	{@const statusMeta = buildStatusMeta(agent)}
	{@const publicIpValue = resolvePublicIpValue(agent)}
	{@const publicIpDisplay = publicIpValue || 'Unknown'}
	{@const usernameValue = resolveUsernameValue(agent)}
	{@const usernameDisplay = usernameValue || 'Unknown'}
	<tr {...props} class={className} tabindex={0} data-slot="table-row">
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
					<Tooltip>
						<TooltipTrigger>
							{#snippet child({ props })}
								<span {...props}>
									<Badge
										variant="outline"
										class="border-amber-500 bg-amber-500/10 text-amber-500"
									>
										VPN
									</Badge>
								</span>
							{/snippet}
						</TooltipTrigger>
						<TooltipContent side="top" align="center" class="max-w-[18rem] text-xs">
							Flagged as a proxy, VPN, or Tor exit node.
						</TooltipContent>
					</Tooltip>
				{/if}
			</div>
		</TableCell>
		<TableCell class="text-center">
			<button
				type="button"
				class="inline-flex w-full items-center justify-center truncate rounded-md px-2 py-1 text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background cursor-pointer"
				onclick={(event) => handleCopyValue(event, publicIpValue, 'Public IP')}
				title={publicIpDisplay === 'Unknown' ? 'Public IP unavailable' : `Copy ${publicIpDisplay}`}
				aria-label={publicIpDisplay === 'Unknown' ? 'Public IP unavailable' : `Copy public IP ${publicIpDisplay}`}
			>
				<span class="truncate">{publicIpDisplay}</span>
			</button>
		</TableCell>
		<TableCell class="text-center">
			<button
				type="button"
				class="inline-flex w-full items-center justify-center truncate rounded-md px-2 py-1 text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background cursor-pointer"
				onclick={(event) => handleCopyValue(event, usernameValue, 'Username')}
				title={usernameDisplay === 'Unknown' ? 'Username unavailable' : `Copy ${usernameDisplay}`}
				aria-label={usernameDisplay === 'Unknown' ? 'Username unavailable' : `Copy username ${usernameDisplay}`}
			>
				<span class="truncate">{usernameDisplay}</span>
			</button>
		</TableCell>
		<TableCell class="text-center">
			{#if tags.length > 0}
				<div class="flex flex-wrap items-center justify-center gap-1">
					{#each tags as tag (tag)}
						<button
							type="button"
							class="group rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background cursor-pointer"
							onclick={(event) => {
								event.stopPropagation();
								onTagClick(tag);
							}}
							aria-label={`Filter by ${tag}`}
						>
							<Badge
								variant="secondary"
								class="px-2 py-0.5 text-xs font-medium transition-colors group-focus-visible:ring-2 group-focus-visible:ring-ring"
							>
								{tag}
							</Badge>
						</button>
					{/each}
				</div>
			{:else}
				<Badge variant="outline" class="border-dashed px-2 py-0.5 text-xs text-muted-foreground">
					Untagged
				</Badge>
			{/if}
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
		<TableCell class="text-center">
			<Tooltip>
				<TooltipTrigger>
					{#snippet child({ props })}
						<span
							{...props}
							class={cn(
								'inline-flex w-full items-center justify-center gap-2 text-sm font-medium',
								statusMeta.className
							)}
						>
							<span
								class={cn('h-2 w-2 rounded-full', statusMeta.indicatorClass)}
								aria-hidden="true"
							></span>
							{statusMeta.label}
						</span>
					{/snippet}
				</TooltipTrigger>
				<TooltipContent side="top" align="center" class="max-w-[18rem] text-xs">
					{statusMeta.tooltip}
				</TooltipContent>
			</Tooltip>
		</TableCell>
	</tr>
{/snippet}

<ContextMenu>
	<ContextMenuTrigger child={TriggerChild} />
	<ContextMenuContent class="w-56">
		<ContextMenuItem onSelect={() => openSection('systemInfo', agent)}>System Info</ContextMenuItem>
		<ContextMenuItem onSelect={() => openSection('notes', agent)}>Notes</ContextMenuItem>
		<ContextMenuItem onSelect={() => openManageTags(agent)}>Manage Tags</ContextMenuItem>

		<ContextMenuSeparator />

		<ContextMenuSub>
			<ContextMenuSubTrigger>Control</ContextMenuSubTrigger>
			<ContextMenuSubContent class="w-48">
				<ContextMenuItem onSelect={() => openSection('appVnc', agent)}>
					App VNC
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
