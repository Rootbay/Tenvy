# App VNC Workflow

The controller now forwards rich application descriptors to the agent when an App VNC session starts. Each descriptor contains
platform-specific executable paths and virtualization hints that point to seeded browser profiles or communication client data
stores. The server resolves the active agent's OS, trims identifiers, and includes both the descriptor and a resolved
virtualization plan in the `start` command payload.

On receipt, the Go agent's `app-vnc` module performs the following steps:

1. Create an isolated workspace under the temporary App VNC root.
2. Clone any referenced profile or data seed paths into that workspace.
3. Apply environment overrides supplied by the virtualization plan.
4. Launch the requested application full-screen on the private surface.
5. Begin a capture loop that snapshots the virtual surface, packages each frame as an `AppVncFramePacket` (with optional cursor metadata), and POSTs it to `/api/agents/{id}/app-vnc/frames`.
6. Monitor heartbeats, configuration updates, and operator input until the session ends.

Frame delivery honours the negotiated session settings: quality presets select PNG or JPEG encoding, the cursor overlay is only included when `captureCursor` is enabled, and clipboard relay remains idle unless operators request sync. Each POST reuses the agent's bearer token and user agent so the controller can validate provenance before handing the payload to `AppVncManager.ingestFrame`.

When the operator stops the session (or the process exits), the module tears down the spawned process, cancels the capture loop, and removes the cloned workspace to avoid persistence. Additional instrumentation records cloning and launch failures in the agent log, making it easier to troubleshoot mismatched seed paths or unsupported executables.
