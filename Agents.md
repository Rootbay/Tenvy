---
context_for: tenvy-server / tenvy-client
---

# Agents Specification

## Overview
The **agent system** consists of two main components:

- **tenvy-server** — the controller, built with **Svelte 5 (Runes Mode)**, **SvelteKit**, **TypeScript**, **TailwindCSS v4**, **shadcn-svelte**, **lucide**, and **Bun**.
- **tenvy-client** — the target agent, written in **Go**, optimized for persistent, stealthy, and adaptive remote management.

The agent maintains a continuous, reliable connection with the server, automatically handling reconnection and synchronization using optimal adaptive intervals.

---

## Navigation (Controller UI)

- **Dashboard** — summary of active clients, system state, and metrics.
- **Clients** — list of all connected agents with detailed info and quick actions.
- **Plugins** — extension and modular feature management for the server and client.
- **Settings** — configuration and administrative preferences.

Planned evolution: multi-admin synchronization (shared agent state between controllers).

---

## Agent Modules

Each module corresponds to a functional capability group.  
Modules are independent, extendable, and can be invoked individually or composed into automation tasks.

### System Info
- Collect OS, hardware, and environment details.
- Gather real-time system statistics.

### Notes
- Maintain agent-specific notes, tags, or operational metadata.

### Control
- **Hidden VNC** — invisible session control.
- **Remote Desktop** — visible session mirroring and input control.
- **Webcam Control** — access, stream, capture.
- **Audio Control** — record or transmit audio streams.
- **Keylogger**
&nbsp; - *Online* — live keystroke streaming.
&nbsp; - *Offline* — persistent local logging with later exfiltration.
&nbsp; - *Advanced Online* — contextual key mapping and application correlation.
- **CMD** — remote command shell access.

### Management
- **File Manager** — list, upload, download, delete, rename, transfer.
- **Task Manager** — view, start, stop, or suspend processes.
- **Registry Manager** — read, edit, and create registry keys (Windows).
- **Startup Manager** — list and modify startup entries.
- **Clipboard Manager** — monitor or modify clipboard data.
- **TCP Connections** — inspect active and listening sockets.

### Recovery
- Credential or configuration recovery features.

### Options
- Manage agent settings, runtime behaviors, and operational preferences.

### Miscellaneous
- **Open URL** — instruct target to open a given link.
- **Message Box** — display a user message on target.
- **Client Chat** — two-way text communication.
- **Report Window** — structured or real-time reporting.
- **IP Geolocation** — resolve and display client location.
- **Environment Variables** — read or modify system variables.

### System Controls
- **Reconnect** — reinitialize connection to controller.
- **Disconnect** — terminate current session gracefully.

### Power
- **Shutdown**
- **Restart**
- **Sleep**
- **Logoff**

---

## Behavior

- **Reconnection Strategy:** adaptive and self-optimizing; avoids fixed intervals.
- **Command Dispatch:** request/response pattern with optional streaming support.
- **Extensibility:** new modules can be added dynamically via plugin architecture.
- **Synchronization:** client state is mirrored on the server dashboard in real time.

---

## Internal References

| Component      | Language / Framework                        | Role                |
|----------------|---------------------------------------------|---------------------|
| tenvy-server   | SvelteKit + TypeScript + Tailwind v4 | Controller / UI     |
| tenvy-client   | Go                                          | Target Agent        |

---

## Authentication System

1. **Redeem voucher (license)**

* User lands on `/redeem`, enters a **high-entropy voucher** (from you or a reseller).
* Backend validates the voucher (unused, not revoked, not expired).
* Create a **pseudonymous account** (`user.id` = random UUID; no email/username yet).
* Attach the voucher to the user.

2. **Create passkey**

* Immediately prompt to “Create a passkey” (WebAuthn ceremony).
* Store the user’s WebAuthn credential public key (never secrets).
* Issue a **short-lived session** (Lucia) and rotate to a long-lived session after passkey is set.

3. **Show recovery options (optional, clearly opt-in)**

* **Recovery codes** (one-time, hashed at rest).
* Optionally let the user add a **TOTP strictly for recovery**. Recommend Aegis Authenticator & Ente Auth.

4. **Subsequent logins**

* Passkey click → WebAuthn assertion → Lucia session.
* No email, no password, no username.

---

### 🔍 Optional additions

* **Device linking:** allow adding additional passkeys per device (e.g., via QR code or a short-lived linking token).  
* **Session management:** show active sessions and let users revoke them individually.  
* **Rate limiting:** apply per-IP and per-user rate limits for voucher redemption and WebAuthn ceremonies.  
* **Expiration handling:** if a voucher expires or is revoked, restrict access gracefully until renewed.  
* **Security hygiene:** use secure, HttpOnly, SameSite-Strict cookies for sessions; rotate on login; and apply a short idle timeout.  
* **Data minimization:** store only essential identifiers and hashed secrets; avoid collecting IPs or PII unless needed for abuse prevention.  
