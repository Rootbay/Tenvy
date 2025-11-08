<script lang="ts">
	import SystemInfoWorkspace from '$lib/components/workspace/tools/system-info-workspace.svelte';
	import type { Client } from '$lib/data/clients';

	const { client } = $props<{ client: Client }>();
</script>

<SystemInfoWorkspace {client} surface="dialog" />
