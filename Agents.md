---

context\_for: tenvy-server / tenvy-client

---



\# Agents Specification



\## Overview



The \*\*agent system\*\* consists of two main components:



\- \*\*tenvy-server\*\* — the controller, built with \*\*Tauri v2\*\*, \*\*Svelte 5 (Runes Mode)\*\*, \*\*SvelteKit\*\*, \*\*TypeScript\*\*, \*\*TailwindCSS v4\*\*, \*\*shadcn-svelte\*\*, \*\*lucide\*\*, and \*\*Bun\*\*.

\- \*\*tenvy-client\*\* — the target agent, written in \*\*Go\*\*, optimized for persistent, stealthy, and adaptive remote management.



The agent maintains a continuous, reliable connection with the server, automatically handling reconnection and synchronization using optimal adaptive intervals.



---



\## Navigation (Controller UI)



\- \*\*Dashboard\*\* — summary of active clients, system state, and metrics.

\- \*\*Clients\*\* — list of all connected agents with detailed info and quick actions.

\- \*\*Plugins\*\* — extension and modular feature management for the server and client.

\- \*\*Settings\*\* — configuration and administrative preferences.



Planned evolution: multi-admin synchronization (shared agent state between controllers).



---



\## Agent Modules



Each module corresponds to a functional capability group.  

Modules are independent, extendable, and can be invoked individually or composed into automation tasks.



\### System Info

\- Collect OS, hardware, and environment details.

\- Gather real-time system statistics.



\### Notes

\- Maintain agent-specific notes, tags, or operational metadata.



\### Control

\- \*\*Hidden VNC\*\* — invisible session control.

\- \*\*Remote Desktop\*\* — visible session mirroring and input control.

\- \*\*Webcam Control\*\* — access, stream, capture.

\- \*\*Audio Control\*\* — record or transmit audio streams.

\- \*\*Keylogger\*\*

&nbsp; - \*Online\* — live keystroke streaming.

&nbsp; - \*Offline\* — persistent local logging with later exfiltration.

&nbsp; - \*Advanced Online\* — contextual key mapping and application correlation.

\- \*\*CMD\*\* — remote command shell access.



\### Management

\- \*\*File Manager\*\* — list, upload, download, delete, rename, transfer.

\- \*\*Task Manager\*\* — view, start, stop, or suspend processes.

\- \*\*Registry Manager\*\* — read, edit, and create registry keys (Windows).

\- \*\*Startup Manager\*\* — list and modify startup entries.

\- \*\*Clipboard Manager\*\* — monitor or modify clipboard data.

\- \*\*TCP Connections\*\* — inspect active and listening sockets.



\### Recovery

\- Placeholder for credential or configuration recovery features.



\### Options

\- Manage agent settings, runtime behaviors, and operational preferences.



\### Miscellaneous

\- \*\*Open URL\*\* — instruct target to open a given link.

\- \*\*Message Box\*\* — display a user message on target.

\- \*\*Client Chat\*\* — two-way text communication.

\- \*\*Report Window\*\* — structured or real-time reporting.

\- \*\*IP Geolocation\*\* — resolve and display client location.

\- \*\*Environment Variables\*\* — read or modify system variables.



\### System Controls

\- \*\*Reconnect\*\* — reinitialize connection to controller.

\- \*\*Disconnect\*\* — terminate current session gracefully.



\### Power

\- \*\*Shutdown\*\*

\- \*\*Restart\*\*

\- \*\*Sleep\*\*

\- \*\*Logoff\*\*



---



\## Behavior



\- \*\*Reconnection Strategy:\*\* adaptive and self-optimizing; avoids fixed intervals.

\- \*\*Command Dispatch:\*\* request/response pattern with optional streaming support.

\- \*\*Extensibility:\*\* new modules can be added dynamically via plugin architecture.

\- \*\*Synchronization:\*\* client state is mirrored on the server dashboard in real time.



---



\## Internal References



| Component      | Language / Framework                        | Role                |

|----------------|---------------------------------------------|---------------------|

| tenvy-server   | Tauri v2 + SvelteKit + TypeScript + Tailwind v4 | Controller / UI     |

| tenvy-client   | Go                                          | Target Agent        |

