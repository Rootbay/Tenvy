export type FileSystemEntryType = 'file' | 'directory' | 'symlink' | 'other';

export interface FileSystemEntry {
        name: string;
        path: string;
        type: FileSystemEntryType;
        size: number | null;
        modifiedAt: string;
        isHidden: boolean;
}

export interface DirectoryListing {
        type: 'directory';
        root: string;
        path: string;
        parent: string | null;
        entries: FileSystemEntry[];
}

export type FileEncoding = 'utf-8' | 'base64';

export interface FileContent {
        type: 'file';
        root: string;
        path: string;
        name: string;
        size: number;
        modifiedAt: string;
        encoding: FileEncoding;
        content: string;
}

export type FileManagerResource = DirectoryListing | FileContent;

export interface FileOperationResponse {
        success: boolean;
        message?: string;
        entry?: FileSystemEntry;
        path?: string;
}
