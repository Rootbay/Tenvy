<script lang="ts">
	import { onMount } from 'svelte';
	import type { DashboardClient } from '$lib/data/dashboard';

        const { clients, highlightCountry } = $props<{
                clients?: DashboardClient[];
                highlightCountry?: string | null;
        }>();

	let MapComponent = $state<typeof import('./client-presence-map.svelte').default | null>(null);

	onMount(async () => {
		const module = await import('./client-presence-map.svelte');
		MapComponent = module.default;
	});
</script>

{#if MapComponent}
	{@const ClientPresenceMap = MapComponent}
	<ClientPresenceMap {clients} {highlightCountry} />
{:else}
	<div
		role="img"
		aria-label="Loading client presence map"
		aria-busy="true"
		class="relative h-full min-h-[280px] w-full overflow-hidden rounded-xl border border-border/60 bg-linear-to-br from-background via-background to-muted/30"
	>
		<div
			class="absolute inset-0 animate-pulse bg-linear-to-br from-muted/40 via-transparent to-muted/20"
		></div>
		<div class="pointer-events-none absolute inset-0 flex items-center justify-center">
			<div class="flex flex-col items-center gap-3 text-xs text-muted-foreground">
				<span class="h-10 w-10 animate-pulse rounded-full border border-border/60 bg-muted/60"
				></span>
				<span>Loading map…</span>
			</div>
		</div>
	</div>
{/if}
