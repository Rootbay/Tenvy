import { agentModuleIds } from "../modules/index.js";

export type PluginDeliveryMode = "manual" | "automatic";

export type PluginSignatureType = "none" | "sha256" | "ed25519";

export interface PluginCapability {
  name: string;
  module: string;
  description?: string;
}

export interface PluginRequirements {
  minAgentVersion?: string;
  maxAgentVersion?: string;
  minClientVersion?: string;
  platforms?: string[];
  architectures?: string[];
  requiredModules?: string[];
}

export interface PluginSignature {
  type: PluginSignatureType;
  hash?: string;
  publicKey?: string;
}

export interface PluginDistribution {
  defaultMode: PluginDeliveryMode;
  autoUpdate: boolean;
  signature: PluginSignature;
}

export interface PluginPackageDescriptor {
  artifact: string;
  sizeBytes?: number;
  hash?: string;
}

export interface PluginManifest {
  id: string;
  name: string;
  version: string;
  description?: string;
  entry: string;
  author?: string;
  homepage?: string;
  license?: string;
  categories?: string[];
  capabilities?: PluginCapability[];
  requirements: PluginRequirements;
  distribution: PluginDistribution;
  package: PluginPackageDescriptor;
}

export function validatePluginManifest(manifest: PluginManifest): string[] {
  const problems: string[] = [];

  const hasModule = (moduleId: string | undefined | null): boolean =>
    moduleId != null && agentModuleIds.has(moduleId.trim());

  if (!manifest.id?.trim()) problems.push("missing id");
  if (!manifest.name?.trim()) problems.push("missing name");
  if (!manifest.version?.trim()) problems.push("missing version");
  if (!manifest.entry?.trim()) problems.push("missing entry");
  if (!manifest.package?.artifact?.trim())
    problems.push("missing package artifact");

  const mode = manifest.distribution?.defaultMode;
  if (mode !== "manual" && mode !== "automatic") {
    problems.push(`unsupported delivery mode: ${mode ?? "undefined"}`);
  }

  const signature = manifest.distribution?.signature;
  if (signature) {
    switch (signature.type) {
      case "none":
        break;
      case "sha256":
        if (!signature.hash?.trim()) {
          problems.push("sha256 signature requires hash");
        }
        break;
      case "ed25519":
        if (!signature.hash?.trim()) {
          problems.push("ed25519 signature requires hash");
        }
        if (!signature.publicKey?.trim()) {
          problems.push("ed25519 signature requires publicKey");
        }
        break;
      default:
        problems.push(`unsupported signature type: ${signature.type}`);
    }
  } else {
    problems.push("missing signature");
  }

  manifest.requirements?.requiredModules?.forEach((moduleId, index) => {
    if (!moduleId?.trim()) {
      problems.push(`required module ${index} is empty`);
      return;
    }
    if (!hasModule(moduleId)) {
      problems.push(`required module ${moduleId} is not registered`);
    }
  });

  manifest.capabilities?.forEach((capability, index) => {
    if (!capability.name?.trim()) {
      problems.push(`capability ${index} is missing name`);
    }
    if (!capability.module?.trim()) {
      problems.push(
        `capability ${capability.name ?? index} is missing module reference`,
      );
      return;
    }
    if (!hasModule(capability.module)) {
      problems.push(
        `capability ${capability.name ?? index} references unknown module ${capability.module}`,
      );
    }
  });

  return problems;
}
