export interface AgentModuleCapability {
  id: string;
  name: string;
  description: string;
}

export interface AgentModuleDefinition {
  id: string;
  title: string;
  description: string;
  commands: string[];
  capabilities: AgentModuleCapability[];
}

export const agentModules: AgentModuleDefinition[] = [
  {
    id: "remote-desktop",
    title: "Remote Desktop",
    description:
      "Interactive remote desktop streaming and operator input relay.",
    commands: ["remote-desktop"],
    capabilities: [
      {
        id: "remote-desktop.stream",
        name: "Desktop streaming",
        description:
          "Stream high-fidelity desktop frames to the controller UI.",
      },
      {
        id: "remote-desktop.input",
        name: "Input relay",
        description:
          "Relay keyboard and pointer events back to the remote host.",
      },
    ],
  },
  {
    id: "audio-control",
    title: "Audio Control",
    description:
      "Capture and inject audio for synchronized operator coordination.",
    commands: ["audio-control"],
    capabilities: [
      {
        id: "audio.capture",
        name: "Audio capture",
        description:
          "Capture remote system audio for monitoring and recording.",
      },
      {
        id: "audio.inject",
        name: "Audio injection",
        description:
          "Inject operator-provided audio streams into the remote session.",
      },
    ],
  },
  {
    id: "clipboard",
    title: "Clipboard Manager",
    description:
      "Synchronize clipboard contents between the operator and remote workstation.",
    commands: ["clipboard"],
    capabilities: [
      {
        id: "clipboard.capture",
        name: "Clipboard capture",
        description:
          "Capture clipboard changes emitted by the remote workstation.",
      },
      {
        id: "clipboard.push",
        name: "Clipboard push",
        description: "Push operator clipboard payloads to the remote host.",
      },
    ],
  },
  {
    id: "recovery",
    title: "Recovery Operations",
    description: "Stage, track, and collect recovery payloads across modules.",
    commands: ["recovery"],
    capabilities: [
      {
        id: "recovery.queue",
        name: "Recovery queue",
        description:
          "Queue recovery jobs for background execution and monitoring.",
      },
      {
        id: "recovery.collect",
        name: "Artifact collection",
        description:
          "Collect artifacts staged by upstream modules for exfiltration.",
      },
    ],
  },
  {
    id: "system-info",
    title: "System Information",
    description:
      "Collect host metadata, hardware configuration, and telemetry snapshots.",
    commands: ["system-info"],
    capabilities: [
      {
        id: "system-info.snapshot",
        name: "System snapshot",
        description:
          "Produce structured operating system and hardware inventories.",
      },
      {
        id: "system-info.telemetry",
        name: "System telemetry",
        description:
          "Surface live telemetry metrics used by scheduling and recovery modules.",
      },
    ],
  },
  {
    id: "notes",
    title: "Incident Notes",
    description:
      "Secure local note taking synchronized with the controller vault.",
    commands: [],
    capabilities: [
      {
        id: "notes.sync",
        name: "Notes sync",
        description:
          "Synchronize local incident notes to the operator vault with delta compression.",
      },
    ],
  },
];

export const agentModuleIndex: ReadonlyMap<string, AgentModuleDefinition> =
  new Map(agentModules.map((module) => [module.id, module]));

export const agentModuleIds: ReadonlySet<string> = new Set(
  agentModules.map((module) => module.id),
);
