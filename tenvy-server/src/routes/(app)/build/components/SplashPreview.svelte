<script lang="ts">
	import type { SplashLayout } from '../lib/constants.js';

	export let className = '';
	export let layout: SplashLayout;
	export let title: string;
	export let subtitle: string;
	export let message: string;
	export let accent: string;
	export let background: string;
	export let text: string;
</script>

<div
	class={`overflow-hidden rounded-lg border border-border/60 ${className}`}
	style={`background:${background};color:${text};`}
>
	{#if layout === 'split'}
		<div class="flex flex-col sm:flex-row">
			<div class="h-2 w-full sm:h-auto sm:w-2" style={`background:${accent};`}></div>
			<div class="flex-1 space-y-3 px-6 py-8 text-left sm:px-8">
				{#if subtitle}
					<p class="text-xs font-semibold tracking-wide uppercase opacity-80">
						{subtitle}
					</p>
				{/if}
				<h4 class="text-xl font-semibold">{title}</h4>
				<p class="text-sm leading-relaxed opacity-80">{message}</p>
			</div>
		</div>
	{:else}
		<div class="flex flex-col items-center gap-3 px-6 py-8 text-center">
			<div class="h-1.5 w-16 rounded-full" style={`background:${accent};`}></div>
			<h4 class="text-lg font-semibold">{title}</h4>
			{#if subtitle}
				<p class="text-xs font-semibold tracking-wide uppercase opacity-80">
					{subtitle}
				</p>
			{/if}
			<p class="text-sm leading-relaxed opacity-80">{message}</p>
		</div>
	{/if}
</div>
