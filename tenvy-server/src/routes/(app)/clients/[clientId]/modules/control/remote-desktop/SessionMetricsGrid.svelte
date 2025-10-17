<script lang="ts">
	import { formatLatency, formatMetric, formatResolution } from './formatters';

	let { fps, bandwidth, streamWidth, streamHeight, latencyMs } = $props<{
		fps: number | null;
		bandwidth: number | null;
		streamWidth: number | null;
		streamHeight: number | null;
		latencyMs: number | null;
	}>();
</script>

<div class="mt-2 grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
	<div class="rounded-lg border border-border/60 bg-background/60 p-3">
		<p class="text-xs text-muted-foreground uppercase">FPS</p>
		<p class="text-sm font-semibold text-foreground">{formatMetric(fps, 'fps')}</p>
	</div>
	<div class="rounded-lg border border-border/60 bg-background/60 p-3">
		<p class="text-xs text-muted-foreground uppercase">Bandwidth</p>
		<p class="text-sm font-semibold text-foreground">{formatMetric(bandwidth, 'kbps')}</p>
	</div>
	<div class="rounded-lg border border-border/60 bg-background/60 p-3">
		<p class="text-xs text-muted-foreground">Resolution</p>
		<p class="text-sm font-semibold text-foreground">
			{formatResolution(streamWidth, streamHeight)}
		</p>
	</div>
	<div class="rounded-lg border border-border/60 bg-background/60 p-3">
		<p class="text-xs text-muted-foreground">Latency</p>
		<p class="text-sm font-semibold text-foreground">{formatLatency(latencyMs)}</p>
	</div>
</div>
