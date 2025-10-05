# Tenvy

> **Tenvy** is a modular, high-performance remote administration framework consisting of:
>
> - **tenvy-server** — controller and UI built with **Tauri v2**, **Svelte 5 (Runes Mode)**, **SvelteKit**, **TypeScript**, **TailwindCSS v4**, **shadcn-svelte**, **lucide**, and **Bun**.
> - **tenvy-client** — lightweight Go-based agent designed for persistent and adaptive communication with the controller.

---

## 🧩 Architecture Overview

Tenvy is designed around two synchronized components:
| Component      | Language / Framework                        | Role                   |
|----------------|---------------------------------------------|------------------------|
| **tenvy-server** | Tauri v2 + SvelteKit + TypeScript + Tailwind v4 | Controller \& Interface |
| **tenvy-client** | Go                                         | Agent / Target Node    |

Agents operate as modular runtime units capable of system interaction, management, and control.  
The server handles orchestration, visualization, and plugin management.

---

## 📂 Project Structure

tenvy/
├── tenvy-server/ # Controller UI and logic (Tauri + SvelteKit)
│ ├── src/
│ ├── static/
│ ├── tauri.conf.json
│ └── package.json
│
├── tenvy-client/ # Go agent source code
│ ├── cmd/
│ ├── internal/
│ ├── modules/
│ └── go.mod
│
├── Agents.md
└── README.md

---

## 🚀 Features (Planned)

Tenvy aims for a complete modular agent architecture with the following feature categories:

- **System Info**
- **Notes**
- **Control** (Hidden VNC, Remote Desktop, Webcam, Audio, Keylogger, CMD)
- **Management** (File, Task, Registry, Startup, Clipboard, TCP)
- **Recovery**
- **Options**
- **Miscellaneous** (Open URL, Message Box, Client Chat, Report Window, IP Geolocation, Env Vars)
- **System Controls** (Reconnect, Disconnect)
- **Power** (Shutdown, Restart, Sleep, Logoff)

Each feature group is represented as a module, dynamically managed and executed from the controller UI.

---

## 🖥️ Server UI Overview

The **Tenvy Server** provides a fast, responsive desktop interface built with **Tauri + Svelte 5**.

**Navigation Tabs**
- **Dashboard** — system overview and active session stats  
- **Clients** — connected agents with details and actions  
- **Plugins** — modular feature extensions  
- **Settings** — configuration and preferences

Future updates include **multi-admin synchronization**, allowing shared agent state between servers.

---

## ⚙️ Technology Stack

| Layer           | Technology                                      |
|-----------------|-------------------------------------------------|
| Controller Core | Tauri v2                                        |
| Frontend        | Svelte 5 (Runes Mode), SvelteKit, TypeScript    |
| Styling         | TailwindCSS v4, shadcn-svelte, lucide           |
| Runtime         | Bun                                             |
| Agent           | Go                                              |

---

## 🧠 Internal Documentation

- \[`Agents.md`](./Agents.md) — structured specification of all agent modules, commands, and internal behavior.  

&nbsp; Used for AI reasoning, automation, and code generation context.

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
- Real-time distributed control
- Plugin registry (TypeScript + Go integration)
- Unified telemetry and audit logging

---

## 📜 License

This project is licensed under the **MIT License**.
