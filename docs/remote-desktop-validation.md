# Remote Desktop Transport Validation

This guide documents the lightweight scripts that can be used to validate QUIC input delivery, WebRTC media diagnostics, and multi-monitor metadata once a remote desktop session is active.

## Engine Plugin Deployment Flow

Remote desktop capture now depends on the external `remote-desktop-engine` plugin. When an operator queues a session, the agent performs the following steps before the controller accepts any commands:

1. The agent asks the controller for the engine manifest and artifact, verifying the signature and package hash before extracting the binary into the plugin cache.
2. The managed engine wrapper spawns the extracted binary and reports the installation status through the periodic plugin telemetry snapshot.
3. The controller refuses to queue remote desktop commands until telemetry confirms that the agent has the required engine version installed and enabled.
4. During transport negotiation the agent includes the staged plugin version, and the controller echoes the required version in the response. Any mismatch causes negotiation to be rejected so the agent can restage the plugin before retrying.

Because the session cannot start until the plugin is in the expected state, operators can rely on the UI telemetry (and the audit log) to confirm that a new engine build has been distributed before launching a stream.

## Prerequisites

* A running controller instance that exposes the SvelteKit API.
* An active remote desktop session for the target agent.
* `bun` installed locally for executing the helper scripts.
* For Linux screen capture builds, install PipeWire development headers (e.g. `libpipewire-0.3-dev`, `libspa-0.2-dev`, and `pkg-config`) and compile the agent with `-tags pipewire` to enable the native backend.
* Environment variables pointing to the controller base URL and the agent identifier:

```bash
export TENVY_TEST_BASE_URL="https://controller.example"
export TENVY_TEST_AGENT_ID="00000000-0000-0000-0000-000000000000"
```

Optionally, set `TENVY_TEST_SESSION_ID` to force a specific session identifier instead of auto-detecting the active session.

## Load Test (`scripts/remote-desktop/load-test.ts`)

This script rapidly dispatches keyboard and pointer bursts through the `/remote-desktop/input` endpoint, capturing end-to-end latency and success rates.

```bash
bun run scripts/remote-desktop/load-test.ts
```

Override defaults with:

* `TENVY_TEST_ITERATIONS` – total number of bursts (default `50`).
* `TENVY_TEST_CONCURRENCY` – number of concurrent in-flight requests (default `4`).

The script prints aggregate latency (`avgLatency`, `p95Latency`) and failure counts to help verify QUIC bypass throughput.

## Multi-Monitor Diagnostics (`scripts/remote-desktop/validate-monitors.ts`)

This helper queries the `/remote-desktop/session` endpoint and dumps reported monitor metadata plus the most recent transport diagnostics:

```bash
bun run scripts/remote-desktop/validate-monitors.ts
```

Warnings are emitted when fewer than two monitors are reported, making it easy to spot multi-display regressions.

## Interpreting Results

* **Consistent <25 ms latency** in the load test implies the QUIC path is functioning.
* **p95 >150 ms** or any failures indicate the agent fell back to websocket delivery—inspect server logs for QUIC handshake errors.
* **Missing monitors** usually means the agent has not yet pushed monitor metadata; confirm the session has streamed at least one frame and retry.

Combine these scripts with the controller UI diagnostics panel to cross-check bitrate, RTT, and codec selection while iterating on encoder settings.

## Automated Backend Coverage

Complement the scripting workflow with the automated capture and monitor integration tests that exercise the DXGI, ScreenCaptureKit, and PipeWire backends:

```bash
go test ./tenvy-client/internal/modules/control/screen \
        ./tenvy-client/internal/modules/control/remotedesktop -run Capture
```

These tests validate backend selection, surface capability diagnostics, and simulate multi-GPU monitor topologies so regressions are caught without requiring hardware swaps.

To cover the clip encoder matrix without requiring a bundled `ffmpeg`, run the targeted native encoder simulations:

```bash
go test ./tenvy-client/internal/modules/control/remotedesktop -run Native
```

The unit tests inject platform-specific factories so Windows (Media Foundation), macOS (VideoToolbox), and Linux (VA-API) code paths are exercised even when cross-compiling from another host.
