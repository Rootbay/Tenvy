export type FileSystemEntryType = "file" | "directory" | "symlink" | "other";

export interface FileSystemEntry {
  name: string;
  path: string;
  type: FileSystemEntryType;
  size: number | null;
  modifiedAt: string;
  isHidden: boolean;
}

export interface DirectoryListing {
  type: "directory";
  root: string;
  path: string;
  parent: string | null;
  entries: FileSystemEntry[];
}

export type FileEncoding = "utf-8" | "base64";

export interface FileContent {
  type: "file";
  root: string;
  path: string;
  name: string;
  size: number;
  modifiedAt: string;
  encoding: FileEncoding;
  content?: string;
  stream?: FileContentStream;
}

export type FileManagerResource = DirectoryListing | FileContent;

export interface FileContentStream {
  id: string;
  part: string;
  index: number;
  count: number;
  offset: number;
  length: number;
}

export interface FileOperationResponse {
  success: boolean;
  message?: string;
  entry?: FileSystemEntry;
  path?: string;
}

export type FileManagerCommandPayload =
  | {
      action: "list-directory";
      path?: string;
      requestId?: string;
      includeHidden?: boolean;
    }
  | {
      action: "read-file";
      path: string;
      requestId?: string;
      encoding?: FileEncoding;
    }
  | {
      action: "create-entry";
      directory: string;
      name: string;
      entryType: "file" | "directory";
      content?: string;
    }
  | {
      action: "rename-entry";
      path: string;
      name: string;
    }
  | {
      action: "move-entry";
      path: string;
      destination: string;
      name?: string;
    }
  | {
      action: "delete-entry";
      path: string;
    }
  | {
      action: "update-file";
      path: string;
      content: string;
      encoding?: FileEncoding;
    };
