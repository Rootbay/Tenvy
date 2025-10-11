export type ClientChatParticipant = 'operator' | 'client';

export interface ClientChatAliasConfiguration {
        operator?: string;
        client?: string;
}

export interface ClientChatFeatureFlags {
        unstoppable: boolean;
        allowNotifications?: boolean;
        allowFileTransfers?: boolean;
}

export interface ClientChatMessage {
        id: string;
        sessionId: string;
        sender: ClientChatParticipant;
        alias?: string;
        body: string;
        timestamp: string;
}

export interface ClientChatSessionState {
        sessionId: string;
        active: boolean;
        unstoppable: boolean;
        operatorAlias: string;
        clientAlias: string;
        startedAt: string;
        stoppedAt?: string;
        features: ClientChatFeatureFlags;
        messages: ClientChatMessage[];
}

export type ClientChatCommandAction = 'start' | 'stop' | 'send-message' | 'configure';

export interface ClientChatCommandMessage {
        id?: string;
        body: string;
        timestamp?: string;
        alias?: string;
}

export interface ClientChatCommandPayload {
        action: ClientChatCommandAction;
        sessionId?: string;
        message?: ClientChatCommandMessage;
        aliases?: ClientChatAliasConfiguration;
        features?: Partial<ClientChatFeatureFlags>;
}

export interface ClientChatMessageEnvelope {
        sessionId: string;
        message: {
                id: string;
                body: string;
                timestamp: string;
                alias?: string;
        };
}

export interface ClientChatStateResponse {
        session: ClientChatSessionState | null;
}

export interface ClientChatMessageResponse {
        accepted: boolean;
        session: ClientChatSessionState;
        message: ClientChatMessage;
}
