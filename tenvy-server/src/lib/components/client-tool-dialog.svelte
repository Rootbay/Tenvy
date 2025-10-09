<script lang="ts">
	import { browser } from '$app/environment';
	import { createEventDispatcher } from 'svelte';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import type { Client } from '$lib/data/clients';
	import { buildClientToolUrl, getClientTool, type DialogToolId } from '$lib/data/client-tools';

	const { toolId, client } = $props<{ toolId: DialogToolId; client: Client }>();

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

	function openWorkspace() {
		if (!browser) return;
		window.open(workspaceUrl, '_blank', 'noopener,noreferrer');
	}

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
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>{tool.title}</Dialog.Title>
			<Dialog.Description>
				{tool.description}
			</Dialog.Description>
		</Dialog.Header>

		{#if toolId === 'system-info'}
			<div class="grid gap-6">
				<div class="grid gap-3 text-sm">
					<div
						class="flex flex-wrap items-center gap-2 text-xs font-medium tracking-wide text-muted-foreground uppercase"
					>
						<span>Client</span>
						<span class="rounded-full bg-primary/10 px-2 py-0.5 text-primary"
							>{client.codename}</span
						>
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

			<Dialog.Footer>
				<Dialog.Close>
					{#snippet child({ props })}
						<Button variant="outline" {...props}>Close</Button>
					{/snippet}
				</Dialog.Close>
				<Button variant="ghost" type="button" onclick={openWorkspace}>Open workspace</Button>
			</Dialog.Footer>
		{:else if toolId === 'notes'}
			<form class="grid gap-6" onsubmit={handleFormSubmit}>
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
				<Dialog.Footer>
					<Dialog.Close>
						{#snippet child({ props })}
							<Button variant="outline" {...props}>Cancel</Button>
						{/snippet}
					</Dialog.Close>
					<Button variant="ghost" type="button" onclick={openWorkspace}>Open workspace</Button>
					<Button type="submit">Save draft</Button>
				</Dialog.Footer>
			</form>
		{:else if toolId === 'open-url'}
			<form class="grid gap-6" onsubmit={handleOpenUrlSubmit}>
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
					The request will stage in the task queue for {client.codename}. Confirmation flow and
					auditing hooks are planned here.
				</p>
				<Dialog.Footer>
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
				</Dialog.Footer>
			</form>
		{:else}
			<form class="grid gap-6" onsubmit={handleFormSubmit}>
				<div class="grid gap-2">
					<Label for={messageTitleId}>Title</Label>
					<Input id={messageTitleId} bind:value={messageTitle} placeholder="System notice" />
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
					<select id={messageStyleId} class={selectClasses} bind:value={messageStyle}>
						<option value="info">Information</option>
						<option value="warning">Warning</option>
						<option value="critical">Critical</option>
					</select>
				</div>
				<p class="text-xs text-muted-foreground">
					Delivery styling and acknowledgement capture will integrate here in a subsequent
					iteration.
				</p>
				<Dialog.Footer>
					<Dialog.Close>
						{#snippet child({ props })}
							<Button variant="outline" {...props}>Cancel</Button>
						{/snippet}
					</Dialog.Close>
					<Button type="submit">Queue message</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>
