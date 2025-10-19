<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import MovableWindow from '$lib/components/ui/movablewindow/MovableWindow.svelte';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import RemoteDesktopWorkspace from '$lib/components/workspace/tools/remote-desktop-workspace.svelte';
	import type { RemoteDesktopSessionState } from '$lib/types/remote-desktop';

	const tool = getClientTool('remote-desktop');
	const windowWidth = 1280;
	const windowHeight = 780;

	let { data } = $props<{
		data: {
			session: RemoteDesktopSessionState | null;
			client: Client;
		};
	}>();

	const client = $derived(data.client);
	const initialSession = $derived(data.session ?? null);

	function handleClose() {
		void goto(resolve(`/clients/${client.id}/modules`));
	}
</script>

<MovableWindow
	title={tool.title}
	width={windowWidth}
	height={windowHeight}
	onClose={handleClose}
>
	<div class="flex h-full flex-col bg-background">
		<div class="border-b border-border/70 bg-muted/40 px-6 py-4 text-sm text-muted-foreground">
			{tool.description}
		</div>
		<div class="flex-1 overflow-auto px-6 py-5">
			<RemoteDesktopWorkspace client={client} initialSession={initialSession} />
		</div>
	</div>
</MovableWindow>
