<script lang="ts">
        import WorkspaceContainer from '../workspace-container.svelte';
        import type { ClientToolDefinition } from '$lib/data/client-tools';
        import type { PageData } from './$types';

	let { data } = $props<{ data: PageData }>();
        const client = $derived(data.client);
        const agent = $derived(data.agent);
        const tool = $derived(data.tool);
        const segments = $derived(data.segments);
        const tools = $derived((data.tools ?? []) as ClientToolDefinition[]);
</script>

<WorkspaceContainer {client} {agent} {tools} activeTool={tool} {segments} />
