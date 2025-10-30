<script lang="ts">
	import WorkspaceContainer from './workspace-container.svelte';
	import { type ClientToolDefinition } from '$lib/data/client-tools';
	import type { PageData } from './$types';

	let { data } = $props<{ data: PageData }>();
	const client = $derived(data.client);
	const agent = $derived(data.agent);
	const tools = $derived((data.tools ?? []) as ClientToolDefinition[]);
</script>

<WorkspaceContainer {client} {agent} {tools}>
	<svelte:fragment slot="empty">
		<div class="space-y-6">
			<div class="rounded-lg border border-dashed border-border/60 bg-background/50 p-6">
				<h2 class="text-lg font-semibold">Select a module</h2>
				<p class="mt-2 text-sm text-muted-foreground">
					Launch a capability from the navigation panel to open its dedicated workspace inside the
					app.
				</p>
				<ul class="mt-4 list-disc space-y-1 pl-5 text-sm text-muted-foreground">
					<li>
						Workspace state is isolated per tool, matching the previous floating dialog behavior.
					</li>
					<li>
						Use the Close action to return to this overview or switch modules from the sidebar.
					</li>
				</ul>
			</div>
		</div>
	</svelte:fragment>
</WorkspaceContainer>
