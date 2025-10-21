# Tenvy

> **Tenvy** is a modular, high-performance remote administration framework consisting of:
>
> - **tenvy-server** — controller and UI built with **Svelte 5 (Runes Mode)**, **SvelteKit**, **TypeScript**, **TailwindCSS v4**, **shadcn-svelte**, **lucide**, and **Bun**.
> - **tenvy-client** — lightweight Go-based agent designed for persistent and adaptive communication with the controller.

---

## 🧩 Architecture Overview

Tenvy is designed around two synchronized components:
| Component      | Language / Framework                        | Role                   |
|----------------|---------------------------------------------|------------------------|
| **tenvy-server** | SvelteKit + TypeScript + Tailwind v4 | Controller \& Interface |
| **tenvy-client** | Go                                         | Agent / Target Node    |

Agents operate as modular runtime units capable of system interaction, management, and control.  
The server handles orchestration, visualization, and plugin management.

---

## 🚀 Features

Tenvy aims for a complete modular agent architecture with the following feature categories:

- **System Info**
- **Notes**
- **Control** (App VNC, Remote Desktop, Webcam, Audio, Keylogger, CMD)
- **Management** (File, Task, Registry, Startup, Clipboard, TCP)
- **Recovery**
- **Options**
- **Miscellaneous** (Open URL, Client Chat, Trigger Monitor, IP Geolocation, Env Vars)
- **System Controls** (Reconnect, Disconnect)
- **Power** (Shutdown, Restart, Sleep, Logoff)

Each feature group is represented as a module, dynamically managed and executed from the controller UI.

---

## ⚙️ Technology Stack

| Layer           | Technology                                      |
|-----------------|-------------------------------------------------|
| Controller Core | SvelteKit                                       |
| Frontend        | Svelte 5 (Runes Mode), SvelteKit, TypeScript    |
| Styling         | TailwindCSS v4, shadcn-svelte, lucide           |
| Runtime         | Bun                                             |
| Agent           | Go                                              |

---

## 🖱️ Linux Remote Desktop Requirements

Wayland sessions on Linux now rely on a virtual input device created via `/dev/uinput`. To allow the agent to inject keyboard and pointer events:

- Ensure the `uinput` kernel module is available and `/dev/uinput` is writable by the agent process (typically by adding the user to the `input` group or configuring udev rules).
- wlroots/Wayland compositors may require enabling virtual input support; consult your compositor documentation if events are ignored.

### 🎞️ Native Clip Encoder Toolchains

Remote desktop clips now prefer platform encoders before falling back to `ffmpeg`. Building those native backends requires the following toolchains in addition to a Go toolchain with `CGO_ENABLED=1`:

| Platform | Required SDKs / Packages | Notes |
|----------|--------------------------|-------|
| Windows  | Windows 10 (or later) SDK with Media Foundation headers and `mfplat.lib` on the link path. | Builds must run in a Visual Studio Developer Command Prompt or have `VCINSTALLDIR`/`WindowsSdkDir` exported so `go build` can find the libraries. |
| macOS    | Xcode Command Line Tools (or full Xcode) providing `VideoToolbox.framework`. | `CGO_CFLAGS`/`CGO_LDFLAGS` should include `-framework VideoToolbox` when cross-compiling. |
| Linux    | VA-API development headers (`libva-dev` or distribution equivalent) and access to `/dev/dri/renderD128`. | When VA-API is unavailable the agent falls back to the software encoder path. |

Cross-compiling the agent should add the relevant SDK paths via `PKG_CONFIG_PATH`/`CGO_CFLAGS`/`CGO_LDFLAGS`. When these prerequisites are missing, the encoder factory records telemetry and the runtime transparently falls back to the existing `ffmpeg` worker.

---

### 🔑 Development access voucher

When running the server locally a development voucher is created automatically so you can complete the onboarding flow without touching the database manually. The default code is `TEN-VY-DEV-ACCESS-0000`, but you can override it by setting the `DEV_VOUCHER_CODE` environment variable before starting the server.

---

## 🧩 Plugin System

Tenvy supports a unified plugin structure for both **server** and **client** sides.  
Plugins are dynamically loadable and communicate via defined message schemas.

| Plugin Type | Description |
|--------------|--------------|
| **Server Plugin** | Extends controller UI or backend logic |
| **Client Plugin** | Adds new system modules or command handlers |
| **Shared Plugin** | Implements both UI and agent-side logic |

---

## 🔮 Future Plans

- Multi-admin synchronization (shared agent state)
- Plugin registry (TypeScript + Go integration)
- Remote desktop streaming optimizations ([roadmap](./docs/remote-desktop-optimizations.md))

---

## 📜 License

This project is licensed under the **MIT License**.
