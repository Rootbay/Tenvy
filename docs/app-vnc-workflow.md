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
5. Monitor heartbeats, configuration updates, and operator input until the session ends.

When the operator stops the session (or the process exits), the module tears down the spawned process and removes the cloned
workspace to avoid persistence. Additional instrumentation records cloning and launch failures in the agent log, making it easier
to troubleshoot mismatched seed paths or unsupported executables.
