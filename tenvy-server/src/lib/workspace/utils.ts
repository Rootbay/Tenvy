import type { WorkspaceLogEntry, WorkspaceLogStatus } from './types';

export function createWorkspaceLogEntry(
        action: string,
        detail?: string,
        status: WorkspaceLogStatus = 'queued'
): WorkspaceLogEntry {
        const now = new Date();
        return {
                id: `${now.getTime()}-${Math.random().toString(36).slice(2, 8)}`,
                timestamp: now.toISOString(),
                action,
                detail,
                status
        } satisfies WorkspaceLogEntry;
}

export function appendWorkspaceLog(
        entries: WorkspaceLogEntry[],
        entry: WorkspaceLogEntry,
        limit = 12
): WorkspaceLogEntry[] {
        const next = [entry, ...entries];
        return next.slice(0, Math.max(1, limit));
}

export function formatWorkspaceTimestamp(value: string): string {
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) {
                return value;
        }
        return new Intl.DateTimeFormat(undefined, {
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
        }).format(date);
}
