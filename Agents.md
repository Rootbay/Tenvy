---
context_for: tenvy-server / tenvy-client / shared
formatting: bun format
linting: bun lint
checking: bun check
---

# Agents Specification

## Overview

- **tenvy-server** — the controller, built with **Svelte 5 (Runes Mode)**, **SvelteKit**, **TypeScript**, **TailwindCSS v4**, **shadcn-svelte**, **lucide**, and **Bun**.
- **tenvy-client** — the target agent, written in **Go**, optimized for persistent, reliable, and adaptive remote management.
- **shared/** — Houses shared schemas and utilities—TypeScript definitions under `shared/types` and the Go manifest validator in `shared/pluginmanifest`—consumed by both the server (e.g., importing `shared/types/plugin-manifest`) and the Go agent (`shared/pluginmanifest`).

---

## Features

- **System Info**
- **Notes**
- **Control** (App VNC, Remote Desktop, Webcam, Audio, Keylogger, CMD)
- **Management** (File, Task, Registry, Startup, Clipboard, TCP)
- **Recovery**
- **Options**
- **Miscellaneous** (Open URL, Message Box, Client Chat, Report Window, IP Geolocation, Env Vars)
- **System Controls** (Reconnect, Disconnect)
- **Power** (Shutdown, Restart, Sleep, Logoff)

---

### Roadmap
Planned evolution includes multi-admin synchronization — enabling shared agent state between multiple controllers.

---

## Agent Modules & Plugins

The agent’s functionality is organized into two extensible layers: **Modules** and **Plugins**.

### Modules
Modules are **core components** compiled directly into the agent executable.  
They provide essential capabilities required for baseline operation, communication, and management.

- Always present — no separate downloads or dependencies  
- Designed for reliability, persistence, and minimal footprint  
- Invocable individually or within automation tasks  
- Serve as the foundation for most agent operations

### Plugins
Plugins are **optional extensions** distributed as standalone `.dll` files (or equivalents on non-Windows systems).  
They extend the agent with advanced or situational capabilities beyond the built-in modules.

Each plugin supports two deployment modes:

- **Manual** — requires explicit download by the operator via the controller UI  
- **Automatic** — fetched and installed automatically upon client connection  

This separation allows the system to remain lightweight by default while supporting rich, on-demand extensibility.

---

## Behavior

- **Reconnection Strategy:** adaptive and self-optimizing; avoids fixed intervals
- **Extensibility:** supports dynamic addition of plugins without core recompilation

---

## Internal References

| Component    | Language / Framework               | Role             |
|---------------|------------------------------------|------------------|
| **tenvy-server** | SvelteKit + TypeScript + TailwindCSS v4 | Controller / UI  |
| **tenvy-client** | Go                               | Target Agent     |
