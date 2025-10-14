export type ToolActivationAction =
        | 'open'
        | 'close'
        | `event:${string}`
        | `operation:${string}`
        | `log:${string}`
        | `lifecycle:${string}`;

export interface ToolActivationCommandPayload {
        toolId: string;
        action: ToolActivationAction;
        initiatedBy?: string;
        timestamp?: string;
        metadata?: Record<string, unknown>;
}
