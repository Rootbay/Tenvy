export type AppVncQuality = "lossless" | "balanced" | "bandwidth";

export type AppVncPlatform = "windows" | "linux" | "macos";

export interface AppVncVirtualizationHints {
  profileSeeds?: Partial<Record<AppVncPlatform, string>>;
  dataRoots?: Partial<Record<AppVncPlatform, string>>;
  environment?: Partial<Record<AppVncPlatform, Record<string, string>>>;
}

export interface AppVncVirtualizationPlan {
  platform?: AppVncPlatform;
  profileSeed?: string;
  dataRoot?: string;
  environment?: Record<string, string>;
}

export interface AppVncApplicationDescriptor {
  id: string;
  name: string;
  summary: string;
  category: string;
  platforms: AppVncPlatform[];
  windowTitleHint?: string;
  executable?: Partial<Record<AppVncPlatform, string>>;
  virtualization?: AppVncVirtualizationHints;
}

export interface AppVncSessionMetadata {
  appId?: string;
  windowTitle?: string;
  processId?: number;
  virtualDisplay?: boolean;
}

export interface AppVncSessionSettings {
  monitor: string;
  quality: AppVncQuality;
  captureCursor: boolean;
  clipboardSync: boolean;
  blockLocalInput: boolean;
  heartbeatInterval: number;
  appId?: string;
  windowTitle?: string;
}

export interface AppVncSessionSettingsPatch {
  monitor?: string;
  quality?: AppVncQuality;
  captureCursor?: boolean;
  clipboardSync?: boolean;
  blockLocalInput?: boolean;
  heartbeatInterval?: number;
  appId?: string;
  windowTitle?: string;
}

export interface AppVncCursorState {
  x: number;
  y: number;
  visible: boolean;
}

export type AppVncPointerButton = "left" | "middle" | "right";

export interface AppVncInputEventBase {
  capturedAt: number;
}

export type AppVncInputEvent =
  | (AppVncInputEventBase & {
      type: "pointer-move";
      x: number;
      y: number;
      normalized?: boolean;
    })
  | (AppVncInputEventBase & {
      type: "pointer-button";
      button: AppVncPointerButton;
      pressed: boolean;
    })
  | (AppVncInputEventBase & {
      type: "pointer-scroll";
      deltaX: number;
      deltaY: number;
      deltaMode?: number;
    })
  | (AppVncInputEventBase & {
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
    });

export interface AppVncInputBurst {
  sessionId: string;
  events: AppVncInputEvent[];
  sequence?: number;
}

export interface AppVncFramePacket {
  sessionId: string;
  sequence: number;
  timestamp: string;
  width: number;
  height: number;
  encoding: "png" | "jpeg";
  image: string;
  cursor?: AppVncCursorState;
  metadata?: AppVncSessionMetadata;
}

export interface AppVncSessionState {
  sessionId: string;
  agentId: string;
  active: boolean;
  createdAt: string;
  lastUpdatedAt?: string;
  lastSequence?: number;
  settings: AppVncSessionSettings;
  metadata?: AppVncSessionMetadata;
  cursor?: AppVncCursorState;
}

export type AppVncCommandAction = "start" | "stop" | "configure" | "input" | "heartbeat";

export interface AppVncCommandPayload {
  action: AppVncCommandAction;
  sessionId?: string;
  settings?: AppVncSessionSettingsPatch;
  events?: AppVncInputEvent[];
  application?: AppVncApplicationDescriptor;
  virtualization?: AppVncVirtualizationPlan;
}

export interface AppVncSessionResponse {
  session: AppVncSessionState | null;
}

export interface AppVncFrameIngestResponse {
  accepted: boolean;
}
