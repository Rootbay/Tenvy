export type ClipboardFormat =
        | 'text'
        | 'image'
        | 'files'
        | 'html'
        | 'rtf'
        | 'unknown';

export interface ClipboardTextData {
        value: string;
        encoding?: string;
        length?: number;
}

export interface ClipboardImageData {
        mimeType: string;
        data: string;
        width?: number;
        height?: number;
}

export interface ClipboardFileEntry {
        name: string;
        size?: number;
        mimeType?: string;
        path?: string;
        digest?: string;
}

export interface ClipboardContent {
        format: ClipboardFormat;
        text?: ClipboardTextData;
        image?: ClipboardImageData;
        files?: ClipboardFileEntry[];
        metadata?: Record<string, string>;
}

export interface ClipboardSnapshot {
        sequence: number;
        capturedAt: string;
        source?: string;
        content?: ClipboardContent;
}

export interface ClipboardStateEnvelope {
        requestId?: string;
        snapshot: ClipboardSnapshot;
}

export interface ClipboardTriggerCondition {
        formats?: ClipboardFormat[];
        pattern?: string;
        caseSensitive?: boolean;
}

export interface ClipboardTriggerAction {
        type: 'notify' | 'command';
        configuration?: Record<string, unknown>;
}

export interface ClipboardTrigger {
        id: string;
        label: string;
        description?: string;
        condition: ClipboardTriggerCondition;
        action: ClipboardTriggerAction;
        createdAt: string;
        updatedAt?: string;
        active: boolean;
}

export interface ClipboardTriggerMatch {
        field: string;
        value: string;
}

export interface ClipboardTriggerEvent {
        eventId: string;
        triggerId: string;
        triggerLabel: string;
        capturedAt: string;
        sequence: number;
        requestId?: string;
        matches?: ClipboardTriggerMatch[];
        content: ClipboardContent;
        action: ClipboardTriggerAction;
}

export interface ClipboardEventEnvelope {
        events: ClipboardTriggerEvent[];
}

export interface ClipboardCommandPayload {
        action: 'get' | 'set' | 'sync-triggers';
        requestId?: string;
        content?: ClipboardContent;
        triggers?: ClipboardTrigger[];
        source?: string;
        sequence?: number;
}
