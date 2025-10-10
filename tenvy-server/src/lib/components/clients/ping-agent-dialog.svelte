<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import {
		Dialog as DialogRoot,
		DialogContent,
		DialogDescription,
		DialogFooter,
		DialogHeader,
		DialogTitle
	} from '$lib/components/ui/dialog/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import AlertCircle from '@lucide/svelte/icons/alert-circle';
	import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';
	import { createEventDispatcher } from 'svelte';
	import type { AgentSnapshot } from '../../../../../shared/types/agent';

	export type PingDialogEvents = {
		close: void;
		submit: void;
		messageChange: string;
	};

	export let agent: AgentSnapshot;
	export let message: string;
	export let error: string | null = null;
	export let success: string | null = null;
	export let pending = false;

	const dispatch = createEventDispatcher<PingDialogEvents>();
</script>

<DialogRoot open onOpenChange={(value: boolean) => (!value ? dispatch('close') : null)}>
	<DialogContent class="sm:max-w-md">
		<DialogHeader>
			<DialogTitle>Send ping</DialogTitle>
			<DialogDescription
				>Queue a keep-alive message for {agent.metadata.hostname}.</DialogDescription
			>
		</DialogHeader>
		<div class="space-y-4">
			<div class="space-y-2">
				<Label for="ping-message" class="text-sm font-medium">Message</Label>
				<Input
					id="ping-message"
					placeholder="Optional message"
					value={message}
					oninput={(event) => dispatch('messageChange', event.currentTarget.value)}
				/>
			</div>
			{#if error}
				<Alert variant="destructive">
					<AlertCircle class="h-4 w-4" />
					<AlertTitle>Ping failed</AlertTitle>
					<AlertDescription>{error}</AlertDescription>
				</Alert>
			{:else if success}
				<Alert class="border-emerald-500/40 bg-emerald-500/10 text-emerald-500">
					<CheckCircle2 class="h-4 w-4" />
					<AlertTitle>Ping queued</AlertTitle>
					<AlertDescription>{success}</AlertDescription>
				</Alert>
			{/if}
		</div>
		<DialogFooter class="gap-2 sm:justify-between">
			<Button type="button" variant="ghost" onclick={() => dispatch('close')}>Cancel</Button>
			<Button type="button" onclick={() => dispatch('submit')} disabled={pending}>
				{#if pending}
					Sending…
				{:else}
					Queue ping
				{/if}
			</Button>
		</DialogFooter>
	</DialogContent>
</DialogRoot>
