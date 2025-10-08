export type NoteVisibility = 'local' | 'shared';

export interface NoteEnvelope {
        id: string;
        visibility: NoteVisibility;
        updatedAt: string;
        version: number;
        ciphertext: string;
        nonce: string;
        digest: string;
}

export interface NoteSyncRequest {
        notes: NoteEnvelope[];
}

export interface NoteSyncResponse {
        notes: NoteEnvelope[];
}
