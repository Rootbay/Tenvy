export interface SystemInfoCommandPayload {
  refresh?: boolean;
}

export interface SystemInfoHost {
  hostname?: string;
  hostId?: string;
  domain?: string;
  ipAddress?: string;
  uptimeSeconds?: number;
  bootTime?: string;
  timezone?: string;
}

export interface SystemInfoOS {
  platform?: string;
  family?: string;
  version?: string;
  kernelVersion?: string;
  kernelArch?: string;
  procs?: number;
  virtualization?: string;
}

export interface SystemInfoCPU {
  id: number;
  model?: string;
  vendor?: string;
  family?: string;
  cacheSizeKb?: number;
  mhz?: number;
  stepping?: number;
  microcode?: string;
  cores?: number;
}

export interface SystemInfoHardware {
  architecture: string;
  virtualizationRole?: string;
  virtualizationSystem?: string;
  physicalCores?: number;
  logicalCores?: number;
  cpus?: SystemInfoCPU[];
}

export interface SystemInfoMemory {
  totalBytes?: number;
  availableBytes?: number;
  usedBytes?: number;
  usedPercent?: number;
  swapTotalBytes?: number;
  swapFreeBytes?: number;
  swapUsedBytes?: number;
  swapUsedPercent?: number;
}

export interface SystemInfoStorage {
  device: string;
  mountpoint: string;
  filesystem?: string;
  totalBytes?: number;
  usedBytes?: number;
  freeBytes?: number;
  usedPercent?: number;
  readOnly?: boolean;
}

export interface SystemInfoNetworkAddress {
  address: string;
  family?: string;
}

export interface SystemInfoNetworkInterface {
  name: string;
  mtu?: number;
  macAddress?: string;
  flags?: string[];
  addresses?: SystemInfoNetworkAddress[];
}

export interface SystemInfoProcess {
  pid: number;
  parentPid?: number;
  executable?: string;
  commandLine?: string;
  workingDirectory?: string;
  createTime?: string;
  uptimeSeconds?: number;
  cpuPercent?: number;
  memoryRssBytes?: number;
  memoryVmsBytes?: number;
  numThreads?: number;
}

export interface SystemInfoRuntime {
  goVersion: string;
  goOs: string;
  goArch: string;
  logicalCpus: number;
  goMaxProcs: number;
  goroutines: number;
  process: SystemInfoProcess;
}

export interface SystemInfoEnvironment {
  username?: string;
  homeDir?: string;
  shell?: string;
  lang?: string;
  pathSeparator: string;
  pathEntries?: string[];
  tempDir?: string;
  environmentCount?: number;
}

export interface SystemInfoAgent {
  id?: string;
  version?: string;
  startTime?: string;
  uptimeSeconds?: number;
  tags?: string[];
}

export interface SystemInfoReport {
  collectedAt: string;
  host: SystemInfoHost;
  os: SystemInfoOS;
  hardware: SystemInfoHardware;
  memory: SystemInfoMemory;
  storage?: SystemInfoStorage[];
  network?: SystemInfoNetworkInterface[];
  runtime: SystemInfoRuntime;
  environment: SystemInfoEnvironment;
  agent: SystemInfoAgent;
  warnings?: string[];
}

export interface SystemInfoSnapshot {
  agentId: string;
  requestId: string;
  receivedAt: string;
  report: SystemInfoReport;
}
