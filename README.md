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
- **Miscellaneous** (Open URL, Message Box, Client Chat, Report Window, IP Geolocation, Env Vars)
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
