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

export type TaskManagerOperation = 'list' | 'detail' | 'start' | 'action';

export interface TaskManagerListCommandRequest {
        operation: 'list';
}

export interface TaskManagerDetailCommandRequest {
        operation: 'detail';
        pid: number;
}

export interface TaskManagerStartCommandRequest {
        operation: 'start';
        payload: StartProcessRequest;
}

export interface TaskManagerActionCommandRequest {
        operation: 'action';
        pid: number;
        action: ProcessAction;
}

export type TaskManagerCommandRequest =
        | TaskManagerListCommandRequest
        | TaskManagerDetailCommandRequest
        | TaskManagerStartCommandRequest
        | TaskManagerActionCommandRequest;

export interface TaskManagerCommandPayload {
        request: TaskManagerCommandRequest;
}

export type TaskManagerCommandResponse =
        | { operation: 'list'; status: 'ok'; result: ProcessListResponse }
        | { operation: 'detail'; status: 'ok'; result: ProcessDetail }
        | { operation: 'start'; status: 'ok'; result: StartProcessResponse }
        | { operation: 'action'; status: 'ok'; result: ProcessActionResponse }
        | { operation: TaskManagerOperation; status: 'error'; error: string; code?: string; details?: unknown };
