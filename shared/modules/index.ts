import moduleDefinitions from '../pluginmanifest/definitions.json';

export interface AgentModuleCapability {
  id: string;
  name: string;
  description: string;
}

export interface AgentModuleTelemetry {
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
  telemetry?: AgentModuleTelemetry[];
}

type ModuleDefinitions = {
  modules: AgentModuleDefinition[];
};

const moduleData = moduleDefinitions as ModuleDefinitions;

export const agentModules: AgentModuleDefinition[] = moduleData.modules;

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

type AgentModuleTelemetryRecord = AgentModuleTelemetry & {
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

const telemetryEntries: [string, AgentModuleTelemetryRecord][] = [];

for (const module of agentModules) {
  const telemetry = Array.isArray(module.telemetry)
    ? module.telemetry
    : [];
  for (const descriptor of telemetry) {
    telemetryEntries.push([
      descriptor.id,
      {
        ...descriptor,
        moduleId: module.id,
        moduleTitle: module.title,
      },
    ]);
  }
}

export const agentModuleTelemetryIndex: ReadonlyMap<
  string,
  AgentModuleTelemetryRecord
> = new Map(telemetryEntries);

export const agentModuleTelemetryIds: ReadonlySet<string> = new Set(
  telemetryEntries.map(([id]) => id),
);
