export type RemoteDesktopQuality = "auto" | "high" | "medium" | "low";

export type RemoteDesktopStreamMode = "images" | "video";

export interface RemoteDesktopSettings {
  quality: RemoteDesktopQuality;
  monitor: number;
  mouse: boolean;
  keyboard: boolean;
  mode: RemoteDesktopStreamMode;
}

export type RemoteDesktopCommandAction = "start" | "stop" | "configure" | "input";

export type RemoteDesktopMouseButton = "left" | "middle" | "right";

export type RemoteDesktopInputEvent =
  | {
      type: "mouse-move";
      x: number;
      y: number;
      normalized?: boolean;
      monitor?: number;
    }
  | {
      type: "mouse-button";
      button: RemoteDesktopMouseButton;
      pressed: boolean;
      monitor?: number;
    }
  | {
      type: "mouse-scroll";
      deltaX: number;
      deltaY: number;
      deltaMode?: number;
      monitor?: number;
    }
  | {
      type: "key";
      pressed: boolean;
      key?: string;
      code?: string;
      keyCode?: number;
      repeat?: boolean;
      altKey?: boolean;
      ctrlKey?: boolean;
      shiftKey?: boolean;
      metaKey?: boolean;
    };

export interface RemoteDesktopCommandPayload {
  action: RemoteDesktopCommandAction;
  sessionId?: string;
  settings?: Partial<RemoteDesktopSettings>;
  events?: RemoteDesktopInputEvent[];
}

export interface RemoteDesktopFrameMetrics {
  fps?: number;
  bandwidthKbps?: number;
  cpuPercent?: number;
  gpuPercent?: number;
  clipQuality?: number;
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
  encoding: "png" | "jpeg";
  data: string;
}

export interface RemoteDesktopVideoFrame {
  offsetMs: number;
  width: number;
  height: number;
  encoding: "jpeg";
  data: string;
}

export interface RemoteDesktopVideoClip {
  durationMs: number;
  frames: RemoteDesktopVideoFrame[];
}

export interface RemoteDesktopFramePacket {
  sessionId: string;
  sequence: number;
  timestamp: string;
  width: number;
  height: number;
  keyFrame: boolean;
  encoding: "png" | "clip";
  image?: string;
  deltas?: RemoteDesktopDeltaRect[];
  clip?: RemoteDesktopVideoClip;
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
