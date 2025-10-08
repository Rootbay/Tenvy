export type ProcessStatus =
        | 'running'
        | 'sleeping'
        | 'stopped'
        | 'idle'
        | 'zombie'
        | 'suspended'
        | 'unknown';

export interface ProcessSummary {
        pid: number;
        ppid?: number;
        name: string;
        cpu: number;
        memory: number;
        status: ProcessStatus;
        command: string;
        user?: string;
        startedAt?: string;
}

export interface ProcessListResponse {
        processes: ProcessSummary[];
        generatedAt: string;
}

export interface ProcessDetail extends ProcessSummary {
        path?: string;
        arguments?: string[];
        cwd?: string;
        threads?: number;
        priority?: number;
        nice?: number;
        cpuTime?: number;
}

export interface StartProcessRequest {
        command: string;
        args?: string[];
        cwd?: string;
        env?: Record<string, string>;
}

export interface StartProcessResponse {
        pid: number;
        command: string;
        args: string[];
        startedAt: string;
}

export type ProcessAction = 'stop' | 'force-stop' | 'suspend' | 'resume' | 'restart';

export interface ProcessActionRequest {
        action: ProcessAction;
}

export interface ProcessActionResponse {
        pid: number;
        action: ProcessAction;
        status: 'ok';
        message?: string;
}
