export type RemoteDesktopQuality = "auto" | "high" | "medium" | "low";

export interface RemoteDesktopSettings {
  quality: RemoteDesktopQuality;
  monitor: number;
  mouse: boolean;
  keyboard: boolean;
}

export type RemoteDesktopCommandAction = "start" | "stop" | "configure";

export interface RemoteDesktopCommandPayload {
  action: RemoteDesktopCommandAction;
  sessionId?: string;
  settings?: Partial<RemoteDesktopSettings>;
}

export interface RemoteDesktopFrameMetrics {
  fps?: number;
  bandwidthKbps?: number;
  cpuPercent?: number;
  gpuPercent?: number;
}

export interface RemoteDesktopCursorState {
  x: number;
  y: number;
  visible: boolean;
}

export interface RemoteDesktopDeltaRect {
  x: number;
  y: number;
  width: number;
  height: number;
  encoding: "png";
  data: string;
}

export interface RemoteDesktopFramePacket {
  sessionId: string;
  sequence: number;
  timestamp: string;
  width: number;
  height: number;
  keyFrame: boolean;
  encoding: "png";
  image?: string;
  deltas?: RemoteDesktopDeltaRect[];
  monitors?: RemoteDesktopMonitor[];
  cursor?: RemoteDesktopCursorState;
  metrics?: RemoteDesktopFrameMetrics;
}

export interface RemoteDesktopMonitor {
  id: number;
  label: string;
  width: number;
  height: number;
}

export interface RemoteDesktopSessionState {
  sessionId: string;
  agentId: string;
  active: boolean;
  createdAt: string;
  lastUpdatedAt?: string;
  lastSequence?: number;
  settings: RemoteDesktopSettings;
  monitors: RemoteDesktopMonitor[];
  metrics?: RemoteDesktopFrameMetrics;
}

export interface RemoteDesktopSessionResponse {
  session: RemoteDesktopSessionState | null;
}

export interface RemoteDesktopStreamSessionMessage {
  session: RemoteDesktopSessionState;
}

export interface RemoteDesktopStreamFrameMessage {
  frame: RemoteDesktopFramePacket;
}

export interface RemoteDesktopStreamEndMessage {
  reason?: string;
}
