<script lang="ts">
	import type { HTMLTableAttributes } from 'svelte/elements';
	import { cn, type WithElementRef } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLTableAttributes, HTMLTableElement> & {
		children?: () => unknown;
	} = $props();
</script>

<table
	bind:this={ref}
	data-slot="table"
	class={cn('w-full caption-bottom text-sm', className)}
	{...restProps}
>
	{@render children?.()}
</table>
