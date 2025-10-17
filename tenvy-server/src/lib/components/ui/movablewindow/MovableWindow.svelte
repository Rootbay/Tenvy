<script lang="ts">
	import { nextZ } from '$lib/stores/windowZ';

	let {
		title = 'Untitled',
		x = 100,
		y = 100,
		width = 400,
		height = 300,
		onClose,
		children
	} = $props<{
		title?: string;
		x?: number;
		y?: number;
		width?: number;
		height?: number;
		onClose?: () => void;
		children?: () => unknown;
	}>();

	let z = $state(10);
	let dragging = $state(false);
	let offsetX = 0;
	let offsetY = 0;
	let velocityX = 0;
	let velocityY = 0;
	let lastX = x;
	let lastY = y;
let windowRef: HTMLElement | null = null;
let isVisible = $state(true);

	function bringToFront() {
		z = nextZ();
	}

	function startDrag(e: PointerEvent) {
		const header = (e.target as HTMLElement).closest('.window-header');
		if (!header) return;
		dragging = true;
		offsetX = e.clientX - x;
		offsetY = e.clientY - y;
		lastX = e.clientX;
		lastY = e.clientY;
	}

	function moveDrag(e: PointerEvent) {
		if (!dragging) return;
		velocityX = e.clientX - lastX;
		velocityY = e.clientY - lastY;
		lastX = e.clientX;
		lastY = e.clientY;

		x = e.clientX - offsetX;
		y = e.clientY - offsetY;
	}

	function endDrag() {
		if (!dragging) return;
		dragging = false;

		const bounds = {
			maxX: window.innerWidth - width,
			maxY: window.innerHeight - height,
			minX: 0,
			minY: 0
		};

		const targetX = Math.min(bounds.maxX, Math.max(bounds.minX, x + velocityX * 25));
		const targetY = Math.min(bounds.maxY, Math.max(bounds.minY, y + velocityY * 25));

		x = targetX;
		y = targetY;
	}

function handlePointerDown(e: PointerEvent) {
	bringToFront();
	startDrag(e);
}

function handleClose() {
	if (onClose) {
		onClose();
		return;
	}
	isVisible = false;
}

	$effect(() => {
		document.addEventListener('pointermove', moveDrag);
		document.addEventListener('pointerup', endDrag);
		return () => {
			document.removeEventListener('pointermove', moveDrag);
			document.removeEventListener('pointerup', endDrag);
		};
	});
</script>

{#if isVisible}
	<div
		bind:this={windowRef}
		class="fixed flex flex-col overflow-hidden rounded-2xl border border-border bg-card text-card-foreground shadow-2xl select-none"
		style:top={`${y}px`}
		style:left={`${x}px`}
		style:width={`${width}px`}
		style:height={`${height}px`}
		style:z-index={z}
		onpointerdown={handlePointerDown}
	>
		<div
			class="window-header flex cursor-move items-center justify-between border-b border-border bg-muted/70 px-4 py-2 text-sm font-medium backdrop-blur-sm"
		>
			<span>{title}</span>
			<button
				class="h-3 w-3 rounded-full bg-destructive/80 transition-colors hover:bg-destructive"
				onclick={handleClose}
				aria-label="Close"
			></button>
		</div>

		{@render children()}
	</div>
{/if}
