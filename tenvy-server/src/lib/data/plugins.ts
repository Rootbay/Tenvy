export type PluginStatus = "active" | "disabled" | "update" | "error";
export type PluginCategory =
  | "collection"
  | "operations"
  | "persistence"
  | "exfiltration"
  | "transport";

export type Plugin = {
  id: string;
  name: string;
  description: string;
  version: string;
  author: string;
  category: PluginCategory;
  status: PluginStatus;
  enabled: boolean;
  autoUpdate: boolean;
  installations: number;
  lastDeployed: string;
  lastChecked: string;
  size: string;
  capabilities: string[];
};

export const plugins: Plugin[] = [
  {
    id: "plugin-recon-telemetry",
    name: "Telemetry Recon",
    description: "Continuously profiles host activity and surfaces anomalies across operator dashboards.",
    version: "2.6.1",
    author: "Tenvy Ops Team",
    category: "collection",
    status: "active",
    enabled: true,
    autoUpdate: true,
    installations: 128,
    lastDeployed: "12 minutes ago",
    lastChecked: "Just now",
    size: "6.4 MB",
    capabilities: ["process insight", "network sweep", "baseline diff"]
  },
  {
    id: "plugin-lateral-pivot",
    name: "Pivot Automator",
    description: "Automates credential replay and network pivoting with guardrails for segmented environments.",
    version: "1.9.4",
    author: "Axis Research",
    category: "operations",
    status: "update",
    enabled: true,
    autoUpdate: false,
    installations: 84,
    lastDeployed: "38 minutes ago",
    lastChecked: "5 minutes ago",
    size: "4.2 MB",
    capabilities: ["credential replay", "path discovery", "just-in-time elevation"]
  },
  {
    id: "plugin-persistence-sentinel",
    name: "Persistence Sentinel",
    description: "Establishes resilient footholds with health checks to recover from tampering or removal attempts.",
    version: "3.1.0",
    author: "Nightglass",
    category: "persistence",
    status: "active",
    enabled: true,
    autoUpdate: true,
    installations: 102,
    lastDeployed: "1 hour ago",
    lastChecked: "8 minutes ago",
    size: "8.1 MB",
    capabilities: ["implant rotation", "tamper repair", "redundant beacons"]
  },
  {
    id: "plugin-exfil-hollow",
    name: "Hollow Channel",
    description: "Low-and-slow exfiltration engine that blends into SaaS traffic with adaptive envelopes.",
    version: "2.2.7",
    author: "Obsidian Works",
    category: "exfiltration",
    status: "disabled",
    enabled: false,
    autoUpdate: false,
    installations: 56,
    lastDeployed: "3 hours ago",
    lastChecked: "2 hours ago",
    size: "5.7 MB",
    capabilities: ["packet shaping", "SaaS mimicry", "dead drop delivery"]
  },
  {
    id: "plugin-relay-transporter",
    name: "Relay Transporter",
    description: "Encrypted relays with automatic domain fronting rotation for unstable environments.",
    version: "1.5.3",
    author: "Tenvy Ops Team",
    category: "transport",
    status: "error",
    enabled: false,
    autoUpdate: false,
    installations: 61,
    lastDeployed: "5 hours ago",
    lastChecked: "17 minutes ago",
    size: "7.5 MB",
    capabilities: ["domain fronting", "mesh relays", "packet padding"]
  }
];

export const pluginStatusLabels: Record<PluginStatus, string> = {
  active: "Active",
  disabled: "Disabled",
  update: "Update available",
  error: "Attention required"
};

export const pluginStatusStyles: Record<PluginStatus, string> = {
  active: "border border-emerald-500/20 bg-emerald-500/10 text-emerald-600",
  disabled: "border border-slate-500/20 bg-slate-500/10 text-slate-600",
  update: "border border-amber-500/20 bg-amber-500/10 text-amber-600",
  error: "border border-red-500/20 bg-red-500/10 text-red-600"
};

export const pluginCategoryLabels: Record<PluginCategory, string> = {
  collection: "Collection",
  operations: "Operations",
  persistence: "Persistence",
  exfiltration: "Exfiltration",
  transport: "Transport"
};

export const pluginCategories = Object.keys(pluginCategoryLabels) as PluginCategory[];
