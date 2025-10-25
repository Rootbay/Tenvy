import { agentModuleIds } from "../modules/index.js";
import nacl from "tweetnacl";

export const pluginDeliveryModes = ["manual", "automatic"] as const;
export type PluginDeliveryMode = (typeof pluginDeliveryModes)[number];

export const pluginSignatureTypes = ["sha256", "ed25519"] as const;
export type PluginSignatureType = (typeof pluginSignatureTypes)[number];

export const pluginSignatureStatuses = [
  "trusted",
  "untrusted",
  "unsigned",
  "invalid",
] as const;
export type PluginSignatureStatus = (typeof pluginSignatureStatuses)[number];

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
  signature?: string;
  signedAt?: string;
  signer?: string;
  certificateChain?: string[];
}

export interface PluginDistribution {
  defaultMode: PluginDeliveryMode;
  autoUpdate: boolean;
  signature: PluginSignature;
}

export interface PluginLicenseInfo {
  spdxId: string;
  name?: string;
  url?: string;
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
  repositoryUrl: string;
  license: PluginLicenseInfo;
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
  manifests?: AgentPluginManifestState;
}

export interface AgentPluginManifestState {
  version?: string;
  digests?: Record<string, string>;
}

export interface PluginManifestDescriptor {
  pluginId: string;
  version: string;
  manifestDigest: string;
  artifactHash?: string | null;
  artifactSizeBytes?: number | null;
  approvedAt?: string | null;
  distribution: {
    defaultMode: PluginDeliveryMode;
    autoUpdate: boolean;
  };
}

export interface PluginManifestSnapshot {
  version: string;
  manifests: PluginManifestDescriptor[];
}

export interface PluginManifestDelta {
  version: string;
  updated: PluginManifestDescriptor[];
  removed: string[];
}

export type PluginSignatureVerificationErrorCode =
  | "UNSIGNED"
  | "HASH_MISMATCH"
  | "HASH_NOT_ALLOWED"
  | "UNTRUSTED_SIGNER"
  | "INVALID_SIGNATURE"
  | "INVALID_TIMESTAMP"
  | "SIGNATURE_EXPIRED"
  | "CERTIFICATE_INVALID"
  | "INVALID_PUBLIC_KEY";

export class PluginSignatureVerificationError extends Error {
  readonly code: PluginSignatureVerificationErrorCode;

  constructor(message: string, code: PluginSignatureVerificationErrorCode) {
    super(message);
    this.name = "PluginSignatureVerificationError";
    this.code = code;
  }
}

export interface PluginSignatureVerificationOptions {
  sha256AllowList?: Iterable<string>;
  ed25519PublicKeys?: Record<string, Uint8Array>;
  resolveEd25519PublicKey?: (
    keyId: string,
  ) => Uint8Array | undefined | Promise<Uint8Array | undefined>;
  certificateValidator?: (chain: readonly string[]) => void | Promise<void>;
  maxSignatureAgeMs?: number;
  now?: () => Date;
}

export interface PluginSignatureVerificationResult {
  trusted: boolean;
  signatureType: PluginSignatureType;
  hash?: string;
  signer?: string | null;
  signedAt?: Date | null;
  publicKey?: string | null;
  certificateChain?: string[];
}

export interface PluginSignatureVerificationSummary
  extends PluginSignatureVerificationResult {
  status: PluginSignatureStatus;
  checkedAt: Date;
  error?: string | null;
  errorCode?: PluginSignatureVerificationErrorCode;
}

const ensureArray = <T>(value: ReadonlyArray<T> | undefined | null): T[] =>
  Array.isArray(value) ? [...value] : [];

const isEmpty = (value: string | undefined | null): boolean =>
  value == null || value.trim() === "";

const validateSemver = (value: string | undefined | null): boolean =>
  !value || SEMVER_PATTERN.test(value.trim());

const textEncoder = new TextEncoder();

const normalizeHex = (value: string | undefined | null): string =>
  value?.trim().toLowerCase() ?? "";

const hexToBytes = (value: string): Uint8Array => {
  const trimmed = value.trim();
  if (trimmed.length === 0 || trimmed.length % 2 !== 0) {
    throw new PluginSignatureVerificationError(
      "signature must be a non-empty even-length hex string",
      "INVALID_SIGNATURE",
    );
  }

  const bytes = new Uint8Array(trimmed.length / 2);
  for (let i = 0; i < trimmed.length; i += 2) {
    const byte = Number.parseInt(trimmed.slice(i, i + 2), 16);
    if (Number.isNaN(byte)) {
      throw new PluginSignatureVerificationError(
        "signature contains non-hex characters",
        "INVALID_SIGNATURE",
      );
    }
    bytes[i / 2] = byte;
  }
  return bytes;
};

const ensureGitHubRepository = (url: string | undefined | null): boolean => {
  if (isEmpty(url)) return false;
  try {
    const parsed = new URL(url!);
    if (parsed.protocol !== "https:") return false;
    if (parsed.hostname !== "github.com") return false;
    const segments = parsed.pathname.split("/").filter(Boolean);
    return segments.length >= 2;
  } catch {
    return false;
  }
};

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

  if (!ensureGitHubRepository(manifest.repositoryUrl)) {
    problems.push("repositoryUrl must reference a GitHub repository");
  }

  if (!manifest.license || isEmpty(manifest.license.spdxId)) {
    problems.push("license requires spdxId");
  }
  if (manifest.license?.url && isEmpty(manifest.license.url)) {
    problems.push("license url cannot be empty string");
  }

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
    if (isEmpty(signature.signature)) {
      problems.push("signed manifests must provide signature value");
    }
  }

  if (isEmpty(manifest.package?.hash)) {
    problems.push("signed packages must include a hash");
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

const resolvePublicKey = async (
  keyId: string,
  options: PluginSignatureVerificationOptions,
): Promise<Uint8Array | undefined> => {
  const trimmed = keyId.trim();
  if (!trimmed) return undefined;

  if (options.resolveEd25519PublicKey) {
    const key = await options.resolveEd25519PublicKey(trimmed);
    if (key) return key.slice();
  }

  const staticKey = options.ed25519PublicKeys?.[trimmed];
  return staticKey ? staticKey.slice() : undefined;
};

const ensureValidSignedAt = (
  signedAt: string | undefined,
  options: PluginSignatureVerificationOptions,
): Date | null => {
  if (!signedAt?.trim()) {
    if (options.maxSignatureAgeMs && options.maxSignatureAgeMs > 0) {
      throw new PluginSignatureVerificationError(
        "signedAt is required when enforcing signature age",
        "INVALID_TIMESTAMP",
      );
    }
    return null;
  }

  const parsed = new Date(signedAt);
  if (Number.isNaN(parsed.valueOf())) {
    throw new PluginSignatureVerificationError(
      "signedAt must be an RFC3339 timestamp",
      "INVALID_TIMESTAMP",
    );
  }

  const now = options.now?.() ?? new Date();
  const maxAge = options.maxSignatureAgeMs ?? 0;
  if (maxAge > 0) {
    const ageMs = now.valueOf() - parsed.valueOf();
    if (ageMs < 0) {
      throw new PluginSignatureVerificationError(
        "signature timestamp is in the future",
        "INVALID_TIMESTAMP",
      );
    }
    if (ageMs > maxAge) {
      throw new PluginSignatureVerificationError(
        "signature has expired",
        "SIGNATURE_EXPIRED",
      );
    }
  }

  return parsed;
};

const ensureHashMatches = (manifest: PluginManifest): string => {
  const manifestHash = normalizeHex(manifest.package?.hash);
  const signatureHash = normalizeHex(manifest.distribution?.signature?.hash);
  if (!manifestHash || !signatureHash || manifestHash !== signatureHash) {
    throw new PluginSignatureVerificationError(
      "plugin hash does not match manifest signature hash",
      "HASH_MISMATCH",
    );
  }
  return signatureHash;
};

export const verifyPluginSignature = async (
  manifest: PluginManifest,
  options: PluginSignatureVerificationOptions = {},
): Promise<PluginSignatureVerificationResult> => {
  const signature = manifest.distribution?.signature;
  if (!signature) {
    throw new PluginSignatureVerificationError(
      "plugin manifest is unsigned",
      "UNSIGNED",
    );
  }

  const signedAt = ensureValidSignedAt(signature.signedAt, options);
  const hash = ensureHashMatches(manifest);

  if (signature.type === "sha256") {
    const allowList = options.sha256AllowList
      ? new Set(
          Array.from(options.sha256AllowList, (value) => normalizeHex(value)),
        )
      : null;

    if (allowList && allowList.size > 0 && !allowList.has(hash)) {
      throw new PluginSignatureVerificationError(
        "hash is not in the allowed set",
        "HASH_NOT_ALLOWED",
      );
    }

    return {
      trusted: allowList ? allowList.size > 0 : false,
      signatureType: "sha256",
      hash,
      signer: signature.signer ?? null,
      signedAt,
      certificateChain: signature.certificateChain
        ? [...signature.certificateChain]
        : undefined,
    };
  }

  if (signature.type !== "ed25519") {
    throw new PluginSignatureVerificationError(
      `unsupported signature type: ${signature.type}`,
      "INVALID_SIGNATURE",
    );
  }

  if (!signature.publicKey?.trim()) {
    throw new PluginSignatureVerificationError(
      "ed25519 signature requires publicKey",
      "UNTRUSTED_SIGNER",
    );
  }

  const publicKey = await resolvePublicKey(signature.publicKey, options);
  if (!publicKey) {
    throw new PluginSignatureVerificationError(
      "ed25519 signer is not trusted",
      "UNTRUSTED_SIGNER",
    );
  }
  if (publicKey.length !== nacl.sign.publicKeyLength) {
    throw new PluginSignatureVerificationError(
      "ed25519 public key has invalid length",
      "INVALID_PUBLIC_KEY",
    );
  }

  const signatureBytes = hexToBytes(signature.signature ?? "");
  if (signatureBytes.length !== nacl.sign.signatureLength) {
    throw new PluginSignatureVerificationError(
      "ed25519 signature has invalid length",
      "INVALID_SIGNATURE",
    );
  }

  const message = textEncoder.encode(hash);
  if (!nacl.sign.detached.verify(message, signatureBytes, publicKey)) {
    throw new PluginSignatureVerificationError(
      "ed25519 signature verification failed",
      "INVALID_SIGNATURE",
    );
  }

  if (options.certificateValidator && signature.certificateChain?.length) {
    try {
      await options.certificateValidator([...signature.certificateChain]);
    } catch (error) {
      throw new PluginSignatureVerificationError(
        (error as Error).message,
        "CERTIFICATE_INVALID",
      );
    }
  }

  return {
    trusted: true,
    signatureType: "ed25519",
    hash,
    signer: signature.signer ?? null,
    signedAt,
    publicKey: signature.publicKey,
    certificateChain: signature.certificateChain
      ? [...signature.certificateChain]
      : undefined,
  };
};
