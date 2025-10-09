# Modules & Plugins System — Improvement Review

## Scope

This document reviews the current state of the built-in modules compiled into the Go agent and the plugin scaffolding exposed by the SvelteKit controller. It highlights architectural gaps that limit extensibility, maintainability, and runtime safety, then recommends concrete improvements.

## Current Architecture Findings

### Agent module orchestration

- Module wiring is duplicated across the agent runtime. Construction of the remote desktop, audio bridge, clipboard manager, and recovery manager is repeated in both `internal/agent/runtime.go` during startup and `internal/agent/lifecycle.go` during re-registration, and similar fields are also enumerated in `internal/agent/agent.go`. The repetition makes it easy for modules to drift out of sync when configuration fields change or when new modules are introduced.
- Modules are plain structs without a shared lifecycle contract. Each component exposes bespoke `New...` and `UpdateConfig` helpers, but there is no interface or registry that describes capabilities, required dependencies, or how modules should reconcile state during reconnects.
- Command dispatch is tightly coupled to the agent type. Adding a module requires editing several switch statements and helper methods rather than registering command handlers declaratively.

### Plugin catalogue & distribution (controller)

- The controller mocks plugin inventory data through static templates in `src/lib/data/plugins.ts`. Every UI refresh derives pseudo-random status and version data from those templates, but there is no manifest ingestion pipeline, compatibility metadata, or linkage to actual artifacts.
- Delivery modes (manual vs. automatic) are encoded as booleans that are recomputed in place. There is no central policy object that reconciles operator preferences, agent capabilities, and plugin prerequisites.
- Plugins and modules are described separately. There is no shared schema that allows a plugin to declare the modules or capabilities it extends on the agent, which prevents automated compatibility checks.

### Cross-cutting observations

- State reconciliation across reconnects is bespoke per module. Without a shared lifecycle hook, it is difficult to ensure that modules flush in-flight work and resume cleanly after a network partition.
- There is no canonical manifest or signing story for client plugins. The current design assumes pre-shipped DLLs, but does not define how the agent validates compatibility, architecture, or trust.

## Recommended Improvements

1. **Introduce a module registry and lifecycle interface.** Define a `Module` interface that supports `Init`, `Handle`, `UpdateConfig`, and `Shutdown` hooks. Maintain a registry that instantiates modules from configuration, and iterate over that registry during runtime and re-registration to remove duplicated wiring.
2. **Describe module capabilities declaratively.** Augment each module with metadata (supported commands, required permissions, resource usage) so the controller can expose accurate tooling and so plugins can declare extension points.
3. **Create a shared plugin manifest schema.** Store manifests in a shared package (e.g., `shared/pluginmanifest`) that both the Go agent and the Svelte controller consume. Include fields for versioning, compatible agent builds, required modules, delivery policy, and signing details.
4. **Persist plugin catalogue data.** Replace the synthetic data in `plugins.ts` with a lightweight persistence layer or manifest loader so that operator changes (enable/disable, delivery mode) survive reloads and synchronize with agents.
5. **Implement policy-driven plugin delivery.** Centralize delivery mode evaluation so that operator intent, agent capabilities, and network constraints produce a deterministic deployment plan (manual queue, auto-sync cadence, rollback behavior).
6. **Add manifest validation and signature checks to the agent.** When downloading plugins, verify architecture, semantic version compatibility, and cryptographic signatures before loading the artifact.
7. **Document the plugin lifecycle.** Provide runbooks for plugin packaging, publication, rollout, and rollback, including how modules should surface telemetry or errors back to the controller.

## Next Steps

- Prototype the module registry and migrate an existing module (e.g., clipboard) to validate the interface.
- Design the shared manifest schema and update the controller to read manifests from disk during development.
- Define signing and trust requirements, and add verification scaffolding to the agent download path.
- Expand the controller UI to surface manifest-driven compatibility signals and delivery policy outcomes.
