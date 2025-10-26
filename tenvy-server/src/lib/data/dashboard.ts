import type { ClientStatus } from './clients';

const MINUTE = 60 * 1000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

export type DashboardClient = {
	id: string;
	codename: string;
	status: ClientStatus;
	connectedAt: string;
	lastSeen: string;
	metrics: {
		latencyMs: number;
	};
	location: {
		city: string;
		country: string;
		countryCode: string;
		latitude: number;
		longitude: number;
	};
};

export type DashboardLogEntry = {
	id: string;
	clientId: string;
	codename: string;
	timestamp: string;
	action: string;
	description: string;
	severity: 'info' | 'warning' | 'critical';
	countryCode: string | null;
	city?: string;
};

export type DashboardCountryStat = {
	countryCode: string;
	countryName: string;
	flag: string;
	count: number;
	onlineCount: number;
	percentage: number;
};

export type DashboardNewClientSnapshot = {
	total: number;
	deltaPercent: number | null;
	series: { timestamp: string; count: number }[];
};

export type DashboardBandwidthSnapshot = {
	totalMb: number;
	totalGb: number;
	deltaPercent: number | null;
	capacityMb: number;
	usagePercent: number;
	peakMb: number;
	series: { timestamp: string; inboundMb: number; outboundMb: number; totalMb: number }[];
};

export type DashboardLatencySnapshot = {
	averageMs: number;
	deltaMs: number;
	series: { timestamp: string; latencyMs: number }[];
};

export type DashboardSnapshot = {
	generatedAt: string;
	totals: {
		total: number;
		connected: number;
		offline: number;
		online: number;
		idle: number;
		dormant: number;
	};
	newClients: {
		today: DashboardNewClientSnapshot;
		week: DashboardNewClientSnapshot;
	};
	bandwidth: DashboardBandwidthSnapshot;
	latency: DashboardLatencySnapshot;
	clients: DashboardClient[];
	logs: DashboardLogEntry[];
	countries: DashboardCountryStat[];
};

export { MINUTE, HOUR, DAY };
