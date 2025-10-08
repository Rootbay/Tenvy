import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { spawn } from 'node:child_process';
import type {
        ProcessListResponse,
        ProcessSummary,
        StartProcessRequest,
        StartProcessResponse
} from '$lib/types/task-manager';
import { splitCommandLine } from '$lib/utils/command';
import { listProcesses, toSummary } from '$lib/server/task-manager/process-utils';

function ensureArgs(payload: StartProcessRequest): {
        command: string;
        args: string[];
        cwd?: string;
        env?: Record<string, string>;
} {
        const trimmed = (payload.command || '').trim();
        if (!trimmed) {
                throw error(400, 'Command is required');
        }
        const baseArgs = Array.isArray(payload.args) ? payload.args.filter((item) => item.trim() !== '') : [];
        let command = trimmed;
        let args = baseArgs;
        if (baseArgs.length === 0) {
                const tokens = splitCommandLine(trimmed);
                if (tokens.length === 0) {
                        throw error(400, 'Command is required');
                }
                command = tokens[0];
                args = tokens.slice(1);
        }
        const env: Record<string, string> | undefined = payload.env
                ? Object.fromEntries(
                                Object.entries(payload.env)
                                        .filter(([key, value]) => typeof key === 'string' && typeof value === 'string')
                                        .map(([key, value]) => [key, value])
                        )
                : undefined;
        const cwd = payload.cwd && payload.cwd.trim() !== '' ? payload.cwd : undefined;
        return { command, args, cwd, env };
}

export const GET: RequestHandler = async () => {
        const list = await listProcesses();
        const processes: ProcessSummary[] = list.map((process) => toSummary(process));
        return json({ processes, generatedAt: new Date().toISOString() } satisfies ProcessListResponse);
};

export const POST: RequestHandler = async ({ request }) => {
        let payload: StartProcessRequest;
        try {
                payload = (await request.json()) as StartProcessRequest;
        } catch {
                throw error(400, 'Invalid JSON payload');
        }

        let args: ReturnType<typeof ensureArgs>;
        try {
                args = ensureArgs(payload);
        } catch (err) {
                if ('status' in (err as Error) && typeof (err as { status: number }).status === 'number') {
                        throw err;
                }
                throw error(400, (err as Error).message || 'Invalid command payload');
        }

        try {
                const child = spawn(args.command, args.args, {
                        cwd: args.cwd,
                        env: args.env ? { ...process.env, ...args.env } : process.env,
                        detached: true,
                        stdio: 'ignore'
                });

                const pid = child.pid;
                if (!pid) {
                        child.kill('SIGTERM');
                        throw error(500, 'Failed to start process');
                }

                child.unref();

                return json(
                        {
                                pid,
                                command: args.command,
                                args: args.args,
                                startedAt: new Date().toISOString()
                        } satisfies StartProcessResponse,
                        { status: 201 }
                );
        } catch (err) {
                throw error(500, `Failed to start process: ${(err as Error).message}`);
        }
};
