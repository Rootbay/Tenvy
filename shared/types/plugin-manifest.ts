import { agentModuleIds } from "../modules/index.js";

export const pluginDeliveryModes = ["manual", "automatic"] as const;
export type PluginDeliveryMode = (typeof pluginDeliveryModes)[number];

export const pluginSignatureTypes = ["none", "sha256", "ed25519"] as const;
export type PluginSignatureType = (typeof pluginSignatureTypes)[number];

export const pluginPlatforms = ["windows", "linux", "macos"] as const;
export type PluginPlatform = (typeof pluginPlatforms)[number];

export const pluginArchitectures = ["x86_64", "arm64"] as const;
export type PluginArchitecture = (typeof pluginArchitectures)[number];

export const pluginInstallStatuses = [
  "pending",
  "installing",
  "installed",
  "failed",
  "blocked",
] as const;
export type PluginInstallStatus = (typeof pluginInstallStatuses)[number];

export const pluginApprovalStatuses = [
  "pending",
  "approved",
  "rejected",
] as const;
export type PluginApprovalStatus = (typeof pluginApprovalStatuses)[number];

const SEMVER_PATTERN =
  /^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z-.]+)?(?:\+[0-9A-Za-z-.]+)?$/;

const hasModule = (moduleId: string | undefined | null): boolean =>
  moduleId != null && agentModuleIds.has(moduleId.trim());

export interface PluginCapability {
  name: string;
  module: string;
  description?: string;
}

export interface PluginRequirements {
  minAgentVersion?: string;
  maxAgentVersion?: string;
  minClientVersion?: string;
  platforms?: PluginPlatform[];
  architectures?: PluginArchitecture[];
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

export interface PluginInstallationTelemetry {
  pluginId: string;
  version: string;
  status: PluginInstallStatus;
  hash?: string;
  lastDeployedAt?: string | null;
  lastCheckedAt?: string | null;
  error?: string;
}

export interface PluginSyncPayload {
  installations: PluginInstallationTelemetry[];
}

const ensureArray = <T>(value: ReadonlyArray<T> | undefined | null): T[] =>
  Array.isArray(value) ? [...value] : [];

const isEmpty = (value: string | undefined | null): boolean =>
  value == null || value.trim() === "";

const validateSemver = (value: string | undefined | null): boolean =>
  !value || SEMVER_PATTERN.test(value.trim());

export function validatePluginManifest(manifest: PluginManifest): string[] {
  const problems: string[] = [];

  if (isEmpty(manifest.id)) problems.push("missing id");
  if (isEmpty(manifest.name)) problems.push("missing name");
  if (isEmpty(manifest.version)) {
    problems.push("missing version");
  } else if (!validateSemver(manifest.version)) {
    problems.push(`invalid semantic version: ${manifest.version}`);
  }
  if (isEmpty(manifest.entry)) problems.push("missing entry");

  if (!manifest.package || isEmpty(manifest.package.artifact)) {
    problems.push("missing package artifact");
  }

  const mode = manifest.distribution?.defaultMode;
  if (!pluginDeliveryModes.includes(mode as PluginDeliveryMode)) {
    problems.push(`unsupported delivery mode: ${mode ?? "undefined"}`);
  }

  const signature = manifest.distribution?.signature;
  if (!signature) {
    problems.push("missing signature");
  } else if (!pluginSignatureTypes.includes(signature.type)) {
    problems.push(`unsupported signature type: ${signature.type}`);
  } else {
    if (signature.type === "sha256" && isEmpty(signature.hash)) {
      problems.push("sha256 signature requires hash");
    }
    if (signature.type === "ed25519") {
      if (isEmpty(signature.hash)) {
        problems.push("ed25519 signature requires hash");
      }
      if (isEmpty(signature.publicKey)) {
        problems.push("ed25519 signature requires publicKey");
      }
    }
  }

  if (manifest.distribution?.signature?.type !== "none") {
    if (isEmpty(manifest.package?.hash)) {
      problems.push("signed packages must include a hash");
    }
  }

  ensureArray(manifest.requirements?.requiredModules).forEach(
    (moduleId, index) => {
      if (isEmpty(moduleId)) {
        problems.push(`required module ${index} is empty`);
        return;
      }
      if (!hasModule(moduleId)) {
        problems.push(`required module ${moduleId} is not registered`);
      }
    },
  );

  ensureArray(manifest.capabilities).forEach((capability, index) => {
    if (isEmpty(capability.name)) {
      problems.push(`capability ${index} is missing name`);
    }
    if (isEmpty(capability.module)) {
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

  ensureArray(manifest.requirements?.platforms).forEach((platform, index) => {
    if (!pluginPlatforms.includes(platform as PluginPlatform)) {
      problems.push(`unsupported platform ${platform ?? index}`);
    }
  });

  ensureArray(manifest.requirements?.architectures).forEach(
    (architecture, index) => {
      if (!pluginArchitectures.includes(architecture as PluginArchitecture)) {
        problems.push(`unsupported architecture ${architecture ?? index}`);
      }
    },
  );

  if (!validateSemver(manifest.requirements?.minAgentVersion)) {
    problems.push(
      `invalid minAgentVersion: ${manifest.requirements?.minAgentVersion}`,
    );
  }
  if (!validateSemver(manifest.requirements?.maxAgentVersion)) {
    problems.push(
      `invalid maxAgentVersion: ${manifest.requirements?.maxAgentVersion}`,
    );
  }
  if (!validateSemver(manifest.requirements?.minClientVersion)) {
    problems.push(
      `invalid minClientVersion: ${manifest.requirements?.minClientVersion}`,
    );
  }

  return problems;
}
