# Remote Desktop Transport Validation

This guide documents the lightweight scripts that can be used to validate QUIC input delivery, WebRTC media diagnostics, and multi-monitor metadata once a remote desktop session is active.

## Prerequisites

* A running controller instance that exposes the SvelteKit API.
* An active remote desktop session for the target agent.
* `bun` installed locally for executing the helper scripts.
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
