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
	let resizing = $state(false);
	let resizeDir = $state('');
	let offsetX = $state(0);
	let offsetY = $state(0);
	let velocityX = $state(0);
	let velocityY = $state(0);
	let lastX = $state(x);
	let lastY = $state(y);
	let lastWidth = $state(width);
	let lastHeight = $state(height);
	let windowRef = $state<HTMLElement | null>(null);
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
		if (resizing) {
			handleResizeMove(e);
			return;
		}
		if (!dragging) return;
		velocityX = e.clientX - lastX;
		velocityY = e.clientY - lastY;
		lastX = e.clientX;
		lastY = e.clientY;

		x = e.clientX - offsetX;
		y = e.clientY - offsetY;
	}

	function endDrag() {
		if (dragging) {
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
		if (resizing) {
			resizing = false;
			resizeDir = '';
		}
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

	function handleFullscreen() {
		if (windowRef) {
			if (windowRef.style.width === '100vw' && windowRef.style.height === '100vh') {
				windowRef.style.width = `${width}px`;
				windowRef.style.height = `${height}px`;
				x = lastX;
				y = lastY;
			} else {
				lastX = x;
				lastY = y;
				windowRef.style.width = '100vw';
				windowRef.style.height = '100vh';
				x = 0;
				y = 0;
			}
		}
	}

	function startResize(e: PointerEvent, dir: string) {
		resizing = true;
		resizeDir = dir;
		lastX = e.clientX;
		lastY = e.clientY;
		lastWidth = width;
		lastHeight = height;
		offsetX = e.clientX;
		offsetY = e.clientY;
		e.stopPropagation();
	}

	function handleResizeMove(e: PointerEvent) {
		const dx = e.clientX - lastX;
		const dy = e.clientY - lastY;

		if (resizeDir.includes('right')) width = Math.max(200, lastWidth + dx);
		if (resizeDir.includes('bottom')) height = Math.max(150, lastHeight + dy);
		if (resizeDir.includes('left')) {
			const newWidth = Math.max(200, lastWidth - dx);
			if (newWidth !== width) {
				x += dx;
				width = newWidth;
			}
		}
		if (resizeDir.includes('top')) {
			const newHeight = Math.max(150, lastHeight - dy);
			if (newHeight !== height) {
				y += dy;
				height = newHeight;
			}
		}
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
			<div class="flex gap-1">
				<button
					class="h-3 w-3 cursor-pointer rounded-full bg-green-500/80 transition-colors hover:bg-green-500"
					onclick={handleFullscreen}
					aria-label="Fullscreen"
				></button>
				<button
					class="h-3 w-3 cursor-pointer rounded-full bg-destructive/80 transition-colors hover:bg-destructive"
					onclick={handleClose}
					aria-label="Close"
				></button>
			</div>
		</div>

		{@render children()}

		<div
			onpointerdown={(e) => startResize(e, 'bottom')}
			class="absolute right-0 bottom-0 left-0 h-1 cursor-s-resize"
		></div>
		<div
			onpointerdown={(e) => startResize(e, 'right')}
			class="absolute top-0 right-0 bottom-0 w-1 cursor-e-resize"
		></div>
		<div
			onpointerdown={(e) => startResize(e, 'bottom-right')}
			class="absolute right-0 bottom-0 h-2 w-2 cursor-se-resize"
		></div>
	</div>
{/if}
