export type WebcamQuality = "max" | "high" | "medium" | "low";

export interface WebcamResolution {
  width: number;
  height: number;
}

export interface WebcamZoomRange {
  min: number;
  max: number;
  step: number;
}

export interface WebcamDeviceCapabilities {
  resolutions?: WebcamResolution[];
  frameRates?: number[];
  zoom?: WebcamZoomRange;
  facingMode?: "user" | "environment" | "external";
}

export interface WebcamDevice {
  id: string;
  label: string;
  capabilities?: WebcamDeviceCapabilities;
}

export interface WebcamDeviceInventory {
  devices: WebcamDevice[];
  capturedAt: string;
  requestId?: string;
  warning?: string;
}

export interface WebcamDeviceInventoryState {
  inventory: WebcamDeviceInventory | null;
  pending: boolean;
}

export interface WebcamStreamSettings {
  quality?: WebcamQuality;
  width?: number;
  height?: number;
  frameRate?: number;
  zoom?: number;
}

export interface WebcamNegotiationOffer {
  transport: "webrtc" | "http";
  offer?: string;
  iceServers?: string[];
  dataChannel?: string;
}

export interface WebcamNegotiationAnswer {
  answer?: string;
  iceServers?: string[];
  dataChannel?: string;
}

export interface WebcamNegotiationState {
  offer?: WebcamNegotiationOffer;
  answer?: WebcamNegotiationAnswer;
}

export type WebcamCommandAction =
  | "enumerate"
  | "start"
  | "stop"
  | "update";

export interface WebcamCommandPayload {
  action: WebcamCommandAction;
  requestId?: string;
  sessionId?: string;
  deviceId?: string;
  settings?: WebcamStreamSettings;
  negotiation?: WebcamNegotiationState;
}

export interface WebcamSessionState {
  sessionId: string;
  agentId: string;
  deviceId?: string;
  createdAt: string;
  updatedAt: string;
  status: "pending" | "active" | "stopped" | "error";
  settings?: WebcamStreamSettings;
  negotiation?: WebcamNegotiationState;
  error?: string;
}
