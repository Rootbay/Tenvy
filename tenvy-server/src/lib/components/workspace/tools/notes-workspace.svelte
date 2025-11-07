<script lang="ts">
	import { browser } from '$app/environment';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import type { Client } from '$lib/data/clients';

	const { client, class: className = '' } = $props<{
		client: Client;
		class?: string;
	}>();

	let noteText = $state(client.notes ?? '');
	let noteTagsInput = $state(client.tags?.join(' ') ?? '');
	let noteSavePending = $state(false);
	let noteSaveError = $state<string | null>(null);
	let noteSaveSuccess = $state<string | null>(null);

	const notesFieldId = `client-${client.id}-notes`;

	function parseTags(input: string): string[] {
		return input
			.split(/[\,\s]+/)
			.map((tag) => tag.trim())
			.filter(Boolean);
	}

	function clearNoteFeedback() {
		noteSaveError = null;
		noteSaveSuccess = null;
	}

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();

		noteSaveError = null;
		noteSaveSuccess = null;

		if (!browser) {
			noteSaveError = 'Notes cannot be saved in this environment';
			return;
		}

		const trimmed = noteText.trimEnd();
		const tags = parseTags(noteTagsInput);

		noteSavePending = true;

		try {
			const response = await fetch(`/api/agents/${client.id}/notes`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ note: trimmed, tags })
			});

			if (!response.ok) {
				const message = (await response.text())?.trim();
				noteSaveError = message || 'Failed to save notes';
				return;
			}

			let responseBody: unknown = null;
			try {
				responseBody = await response.json();
			} catch {
				responseBody = null;
			}

			let nextNote = trimmed;
			if (responseBody && typeof (responseBody as Record<string, unknown>).note === 'string') {
				const received = (responseBody as { note: string }).note ?? '';
				nextNote = received.trimEnd();
			}

			let nextTags = tags;
			if (responseBody && Array.isArray((responseBody as Record<string, unknown>).tags)) {
				nextTags = (responseBody as { tags: unknown[] }).tags
					.map((tag) => `${tag}`.trim())
					.filter(Boolean);
			}

			noteText = nextNote;
			client.notes = nextNote;
			noteTagsInput = nextTags.join(' ');
			noteSaveSuccess = 'Notes saved';
		} catch (err) {
			noteSaveError = err instanceof Error ? err.message : 'Failed to save notes';
		} finally {
			noteSavePending = false;
		}
	}
</script>

<form class={`flex h-full flex-col ${className}`} onsubmit={handleSubmit}>
	<div class="flex-1 space-y-6 overflow-auto px-6 py-5">
		<div class="grid gap-2">
			<Label for={notesFieldId}>Operational notes</Label>
			<Textarea
				id={notesFieldId}
				class="min-h-32"
				bind:value={noteText}
				on:input={clearNoteFeedback}
				placeholder={`Add context, requirements, or follow-up actions for ${client.codename}.`}
			/>
		</div>
		<div class="grid gap-2">
			<Label for={`${notesFieldId}-tags`}>Quick tags</Label>
			<Input
				id={`${notesFieldId}-tags`}
				bind:value={noteTagsInput}
				on:input={clearNoteFeedback}
				placeholder="intel priority staging"
			/>
		</div>
		{#if noteSaveError}
			<p class="text-sm text-destructive">{noteSaveError}</p>
		{:else if noteSaveSuccess}
			<p class="text-sm text-emerald-600">{noteSaveSuccess}</p>
		{/if}
	</div>
	<div class="flex items-center justify-end gap-2 border-t border-border/70 bg-muted/30 px-6 py-4">
		<slot name="secondary" let:noteSavePending />
		<Button type="submit" disabled={noteSavePending}>
			{#if noteSavePending}
				Saving…
			{:else}
				Save draft
			{/if}
		</Button>
	</div>
</form>
