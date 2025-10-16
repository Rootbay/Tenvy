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
				Go to the <strong>Build</strong> page by clicking the <strong>Build</strong> button in the sidebar. Choose your desired host, port, and any additional options. Then click <strong>Build Agent</strong>, and the generated client binary will be saved in \tenvy-server\static\builds.
			</DialogDescription>
		</DialogHeader>
		<div class="space-y-3 text-sm text-muted-foreground">
			<p>Send the binary to the target system and execute it - once the client connects, it will automatically appear in this list.</p>
		</div>
		<DialogFooter>
			<Button type="button" variant="ghost" onclick={handleClose}>Close</Button>
		</DialogFooter>
	</DialogContent>
</DialogRoot>
