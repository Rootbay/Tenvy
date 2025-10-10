# Remote Desktop Optimization Strategy

This note captures proposed optimizations for the Tenvy remote desktop pipeline. The goal is to reduce latency, increase visual quality, and keep bandwidth usage adaptive across diverse network conditions.

## 1. GPU-Accelerated Video Encoding (H.265/HEVC)

**Rationale**
- CPU-bound encoding limits achievable frame rates during high-motion sessions.
- Modern GPUs expose hardware encoders (NVENC, AMF, Quick Sync) capable of low-latency HEVC output at 2-4× better compression than H.264 for the same quality.

**Implementation Notes**
- Detect supported GPU encoders on the client (e.g., Windows: DXGI + NVENC/Media Foundation; Linux: VA-API + FFmpeg bindings).
- Prefer HEVC main profile with 4:2:0 chroma, 8-bit for broad compatibility, and allow fallbacks to AVC/H.264 when hardware is unavailable.
- Use low-latency presets with capped GOP length (e.g., GOP=30, keyframe interval ≈1s) and enable B-frames only if encoder latency allows (<10 ms).
- Expose encoder selection in the controller UI so operators can override defaults when required.

## 2. Adaptive Bitrate & Resolution Ladder

**Rationale**
- Static bitrates cause degradation under fluctuating network conditions; adaptive streaming keeps input responsive.

**Implementation Notes**
- Establish bitrate/resolution tiers (e.g., 1440p@8 Mbps, 1080p@5 Mbps, 720p@3 Mbps, 480p@1.5 Mbps).
- Continuously sample end-to-end RTT, frame loss, and buffer occupancy to drive an AIMD (additive-increase/multiplicative-decrease) controller.
- Allow the server to request downgraded tiers when viewer bandwidth is constrained while keeping local capture at native resolution for quick upscale if conditions improve.
- Keep capture and encode threads decoupled via bounded queues; drop frames instead of stalling input when bandwidth collapses.

## 3. Region-Based Updates (Dirty Rectangle Streaming)

**Rationale**
- Full-frame encoding is wasteful for largely static desktops.

**Implementation Notes**
- Track changed regions via OS APIs (Windows: `GetChangedRegions` from DXGI, macOS: `CGDisplayStream`, X11: Damage extension/Wayland frame callbacks).
- Stitch dirty rectangles into macroblocks aligned with encoder requirements before submission.
- Combine region-based capture with temporal noise suppression to avoid flicker from transient UI effects.
- When no regions change, insert very-low-bitrate keepalive frames or switch to lossless cursor-only updates.

## 4. Input Responsiveness Enhancements

- Implement a parallel input channel using reliable UDP or QUIC datagrams to avoid contention with the video stream.
- Timestamp pointer/keyboard events client-side and reconcile them with frame presentation times to enable input prediction/render-catchup in the UI.

## 5. Protocol & Transport Considerations

- Adopt QUIC or WebRTC data channels for combined NAT traversal, congestion control, and optional DTLS encryption.
- Negotiate codec support and capabilities during session handshake; include encoder hardware telemetry for diagnostics.
- Support intra-refresh or periodic intra frames to recover from packet loss without forcing full keyframe resets.
- Harden HTTP fallbacks with aggressive connection pooling, TLS 1.2+ enforcement, and HTTP/2 to minimize handshake overhead when QUIC is unavailable.

## 6. Observability & Tuning

- Emit metrics for capture latency, encode latency, bitrate, frame delivery jitter, and packet loss.
- Surface metrics in the controller dashboard with visual thresholds to simplify troubleshooting.
- Record per-session encoder decisions to inform future heuristic tuning.

## 7. Migration Plan

1. Implement capability probing (GPU encoder availability, transport support).
2. Introduce HEVC pipeline behind a feature flag; collect telemetry for success rates.
3. Add adaptive bitrate controller and resolution ladder.
4. Integrate dirty rectangle capture path and fallbacks.
5. Harden transport (QUIC/WebRTC) and update UI controls while ensuring HTTP/TLS fallbacks share the optimized connection pool.
6. Roll out observability dashboards and log sampling.

These steps progressively reduce risk while delivering tangible improvements to the remote desktop experience.
