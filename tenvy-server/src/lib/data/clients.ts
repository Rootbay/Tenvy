export type ClientStatus = "online" | "idle" | "dormant" | "offline";
export type ClientPlatform = "windows" | "linux" | "macos";
export type ClientRisk = "Low" | "Medium" | "High";

export type Client = {
  id: string;
  codename: string;
  hostname: string;
  ip: string;
  location: string;
  os: string;
  platform: ClientPlatform;
  version: string;
  status: ClientStatus;
  lastSeen: string;
  tags: string[];
  risk: ClientRisk;
  notes?: string;
};

export const clients: Client[] = [
  {
    id: "tv-001",
    codename: "VELA",
    hostname: "vela-edge",
    ip: "10.21.34.12",
    location: "Lisbon, PT",
    os: "Ubuntu 22.04",
    platform: "linux",
    version: "1.4.2",
    status: "online",
    lastSeen: "Just now",
    tags: ["priority", "intel"],
    risk: "Medium",
    notes: "Streaming live telemetry"
  },
  {
    id: "tv-002",
    codename: "ORION",
    hostname: "orion-dc",
    ip: "172.16.40.5",
    location: "Berlin, DE",
    os: "Windows 11 Pro",
    platform: "windows",
    version: "1.3.9",
    status: "idle",
    lastSeen: "4 minutes ago",
    tags: ["windows", "operations"],
    risk: "Low"
  },
  {
    id: "tv-003",
    codename: "HALO",
    hostname: "halo-hub",
    ip: "10.0.14.87",
    location: "Toronto, CA",
    os: "Windows Server 2019",
    platform: "windows",
    version: "1.4.2",
    status: "online",
    lastSeen: "42 seconds ago",
    tags: ["datacenter", "persistence"],
    risk: "High",
    notes: "Elevated privileges maintained"
  },
  {
    id: "tv-004",
    codename: "LYRA",
    hostname: "lyra-field",
    ip: "192.168.4.32",
    location: "Austin, US",
    os: "macOS 14.4",
    platform: "macos",
    version: "1.3.4",
    status: "dormant",
    lastSeen: "1 hour ago",
    tags: ["intel", "executive"],
    risk: "Medium"
  },
  {
    id: "tv-005",
    codename: "NOVA",
    hostname: "nova-proxy",
    ip: "10.10.5.91",
    location: "Reykjavik, IS",
    os: "Debian 12",
    platform: "linux",
    version: "1.4.0",
    status: "online",
    lastSeen: "2 minutes ago",
    tags: ["relay", "priority"],
    risk: "Low"
  },
  {
    id: "tv-006",
    codename: "ATLAS",
    hostname: "atlas-gw",
    ip: "172.18.14.200",
    location: "Singapore, SG",
    os: "Windows 10 Enterprise",
    platform: "windows",
    version: "1.2.8",
    status: "offline",
    lastSeen: "6 hours ago",
    tags: ["watch", "legacy"],
    risk: "High",
    notes: "Awaiting scheduled reconnect"
  },
  {
    id: "tv-007",
    codename: "ECHO",
    hostname: "echo-edge",
    ip: "10.44.9.76",
    location: "Tokyo, JP",
    os: "Ubuntu 20.04",
    platform: "linux",
    version: "1.3.7",
    status: "idle",
    lastSeen: "9 minutes ago",
    tags: ["asia", "collection"],
    risk: "Medium"
  },
  {
    id: "tv-008",
    codename: "QUILL",
    hostname: "quill-lt",
    ip: "192.168.88.14",
    location: "Tallinn, EE",
    os: "macOS 13.6",
    platform: "macos",
    version: "1.2.5",
    status: "dormant",
    lastSeen: "2 hours ago",
    tags: ["staging", "intel"],
    risk: "Low"
  }
];

export const statusLabels: Record<ClientStatus, string> = {
  online: "Online",
  idle: "Idle",
  dormant: "Dormant",
  offline: "Offline"
};

export const statusStyles: Record<ClientStatus, string> = {
  online: "border border-emerald-500/20 bg-emerald-500/10 text-emerald-600",
  idle: "border border-sky-500/20 bg-sky-500/10 text-sky-600",
  dormant: "border border-amber-500/20 bg-amber-500/10 text-amber-600",
  offline: "border border-slate-500/20 bg-slate-500/10 text-slate-600"
};

export const riskStyles: Record<ClientRisk, string> = {
  Low: "border border-emerald-500/20 bg-emerald-500/10 text-emerald-600",
  Medium: "border border-amber-500/20 bg-amber-500/10 text-amber-600",
  High: "border border-red-500/20 bg-red-500/10 text-red-600"
};

export const statusSummaryOrder: ClientStatus[] = ["online", "idle", "dormant", "offline"];

export const availableTags = Array.from(new Set(clients.flatMap((client) => client.tags))).sort((a, b) =>
  a.localeCompare(b)
);
