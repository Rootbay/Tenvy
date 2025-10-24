# PLUGINS & MODULES — SYSTEM DESIGN SPECIFICATION

## COMPONENTS

**Server (Svelte)**
- Hosts plugin marketplace, manifests, and telemetry endpoints.
- Manages roles, plugin approval, and agent build configuration.
- Exposes APIs for plugin lifecycle and agent communication.

**Agent (Go)**
- Runtime client executing built-in modules and dynamic plugins.
- Communicates with server for sync, telemetry, and plugin delivery.
- Built from configurable template via `/build` endpoint.

**Shared**
- Defines schemas, manifest validators, and telemetry contracts.
- Ensures strict compatibility across server and agent.

---

## RUNTIME HIERARCHY

`
[Disk Stub] → [In-Memory Loader] → [Server] → [Modules + Plugins]
`

**Disk Stub**
- Minimal bootstrap binary.
- Launches or updates in-memory loader.

**In-Memory Loader (Agent Core)**
- Initializes environment and runtime.
- Authenticates with server.
- Loads modules and plugins as needed.
- Enforces manifest validation, version, and signature policies.

**Server (Controller)**
- Provides manifests, plugin binaries, telemetry sync, and marketplace.
- Defines plugin approval and signing policies.

**Modules / Plugins**
- Runtime components extending agent functionality.
- Modules are built-in; plugins are external and dynamic.

---

## MODULES

**Definition**
- Built-in Go components providing persistent capabilities.
- Implement `Module` interface for unified lifecycle.

```go
type Module interface {
    ID() string
    Init(ctx context.Context, cfg Config) error
    Handle(ctx context.Context, cmd Command) error
    UpdateConfig(cfg Config) error
    Shutdown(ctx context.Context) error
}
```

**Lifecycle**

1. Registered in module registry.
2. Initialized at agent startup or re-registration.
3. Handle commands via message routing.
4. Shutdown gracefully on agent exit.

**Examples**

* remote-desktop
* clipboard-manager
* recovery-manager
* audio-bridge

---

## PLUGINS

**Definition**

* External, signed extensions that enhance or extend modules.
* Installed dynamically at runtime via manifest-driven validation.
* Validated by both agent and server using shared schema.

**Capabilities**

* Extend existing module APIs.
* Add new runtime functionality.
* Register telemetry emitters.

**Loader Workflow**

1. Agent requests plugin manifest list.
2. Server returns approved manifests.
3. Agent validates:

   * signature (`sha256` / `ed25519`)
   * platform/architecture compatibility
   * version constraints
   * hash integrity
4. Plugin loaded into memory, registered to module registry.
5. Telemetry updates server.

---

## SHARED PLUGIN MANIFEST SCHEMA

| Field                          | Type     | Description                      |
| ------------------------------ | -------- | -------------------------------- |
| `id`                           | string   | Unique lowercase slug            |
| `name`                         | string   | Display name                     |
| `version`                      | string   | Semantic version                 |
| `entry`                        | string   | Entry path inside plugin archive |
| `capabilities`                 | string[] | Bound module features            |
| `requirements.platforms`       | string[] | Allowed platforms                |
| `requirements.architectures`   | string[] | Allowed architectures            |
| `requirements.minAgentVersion` | string   | Minimum agent version            |
| `requirements.maxAgentVersion` | string   | Maximum agent version            |
| `distribution.defaultMode`     | enum     | `manual` or `automatic`          |
| `distribution.signature`       | enum     | `sha256` or `ed25519`            |
| `package.artifact`             | string   | File name                        |
| `package.hash`                 | string   | SHA256 hash                      |
| `package.sizeBytes`            | int      | Artifact size                    |
| `author`                       | string   | Developer ID                     |
| `homepage`                     | string   | Info URL                         |

**Validation Rules**

* Unknown module IDs are rejected.
* Hash mismatch blocks installation.
* Unsigned or expired manifests are ignored.
* Version out-of-range blocks load.
* Platform or architecture mismatch blocks load.

---

## MARKETPLACE

**Purpose**
Centralized repository for publishing and retrieving plugins.

**Roles**

| Role        | Capabilities                                        |
| ----------- | --------------------------------------------------- |
| `admin`     | Approve, reject, delete plugins. Override policies. |
| `developer` | Upload, version, sign, and manage own plugins.      |
| `user`      | Browse and install approved plugins.                |

**Upload Workflow**

1. Developer uploads plugin package (`.zip` / `.tar.gz`).
2. Server extracts manifest and validates schema.
3. Signature verification using shared trust config.
4. Version uniqueness check.
5. Admin approval required for distribution.

**Distribution**

* Approved plugins appear in marketplace UI.
* Agents pull manifests during sync.
* Installation depends on manifest `defaultMode` and policy.

**Marketplace APIs**

```
GET /api/plugins              → list all plugins
POST /api/plugins             → upload plugin (developer)
PATCH /api/plugins/:id        → update or approve (admin)
GET /api/clients/:id/plugins  → client-specific plugin view
PATCH /api/clients/:id/plugins/:pluginId → enable/disable
```

---

## AGENT–SERVER COMMUNICATION

**Sync Sequence**

1. Agent authenticates to server.
2. Server responds with manifest delta and configuration.
3. Agent installs, updates, or removes plugins.
4. Agent emits telemetry.

**Telemetry Payload**

```json
{
  "pluginId": "string",
  "version": "string",
  "status": "installed|blocked|error|disabled",
  "hash": "string",
  "timestamp": "int64",
  "error": "string|null"
}
```

---

## BUILD SYSTEM

**/build Page**

* Svelte route for creating customized agent binaries.
* Inputs: platform, architecture, modules, signing options, telemetry settings.
* Output: compiled agent binary with embedded configuration.

**Build Steps**

1. User selects configuration.
2. Server serializes build config to JSON.
3. Go build pipeline executes with provided parameters.
4. Output binary stored and returned to UI for download.

**Build Schema Example**

```json
{
  "platform": "windows",
  "architecture": "x86_64",
  "modules": ["remote-desktop", "clipboard"],
  "autostart": true,
  "encryption": "ed25519",
  "telemetryInterval": 60
}
```

---

## LIFECYCLE SEQUENCE

| Stage | Action           | Responsibility                     |
| ----- | ---------------- | ---------------------------------- |
| 1     | Agent starts     | Stub launches loader               |
| 2     | Loader init      | Module registry creation           |
| 3     | Server sync      | Fetch manifests                    |
| 4     | Validation       | Verify signatures, hashes, version |
| 5     | Load plugins     | Register modules and capabilities  |
| 6     | Report telemetry | Send to server                     |
| 7     | Shutdown         | Graceful unload and cleanup        |

---

## SECURITY MODEL

**Signature Enforcement**

* Signatures validated using `resources/plugin-signers.json`.
* Server and agent share trust config.

**Trust Policy Structure**

```json
{
  "allowUnsigned": false,
  "sha256AllowList": [],
  "ed25519PublicKeys": {
    "release": "aabbcc..."
  },
  "maxSignatureAgeMs": 1209600000
}
```

**Enforcement Logic**

* Unsigned plugin → reject.
* Expired signature → reject.
* Unlisted hash → reject.
* Invalid key → reject.

---

## EXTENSION POINTS

**Modules Expose**

* Command handlers
* Event emitters
* Extension interfaces (`RegisterExtension()`)

**Plugins Implement**

* Capability descriptors referencing module extension IDs.
* Runtime registration hooks.

---

## EXTRA

* Dependency graph between plugins.
* Automatic rollback on plugin failure.
* Version conflict resolver.
* WASM sandbox for plugin isolation.
* Plugin telemetry visualization in marketplace.
