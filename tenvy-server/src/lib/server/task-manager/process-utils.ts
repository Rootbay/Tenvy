import si from 'systeminformation';
import type {
        ProcessDetail,
        ProcessStatus,
        ProcessSummary
} from '$lib/types/task-manager';
import { splitCommandLine } from '$lib/utils/command';

function parseDate(value?: string | null): string | undefined {
        if (!value || value === '0') {
                return undefined;
        }
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) {
                return undefined;
        }
        return parsed.toISOString();
}

export function normalizeStatus(input?: string | null): ProcessStatus {
        if (!input) {
                return 'unknown';
        }
        const value = input.toLowerCase();
        if (value.length === 1) {
                switch (value) {
                        case 'r':
                                return 'running';
                        case 's':
                                return 'sleeping';
                        case 't':
                        case 'x':
                                return 'stopped';
                        case 'i':
                                return 'idle';
                        case 'z':
                                return 'zombie';
                        default:
                                return 'unknown';
                }
        }
        if (value.includes('suspend')) {
                return 'suspended';
        }
        if (value.includes('sleep')) {
                return 'sleeping';
        }
        if (value.includes('stop')) {
                return 'stopped';
        }
        if (value.includes('idle')) {
                return 'idle';
        }
        if (value.includes('zombie') || value.includes('defunct')) {
                return 'zombie';
        }
        if (value.includes('run')) {
                return 'running';
        }
        return 'unknown';
}

export function toSummary(process: si.Systeminformation.ProcessesProcessData): ProcessSummary {
        const memoryBytes = Number.isFinite(process.memRss) ? process.memRss : 0;
        return {
                pid: process.pid,
                ppid: process.parentPid || undefined,
                name: process.name,
                cpu: Number.isFinite(process.cpu) ? process.cpu : 0,
                memory: memoryBytes,
                status: normalizeStatus(process.state),
                command: process.command || process.path || process.name,
                user: process.user || undefined,
                startedAt: parseDate(process.started)
        } satisfies ProcessSummary;
}

export function toDetail(process: si.Systeminformation.ProcessesProcessData): ProcessDetail {
        const summary = toSummary(process);
        const args = typeof process.params === 'string' && process.params.trim() !== '' ? splitCommandLine(process.params) : [];
        const cpuTime = Number.isFinite(process.cpuu) || Number.isFinite(process.cpus)
                ? (Number.isFinite(process.cpuu) ? process.cpuu : 0) + (Number.isFinite(process.cpus) ? process.cpus : 0)
                : undefined;

        return {
                ...summary,
                path: process.path || undefined,
                arguments: args,
                priority: Number.isFinite(process.priority) ? process.priority : undefined,
                nice: Number.isFinite(process.nice) ? process.nice : undefined,
                cpuTime,
                cwd: undefined
        } satisfies ProcessDetail;
}

export async function listProcesses(): Promise<si.Systeminformation.ProcessesProcessData[]> {
        const { list } = await si.processes();
        return list;
}

export async function findProcess(pid: number): Promise<si.Systeminformation.ProcessesProcessData | undefined> {
        const processes = await listProcesses();
        return processes.find((item) => item.pid === pid);
}
