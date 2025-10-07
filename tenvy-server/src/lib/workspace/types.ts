export type WorkspaceLogStatus = 'draft' | 'queued' | 'in-progress' | 'complete';

export interface WorkspaceLogEntry {
        id: string;
        timestamp: string;
        action: string;
        status: WorkspaceLogStatus;
        detail?: string;
}
