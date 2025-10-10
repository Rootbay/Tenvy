<script lang="ts">
	import {
		Dialog as DialogRoot,
		DialogContent,
		DialogDescription,
		DialogFooter,
		DialogHeader,
		DialogTitle
	} from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { createEventDispatcher } from 'svelte';

	export let open: boolean;

	const dispatch = createEventDispatcher<{ close: void }>();

	function handleClose() {
		dispatch('close');
	}
</script>

<DialogRoot {open} onOpenChange={(value: boolean) => (!value ? handleClose() : null)}>
	<DialogContent class="sm:max-w-md">
		<DialogHeader>
			<DialogTitle>Connect an agent</DialogTitle>
			<DialogDescription>
				Generate a deployment command, execute it on the target system, and the agent will appear in
				this list once it connects.
			</DialogDescription>
		</DialogHeader>
		<div class="space-y-3 text-sm text-muted-foreground">
			<p>Install the client binary, provide the controller URL, and confirm network access.</p>
			<p>Agents automatically authenticate and begin reporting metrics immediately after launch.</p>
		</div>
		<DialogFooter>
			<Button type="button" variant="ghost" onclick={handleClose}>Close</Button>
		</DialogFooter>
	</DialogContent>
</DialogRoot>
