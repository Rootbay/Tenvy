<script lang="ts">
	import { Pagination as PaginationPrimitive } from 'bits-ui';

	import { cn } from '$lib/utils.js';

	type $$Props = PaginationPrimitive.RootProps;

	type ChildSnippetProps = Parameters<NonNullable<PaginationPrimitive.RootProps['child']>>[0];
	type PaginationRange = { start: number; end: number };
	type PaginationItem =
		| { type: 'page'; value: number; key: string }
		| { type: 'ellipsis'; key: string };

	interface $$Slots {
		default: {
			pages: PaginationItem[];
			range: PaginationRange;
			currentPage: number;
		};
	}

	let {
		ref = $bindable(null),
		class: className,
		children,
		count = 0,
		perPage = 10,
		page = $bindable(1),
		siblingCount = 1,
		...restProps
	}: PaginationPrimitive.RootProps = $props();
</script>

{#snippet RootChild({ props, pages, range, currentPage }: ChildSnippetProps)}
	<div
		{...props}
		class={cn('mx-auto flex w-full justify-center', className, props.class as string | undefined)}
	>
		{@render children?.({ pages, range, currentPage })}
	</div>
{/snippet}

<PaginationPrimitive.Root
	bind:ref
	bind:page
	role="navigation"
	aria-label="pagination"
	data-slot="pagination"
	child={RootChild}
	{count}
	{perPage}
	{siblingCount}
	{...restProps}
/>
