export type WorkspaceLogStatus =
        | 'draft'
        | 'queued'
        | 'pending'
        | 'in-progress'
        | 'complete'
        | 'failed';

export interface WorkspaceLogEntry {
        id: string;
        timestamp: string;
        action: string;
        status: WorkspaceLogStatus;
        detail?: string;
}
