# App VNC Workflow

The controller now forwards rich application descriptors to the agent when an App VNC session starts. Each descriptor contains
platform-specific executable paths and virtualization hints that point to seeded browser profiles or communication client data
stores. The server resolves the active agent's OS, trims identifiers, and includes both the descriptor and a resolved
virtualization plan in the `start` command payload.

## Cross-platform application catalogue

- Windows, Linux, and macOS descriptors live in [`tenvy-server/src/lib/data/app-vnc-apps.ts`](../tenvy-server/src/lib/data/app-vnc-apps.ts).
- Each descriptor advertises per-platform executable paths and default seed/data directories. macOS support mirrors the Windows
  and Linux hints so that the controller can fall back to conventional locations if no bundle overrides are configured.

## Seed bundle storage

- Upload seed bundles from the **Seed bundle management** section of the App VNC workspace UI.
- Bundles are stored as ZIP archives under `resources/app-vnc` (override with `TENVY_APP_VNC_RESOURCE_DIR`). Metadata is tracked
  in `manifest.json` alongside the archives.
- The operator may upload independent profile and data bundles per application platform. Removing a bundle reverts the platform
  to its descriptor defaults.

## Runtime seed delivery

1. When the controller queues a `start` command it resolves the agent platform and looks up bundles from `resources/app-vnc`.
2. Matching bundles are converted into relative download URLs (`/api/app-vnc/seeds/{id}/download?agent={agentId}`) and embedded in
   the virtualization plan.
3. The Go agent detects URLs in the plan, downloads the ZIP archive through the authenticated REST endpoint, and expands it into
   the disposable workspace. Existing directories are removed prior to extraction to avoid stale artefacts.
4. If no bundle is defined, the agent falls back to cloning the raw filesystem path supplied by the descriptor.

On receipt, the Go agent's `app-vnc` module performs the following steps:

1. Create an isolated workspace under the temporary App VNC root.
2. Materialise remote bundles or clone any filesystem-based profile/data seeds into that workspace.
3. Apply environment overrides supplied by the virtualization plan.
4. Launch the requested application full-screen on the private surface.
5. Begin a capture loop that snapshots the virtual surface, packages each frame as an `AppVncFramePacket` (with optional cursor metadata), and POSTs it to `/api/agents/{id}/app-vnc/frames`.
6. Monitor heartbeats, configuration updates, and operator input until the session ends.

Frame delivery honours the negotiated session settings: quality presets select PNG or JPEG encoding, the cursor overlay is only included when `captureCursor` is enabled, and clipboard relay remains idle unless operators request sync. Each POST reuses the agent's bearer token and user agent so the controller can validate provenance before handing the payload to `AppVncManager.ingestFrame`.

When the operator stops the session (or the process exits), the module tears down the spawned process, cancels the capture loop, and removes the cloned workspace to avoid persistence. Additional instrumentation records cloning and launch failures in the agent log, making it easier to troubleshoot mismatched seed paths or unsupported executables.
