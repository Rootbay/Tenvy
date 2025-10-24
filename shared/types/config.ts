export interface AgentPluginSignaturePolicy {
  /**
   * Optional list of SHA-256 hashes that may be installed without a public-key
   * signature. Hashes are compared case-insensitively.
   */
  sha256AllowList?: string[];
  /**
   * Mapping of signer identifiers to Ed25519 public keys (hex encoded).
   */
  ed25519PublicKeys?: Record<string, string>;
  /**
   * Maximum accepted age for signatures in milliseconds. Omit to disable the
   * expiration check.
   */
  maxSignatureAgeMs?: number;
}

export interface AgentPluginConfig {
  signaturePolicy?: AgentPluginSignaturePolicy;
}

export interface AgentConfig {
  /**
   * Base interval in milliseconds used by the agent to poll the controller for new work.
   */
  pollIntervalMs: number;
  /**
   * Maximum backoff interval in milliseconds applied when network issues occur.
   */
  maxBackoffMs: number;
  /**
   * Randomisation factor applied to poll intervals to avoid detection patterns.
   */
  jitterRatio: number;
  /**
   * Plugin specific configuration pushed from the controller.
   */
  plugins?: AgentPluginConfig;
}

export const defaultAgentConfig: AgentConfig = Object.freeze({
  pollIntervalMs: 5_000,
  maxBackoffMs: 60_000,
  jitterRatio: 0.2,
} satisfies AgentConfig);

export interface ServerAgentConfig {
  agent: AgentConfig;
}

export const defaultServerAgentConfig: ServerAgentConfig = Object.freeze({
  agent: defaultAgentConfig,
});
