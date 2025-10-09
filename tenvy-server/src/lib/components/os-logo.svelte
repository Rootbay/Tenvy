<script lang="ts">
	const { os } = $props<{ os: string }>();

	type VariantId = 'windows11' | 'windows10' | 'windows' | 'macos' | 'linux' | 'unknown';

	interface Variant {
		id: VariantId;
		label: string;
		source: string;
	}

	function normalize(value: string): string {
		return value.trim().toLowerCase();
	}

	function determineVariant(rawOs: string): Variant {
		const value = normalize(rawOs ?? '');

		if (value.includes('windows')) {
			if (value.includes('11')) {
				return { id: 'windows11', label: 'Windows 11', source: rawOs };
			}
			if (value.includes('10')) {
				return { id: 'windows10', label: 'Windows 10', source: rawOs };
			}
			return { id: 'windows', label: 'Windows', source: rawOs };
		}

		if (value.includes('mac') || value.includes('os x') || value.includes('darwin')) {
			return { id: 'macos', label: 'macOS', source: rawOs };
		}

		const linuxKeywords = [
			'linux',
			'ubuntu',
			'debian',
			'fedora',
			'arch',
			'centos',
			'gentoo',
			'suse'
		];
		if (linuxKeywords.some((keyword) => value.includes(keyword))) {
			return { id: 'linux', label: 'Linux', source: rawOs };
		}

		const label = rawOs.trim() === '' ? 'Unknown OS' : rawOs.trim();
		return { id: 'unknown', label, source: rawOs };
	}

	const variant = $derived(determineVariant(os));
</script>

<div class="flex items-center justify-center" aria-hidden="false">
	<span
		class="inline-flex h-10 w-10 items-center justify-center rounded-md border border-border/60 bg-muted/40"
		role="img"
		aria-label={variant.label}
		title={variant.label}
	>
		{#if variant.id === 'windows11'}
			<svg viewBox="0 0 48 48" class="h-6 w-6 text-sky-400" aria-hidden="true">
				<rect x="4" y="6" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="26" y="4" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="4" y="26" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="26" y="24" width="18" height="18" rx="1.5" fill="currentColor" />
			</svg>
		{:else if variant.id === 'windows10'}
			<svg viewBox="0 0 48 48" class="h-6 w-6 text-sky-600" aria-hidden="true">
				<rect x="4" y="6" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="26" y="4" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="4" y="26" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="26" y="24" width="18" height="18" rx="1.5" fill="currentColor" />
			</svg>
		{:else if variant.id === 'windows'}
			<svg viewBox="0 0 48 48" class="h-6 w-6 text-slate-500" aria-hidden="true">
				<rect x="4" y="6" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="26" y="4" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="4" y="26" width="18" height="18" rx="1.5" fill="currentColor" />
				<rect x="26" y="24" width="18" height="18" rx="1.5" fill="currentColor" />
			</svg>
		{:else if variant.id === 'macos'}
			<svg
				viewBox="0 0 48 48"
				class="h-6 w-6 text-slate-800 dark:text-slate-100"
				aria-hidden="true"
			>
				<path
					fill="currentColor"
					d="M33.6 24.4c-.1-4.3 3.5-6.6 3.7-6.7-2-2.9-5.2-3.3-6.3-3.3-2.7-.3-5.2 1.6-6.5 1.6-1.3 0-3.3-1.5-5.4-1.5-2.8.1-5.4 1.6-6.9 4-3 5.2-.7 12.8 2.3 17 1.5 2 3.1 4.3 5.3 4.2 2.1-.1 2.9-1.4 5.4-1.4 2.5 0 3.2 1.4 5.4 1.4 2.2-.1 3.6-2 5-4 1.6-2.3 2.3-4.6 2.4-4.7-.1 0-4.6-1.8-4.7-6.6z"
				/>
				<path
					fill="currentColor"
					d="M27 10.1c1.2-1.5 2-3.5 1.8-5.6-1.7.1-3.8 1.2-5.1 2.7-1.1 1.3-2.2 3.3-1.9 5.3 2 .1 3.9-1.1 5.2-2.4z"
				/>
			</svg>
		{:else if variant.id === 'linux'}
			<svg viewBox="0 0 48 48" class="h-6 w-6 text-slate-700" aria-hidden="true">
				<path
					fill="currentColor"
					d="M24 6c-4.9 0-8.9 4-8.9 8.9v3.1C11 20.6 8 25.4 8 30.8 8 38.6 14.6 44 24 44s16-5.4 16-13.2c0-5.4-3-10.2-7.1-12.8V14.9C32.9 10 28.9 6 24 6zm-5.2 26.9c-.9 0-1.7-.8-1.7-1.7 0-.9.8-1.7 1.7-1.7s1.7.8 1.7 1.7c0 .9-.8 1.7-1.7 1.7zm10.4 0c-.9 0-1.7-.8-1.7-1.7 0-.9.8-1.7 1.7-1.7s1.7.8 1.7 1.7c0 .9-.8 1.7-1.7 1.7z"
				/>
			</svg>
		{:else}
			<svg viewBox="0 0 48 48" class="h-6 w-6 text-muted-foreground" aria-hidden="true">
				<circle cx="24" cy="24" r="18" fill="none" stroke="currentColor" stroke-width="3.5" />
				<path
					d="M24 28v-1.2c0-2 1.3-3 2.6-3.9 1.3-.9 2.4-2 2.4-4.1 0-2.9-2.3-4.7-5.6-4.7-2.4 0-4.2.9-5.4 2.5"
					fill="none"
					stroke="currentColor"
					stroke-width="3.5"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
				<circle cx="24" cy="34.5" r="2.2" fill="currentColor" />
			</svg>
		{/if}
	</span>
	<span class="sr-only">{variant.source || variant.label}</span>
</div>
