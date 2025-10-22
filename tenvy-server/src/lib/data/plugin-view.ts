export type PluginStatus = 'active' | 'disabled' | 'update' | 'error';

export type PluginCategory =
	| 'collection'
	| 'operations'
	| 'persistence'
	| 'exfiltration'
	| 'transport'
	| 'recovery';

export type PluginDeliveryMode = 'manual' | 'automatic';

export const pluginCategories: PluginCategory[] = [
	'collection',
	'operations',
	'persistence',
	'exfiltration',
	'transport',
	'recovery'
];

export const pluginCategoryLabels: Record<PluginCategory, string> = {
	collection: 'Collection',
	operations: 'Operations',
	persistence: 'Persistence',
	exfiltration: 'Exfiltration',
	transport: 'Transport',
	recovery: 'Recovery'
};

export const pluginDeliveryModeLabels: Record<PluginDeliveryMode, string> = {
	manual: 'Manual delivery',
	automatic: 'Automatic sync'
};

export const pluginStatusLabels: Record<PluginStatus, string> = {
	active: 'Active',
	disabled: 'Disabled',
	update: 'Update available',
	error: 'Attention required'
};

export const pluginStatusStyles: Record<PluginStatus, string> = {
	active: 'border-emerald-500/40 text-emerald-500',
	disabled: 'border-muted text-muted-foreground',
	update: 'border-amber-500/60 text-amber-500',
	error: 'border-red-500/60 text-red-500'
};

export type PluginApprovalStatus = 'pending' | 'approved' | 'rejected';

export const pluginApprovalLabels: Record<PluginApprovalStatus, string> = {
	pending: 'Pending approval',
	approved: 'Approved',
	rejected: 'Rejected'
};

export type PluginDistributionView = {
	defaultMode: PluginDeliveryMode;
	allowManualPush: boolean;
	allowAutoSync: boolean;
	manualTargets: number;
	autoTargets: number;
	lastManualPush: string;
	lastAutoSync: string;
};

export type Plugin = {
	id: string;
	name: string;
	description: string;
	version: string;
	author: string;
	category: PluginCategory;
	status: PluginStatus;
	enabled: boolean;
	autoUpdate: boolean;
	installations: number;
	lastDeployed: string;
	lastChecked: string;
	size: string;
	capabilities: string[];
	artifact: string;
	distribution: PluginDistributionView;
	requiredModules: { id: string; title: string }[];
	approvalStatus: PluginApprovalStatus;
	approvedAt?: string;
	signature: PluginSignatureState;
};

export type PluginSignatureState = {
	status: PluginSignatureStatus;
	trusted: boolean;
	type: PluginSignatureType;
	hash?: string | null;
	signer?: string | null;
	publicKey?: string | null;
	signedAt?: string | null;
	checkedAt?: string | null;
	error?: string | null;
	errorCode?: string | null;
	certificateChain?: string[] | null;
};

export type PluginUpdatePayload = {
	status?: PluginStatus;
	enabled?: boolean;
	autoUpdate?: boolean;
	installations?: number;
	distribution?: {
		defaultMode?: PluginDeliveryMode;
		allowManualPush?: boolean;
		allowAutoSync?: boolean;
		manualTargets?: number;
		autoTargets?: number;
	};
	approvalStatus?: PluginApprovalStatus;
	approvedAt?: Date | null;
	approvalNote?: string | null;
};

const BYTES_IN_KIB = 1024;
const RELATIVE_THRESHOLDS = {
	minute: 60,
	hour: 60,
	day: 24,
	week: 7
} as const;

export function formatFileSize(bytes?: number | null): string {
	if (!bytes || bytes <= 0) return 'unknown';

	const units = ['bytes', 'KB', 'MB', 'GB'];
	let value = bytes;
	let unitIndex = 0;

	while (value >= BYTES_IN_KIB && unitIndex < units.length - 1) {
		value /= BYTES_IN_KIB;
		unitIndex += 1;
	}

	return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

export function formatRelativeTime(input?: Date | null): string {
	if (!input) return 'never';

	const now = new Date();
	const diffMs = now.getTime() - input.getTime();

	if (!Number.isFinite(diffMs)) return 'never';

	const diffSeconds = Math.round(diffMs / 1000);

	if (diffSeconds < 30) return 'just now';
	if (diffSeconds < RELATIVE_THRESHOLDS.minute * 2) {
		return '1 minute ago';
	}

	const diffMinutes = Math.round(diffSeconds / RELATIVE_THRESHOLDS.minute);
	if (diffMinutes < RELATIVE_THRESHOLDS.hour) {
		return `${diffMinutes} minute${diffMinutes === 1 ? '' : 's'} ago`;
	}

	const diffHours = Math.round(diffMinutes / RELATIVE_THRESHOLDS.hour);
	if (diffHours < RELATIVE_THRESHOLDS.day) {
		return `${diffHours} hour${diffHours === 1 ? '' : 's'} ago`;
	}

	const diffDays = Math.round(diffHours / RELATIVE_THRESHOLDS.day);
	if (diffDays < 14) {
		return `${diffDays} day${diffDays === 1 ? '' : 's'} ago`;
	}

	const diffWeeks = Math.round(diffDays / RELATIVE_THRESHOLDS.week);
	if (diffWeeks < 9) {
		return `${diffWeeks} week${diffWeeks === 1 ? '' : 's'} ago`;
	}

	return input.toISOString();
}
import type {
	PluginSignatureStatus,
	PluginSignatureType
} from '../../../../shared/types/plugin-manifest.js';
