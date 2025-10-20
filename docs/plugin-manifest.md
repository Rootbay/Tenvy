# Plugin Manifest & Telemetry Guide

This document describes the shared plugin manifest contract, validation rules, and the rollout telemetry lifecycle that now drives the controller UI and Go agent.

## Manifest schema

Plugin manifests live under `resources/plugin-manifests/` (configurable via `TENVY_PLUGIN_MANIFEST_DIR`). Each manifest must satisfy the schema exported from `shared/types/plugin-manifest.ts` and validated both by the Svelte controller (`validatePluginManifest`) and the Go agent (`shared/pluginmanifest`).

Key fields:

| Field | Description |
| --- | --- |
| `id` | Unique plugin identifier (lowercase slug). |
| `name`, `description`, `author`, `homepage` | Human metadata surfaced in the UI. |
| `version` | Semantic version string validated on both stacks. |
| `entry` | Entry point inside the extracted archive. |
| `capabilities` | Capability descriptors linked to agent modules. Unknown module IDs are rejected. |
| `requirements.platforms` / `.architectures` | Accept only `windows`, `linux`, `macos` and `x86_64`, `arm64`. |
| `requirements.minAgentVersion` / `maxAgentVersion` | Optional semver range enforced during telemetry ingestion. |
| `distribution.defaultMode` | `manual` or `automatic`. |
| `distribution.signature` | Signing requirement: `sha256` or `ed25519` mandates `package.hash`, ed25519 also requires `publicKey`. |
| `package` | Artifact metadata (`artifact`, `sizeBytes`, `hash`). |

Any schema violation is logged and the manifest is skipped. Signed manifests without a `package.hash` are rejected up-front.

## Runtime validation

The Go agent uses the internal plugin manager to compute telemetry snapshots. During each sync it:

1. Loads manifests from disk and validates them using the shared Go validator.
2. Computes artifact SHA-256 and compares it against the manifest hash when signatures are required.
3. Emits telemetry payloads (`PluginSyncPayload`) describing version, hash, install status, and timestamps.

On the controller, `PluginTelemetryStore.syncAgent` enforces the same contract:

- Unknown modules, incompatible platforms/architectures, or out-of-range versions mark the install as `blocked`.
- Unapproved plugins (runtime `approvalStatus !== 'approved'`) are blocked regardless of reported status.
- Hash mismatches or missing hashes for signed plugins raise audit events in `audit_event`.

Aggregated metrics (`installations`, `lastDeployedAt`, `lastCheckedAt`) are persisted to the `plugin` table so dashboards and API responses stay in sync.

## Telemetry UI & controls

The client detail view (`/(app)/clients/[clientId]/plugins`) now renders live telemetry:

- Status badges for global state and per-agent installs.
- Expected vs. observed hashes with tooltips.
- Delivery policy summary (default mode, auto-update flag).
- Enable/disable toggle per client (persisted in `plugin_installation.enabled`).
- Error and audit signals when installs are blocked.

The API surface:

- `GET /api/clients/:id/plugins` returns `ClientPlugin` views combining manifest, runtime, and telemetry data.
- `PATCH /api/clients/:id/plugins/:pluginId` toggles per-client enablement while preserving audit history.

## Rollout & rollback

1. **Rollout**
   - Publish a manifest with signature metadata and drop the signed binary in the agent's plugin directory.
   - On next sync, the agent reports telemetry. Approved plugins transition to `installed`; mismatches surface as `blocked` with audit trails.
   - Operators can enable or disable delivery per client via the new UI controls.

2. **Rollback**
   - Set the plugin's `approvalStatus` to `rejected` using `/api/plugins/:id` to immediately block further installs.
   - Use the client telemetry view to disable affected agents. Blocked installs log audit entries for incident response.

3. **Recovery**
   - Publish a corrected manifest/binary pair (same ID, higher version). Telemetry automatically reconciles once hashes align.

Refer to `shared/types/plugin-manifest.ts` and `shared/pluginmanifest/manifest.go` for the canonical schema definitions.

## Signature trust configuration

The controller and agent share a trust policy that defines which signatures are accepted. The policy defaults to
`resources/plugin-signers.json` and can be overridden by setting `TENVY_PLUGIN_TRUST_CONFIG` to an absolute or relative path.

```jsonc
{
  "allowUnsigned": false,
  "sha256AllowList": [
    "0123abcd…"
  ],
  "ed25519PublicKeys": {
    "release": "aabbcc…"
  },
  "maxSignatureAgeMs": 1209600000
}
```

- `allowUnsigned`: when `false`, unsigned manifests are rejected by the agent, controller ingestion, telemetry sync, and the
  marketplace approval workflow.
- `sha256AllowList`: case-insensitive SHA-256 hashes that may be installed without a public key. Use sparingly for emergency
  recovery.
- `ed25519PublicKeys`: map of signer IDs to 32-byte Ed25519 public keys (hex). These keys are exposed to the agent so it can
  verify release artifacts locally.
- `maxSignatureAgeMs`: optional upper bound for signature freshness. When set, signatures older than this duration are marked
  expired.

### Provisioning and rotation

1. Generate Ed25519 key pairs on an offline machine. Store public keys in `plugin-signers.json` and distribute the private key
   to your build pipeline.
2. Deploy the updated policy file to both the controller and agents (or update the `TENVY_PLUGIN_TRUST_CONFIG` path). Restart
   services so caches refresh.
3. When rotating keys, add the new key alongside the existing one. Publish a signed manifest and plugin with the new key.
4. Confirm the UI reports the signature as `trusted`, then remove the retired key from the policy and roll out the change.

The policy loader caches values in memory; call `refreshSignaturePolicy()` (used in tests) or restart the server after edits.
