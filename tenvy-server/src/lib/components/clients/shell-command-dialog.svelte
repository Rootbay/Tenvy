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

	export type ShellDialogEvents = {
		close: void;
		submit: void;
		commandChange: string;
		timeoutChange: number | undefined;
	};

	export let agent: AgentSnapshot;
	export let command: string;
	export let timeout: number | undefined;
	export let error: string | null = null;
	export let success: string | null = null;
	export let pending = false;

	const dispatch = createEventDispatcher<ShellDialogEvents>();
</script>

<DialogRoot open onOpenChange={(value: boolean) => (!value ? dispatch('close') : null)}>
	<DialogContent class="sm:max-w-lg">
		<DialogHeader>
			<DialogTitle>Run shell command</DialogTitle>
			<DialogDescription
				>Dispatch a command for {agent.metadata.hostname} to execute.</DialogDescription
			>
		</DialogHeader>
		<div class="space-y-4">
			<div class="space-y-2">
				<Label for="shell-command" class="text-sm font-medium">Command</Label>
				<textarea
					id="shell-command"
					class="min-h-28 w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus:ring-2 focus:ring-primary focus:outline-none"
					rows={4}
					placeholder="whoami"
					value={command}
					oninput={(event) => dispatch('commandChange', event.currentTarget.value)}
				></textarea>
			</div>
			<div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center">
				<div class="space-y-2">
					<Label for="shell-timeout" class="text-sm font-medium">Timeout (seconds)</Label>
					<Input
						id="shell-timeout"
						type="number"
						min={1}
						placeholder="Optional timeout"
						value={timeout ?? ''}
						oninput={(event) => {
							const raw = event.currentTarget.value;
							const value = Number.parseInt(raw, 10);
							dispatch(
								'timeoutChange',
								raw.trim() === '' || Number.isNaN(value) ? undefined : value
							);
						}}
					/>
				</div>
				<p class="text-xs text-muted-foreground">Leave blank to use the agent default.</p>
			</div>
			{#if error}
				<Alert variant="destructive">
					<AlertCircle class="h-4 w-4" />
					<AlertTitle>Shell dispatch failed</AlertTitle>
					<AlertDescription>{error}</AlertDescription>
				</Alert>
			{:else if success}
				<Alert class="border-emerald-500/40 bg-emerald-500/10 text-emerald-500">
					<CheckCircle2 class="h-4 w-4" />
					<AlertTitle>Shell command queued</AlertTitle>
					<AlertDescription>{success}</AlertDescription>
				</Alert>
			{/if}
		</div>
		<DialogFooter class="gap-2 sm:justify-between">
			<Button type="button" variant="ghost" onclick={() => dispatch('close')}>Cancel</Button>
			<Button type="button" onclick={() => dispatch('submit')} disabled={pending}>
				{#if pending}
					Dispatching…
				{:else}
					Queue command
				{/if}
			</Button>
		</DialogFooter>
	</DialogContent>
</DialogRoot>
