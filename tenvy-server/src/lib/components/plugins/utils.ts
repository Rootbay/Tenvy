import {
	formatRelativeTime,
	pluginDeliveryModeLabels,
	type Plugin,
	type PluginDeliveryMode,
	type PluginStatus
} from '$lib/data/plugin-view.js';
import { ShieldAlert, ShieldCheck } from '@lucide/svelte';

import type { MarketplaceStatus } from '$lib/data/marketplace.js';

type SignatureMeta = {
	label: string;
	icon: typeof ShieldCheck;
	class: string;
};

export const distributionModes: PluginDeliveryMode[] = ['manual', 'automatic'];

export function distributionNotice(plugin: Plugin): string {
	if (!plugin.enabled) return 'Plugin currently disabled';

	const notes = [`Default: ${pluginDeliveryModeLabels[plugin.distribution.defaultMode]}`];

	if (!plugin.distribution.allowManualPush) {
		notes.push('manual pushes blocked');
	}

	if (!plugin.distribution.allowAutoSync) {
		notes.push('auto-sync paused');
	}

	return notes.join(' · ');
}

export function statusSeverity(status: PluginStatus): string {
	switch (status) {
		case 'error':
			return 'text-red-500';
		case 'update':
			return 'text-amber-500';
		default:
			return 'text-muted-foreground';
	}
}

export function signatureBadge(signature: Plugin['signature']): SignatureMeta {
	switch (signature.status) {
		case 'trusted':
			return {
				label: 'Signature trusted',
				icon: ShieldCheck,
				class: 'border-emerald-500/40 text-emerald-500'
			} as const;
		case 'unsigned':
			return {
				label: 'Unsigned',
				icon: ShieldAlert,
				class: 'border-amber-500/40 text-amber-500'
			} as const;
		case 'untrusted':
			return {
				label: 'Untrusted signature',
				icon: ShieldAlert,
				class: 'border-amber-500/40 text-amber-500'
			} as const;
		case 'invalid':
		default:
			return {
				label: 'Signature invalid',
				icon: ShieldAlert,
				class: 'border-red-500/40 text-red-500'
			} as const;
	}
}

export function formatSignatureTime(value: string | null | undefined): string {
	if (!value) return 'never';
	const parsed = new Date(value);
	if (Number.isNaN(parsed.valueOf())) {
		return value;
	}
	return formatRelativeTime(parsed);
}

export const marketplaceStatusStyles: Record<MarketplaceStatus, string> = {
	approved: 'border border-emerald-500/40 bg-emerald-500/10 text-emerald-500',
	pending: 'border border-amber-500/40 bg-amber-500/10 text-amber-500',
	rejected: 'border border-red-500/40 bg-red-500/10 text-red-500'
};
