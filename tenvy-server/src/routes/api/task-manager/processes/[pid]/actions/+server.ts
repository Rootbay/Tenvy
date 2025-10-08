import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { promisify } from 'node:util';
import { execFile, spawn } from 'node:child_process';
import { platform } from 'node:os';
import type { ProcessActionRequest, ProcessActionResponse } from '$lib/types/task-manager';
import { findProcess, toDetail } from '$lib/server/task-manager/process-utils';
import { splitCommandLine } from '$lib/utils/command';

const execFileAsync = promisify(execFile);

async function stopProcess(pid: number, { force = false } = {}) {
        if (platform() === 'win32') {
                const args = ['/pid', pid.toString(), '/t'];
                if (force) {
                        args.push('/f');
                }
                try {
                        await execFileAsync('taskkill', args);
                } catch (err) {
                        const message = (err as Error).message;
                        if (message.includes('not found') || message.includes('No instance')) {
                                throw error(404, 'Process not found');
                        }
                        throw error(500, `Failed to terminate process: ${message}`);
                }
                return;
        }
        try {
                process.kill(pid, force ? 'SIGKILL' : 'SIGTERM');
        } catch (err) {
                const code = (err as NodeJS.ErrnoException).code;
                if (code === 'ESRCH') {
                        throw error(404, 'Process not found');
                }
                if (code === 'EPERM') {
                        throw error(403, 'Insufficient privileges to terminate process');
                }
                throw error(500, `Failed to terminate process: ${(err as Error).message}`);
        }
}

async function suspendProcess(pid: number) {
        if (platform() === 'win32') {
                try {
                        await execFileAsync('powershell.exe', [
                                '-NoProfile',
                                '-Command',
                                `Suspend-Process -Id ${pid}`
                        ]);
                } catch (err) {
                        throw error(500, `Failed to suspend process: ${(err as Error).message}`);
                }
                return;
        }
        try {
                process.kill(pid, 'SIGSTOP');
        } catch (err) {
                const code = (err as NodeJS.ErrnoException).code;
                if (code === 'ESRCH') {
                        throw error(404, 'Process not found');
                }
                if (code === 'EPERM') {
                        throw error(403, 'Insufficient privileges to suspend process');
                }
                throw error(500, `Failed to suspend process: ${(err as Error).message}`);
        }
}

async function resumeProcess(pid: number) {
        if (platform() === 'win32') {
                try {
                        await execFileAsync('powershell.exe', [
                                '-NoProfile',
                                '-Command',
                                `Resume-Process -Id ${pid}`
                        ]);
                } catch (err) {
                        throw error(500, `Failed to resume process: ${(err as Error).message}`);
                }
                return;
        }
        try {
                process.kill(pid, 'SIGCONT');
        } catch (err) {
                const code = (err as NodeJS.ErrnoException).code;
                if (code === 'ESRCH') {
                        throw error(404, 'Process not found');
                }
                if (code === 'EPERM') {
                        throw error(403, 'Insufficient privileges to resume process');
                }
                throw error(500, `Failed to resume process: ${(err as Error).message}`);
        }
}

async function restartProcess(pid: number): Promise<number> {
        const processInfo = await findProcess(pid);
        if (!processInfo) {
                throw error(404, 'Process not found');
        }
        const detail = toDetail(processInfo);
        const commandTokens = detail.path
                ? [detail.path]
                : detail.command
                ? splitCommandLine(detail.command)
                : [];
        if (commandTokens.length === 0) {
                throw error(400, 'Unable to determine command used to start the process');
        }
        const args = detail.arguments && detail.arguments.length > 0 ? detail.arguments : commandTokens.slice(1);
        const command = detail.path ?? commandTokens[0];

        await stopProcess(pid, { force: true });

        try {
                const child = spawn(command, args, {
                        detached: true,
                        stdio: 'ignore'
                });
                const newPid = child.pid;
                if (!newPid) {
                        child.kill('SIGTERM');
                        throw error(500, 'Failed to restart process');
                }
                child.unref();
                return newPid;
        } catch (err) {
                throw error(500, `Failed to restart process: ${(err as Error).message}`);
        }
}

function validatePayload(payload: ProcessActionRequest | undefined): ProcessActionRequest {
        if (!payload || typeof payload.action !== 'string') {
                        throw error(400, 'Invalid action payload');
        }
        const action = payload.action;
        if (!['stop', 'force-stop', 'suspend', 'resume', 'restart'].includes(action)) {
                throw error(400, 'Unsupported process action');
        }
        return { action: action as ProcessActionRequest['action'] };
}

export const POST: RequestHandler = async ({ params, request }) => {
        const pidValue = params.pid;
        const pid = Number.parseInt(pidValue ?? '', 10);
        if (!Number.isInteger(pid) || pid <= 0) {
                throw error(400, 'Invalid process identifier');
        }

        let payload: ProcessActionRequest | undefined;
        try {
                payload = (await request.json()) as ProcessActionRequest;
        } catch {
                throw error(400, 'Invalid JSON payload');
        }

        const action = validatePayload(payload);

        switch (action.action) {
                case 'stop':
                        await stopProcess(pid);
                        break;
                case 'force-stop':
                        await stopProcess(pid, { force: true });
                        break;
                case 'suspend':
                        await suspendProcess(pid);
                        break;
                case 'resume':
                        await resumeProcess(pid);
                        break;
                case 'restart':
                        await restartProcess(pid);
                        break;
                default:
                        throw error(400, 'Unsupported process action');
        }

        return json({ pid, action: action.action, status: 'ok' } satisfies ProcessActionResponse);
};
