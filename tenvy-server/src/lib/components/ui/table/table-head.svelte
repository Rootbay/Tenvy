<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';
	import { cn, type WithElementRef } from '$lib/utils.js';

	let {
		ref = $bindable(null),
		class: className,
		children,
		...restProps
	}: WithElementRef<HTMLAttributes<HTMLTableCellElement>, HTMLTableCellElement> & {
		children?: () => unknown;
	} = $props();
</script>

<th
	bind:this={ref}
	data-slot="table-head"
	class={cn(
		'h-12 px-4 text-left align-middle text-xs font-medium text-muted-foreground [&:has([role=checkbox])]:pr-0',
		className
	)}
	{...restProps}
>
	{@render children?.()}
</th>
