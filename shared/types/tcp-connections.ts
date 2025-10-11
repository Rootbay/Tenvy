export type TcpConnectionFamily = 'IPv4' | 'IPv6';

export type TcpConnectionState =
        | 'LISTENING'
        | 'ESTABLISHED'
        | 'CLOSE_WAIT'
        | 'SYN_SENT'
        | 'SYN_RECEIVED'
        | 'FIN_WAIT_1'
        | 'FIN_WAIT_2'
        | 'TIME_WAIT'
        | 'LAST_ACK'
        | 'CLOSING'
        | 'BOUND'
        | 'CLOSED'
        | 'UNKNOWN';

export interface TcpConnectionEndpoint {
        address: string;
        port: number;
        family: TcpConnectionFamily;
        host?: string;
        label?: string;
}

export interface TcpConnectionProcess {
        pid?: number;
        name?: string;
        executable?: string;
        commandLine?: string;
        username?: string;
}

export interface TcpConnectionEntry {
        id: string;
        local: TcpConnectionEndpoint;
        remote?: TcpConnectionEndpoint | null;
        state: TcpConnectionState;
        listening: boolean;
        process?: TcpConnectionProcess;
        family: TcpConnectionFamily;
}

export interface TcpConnectionQuery {
        localFilter?: string;
        remoteFilter?: string;
        state?: TcpConnectionState | 'all';
        includeIpv6?: boolean;
        resolveDns?: boolean;
        limit?: number;
}

export interface TcpConnectionSnapshot {
        capturedAt: string;
        total: number;
        truncated?: boolean;
        connections: TcpConnectionEntry[];
        requestId?: string;
        query?: TcpConnectionQuery;
}

export interface TcpConnectionSnapshotEnvelope {
        requestId?: string;
        snapshot: TcpConnectionSnapshot;
}

export interface TcpConnectionsCommandPayload {
        action: 'enumerate';
        requestId: string;
        query?: TcpConnectionQuery;
}
