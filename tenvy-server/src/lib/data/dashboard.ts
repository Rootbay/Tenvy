import { countryCodeToFlag } from '$lib/utils/location';
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

type ClientSeed = {
	id: string;
	codename: string;
	status: ClientStatus;
	location: DashboardClient['location'];
	connectedHoursAgo: number;
	lastSeenMinutesAgo: number;
	latencyMs: number;
};

const clientSeeds: ClientSeed[] = [
	{
		id: 'tv-001',
		codename: 'VELA',
		status: 'online',
		location: {
			city: 'Lisbon',
			country: 'Portugal',
			countryCode: 'PT',
			latitude: 38.7223,
			longitude: -9.1393
		},
		connectedHoursAgo: 72,
		lastSeenMinutesAgo: 2,
		latencyMs: 142
	},
	{
		id: 'tv-002',
		codename: 'ORION',
		status: 'idle',
		location: {
			city: 'Berlin',
			country: 'Germany',
			countryCode: 'DE',
			latitude: 52.52,
			longitude: 13.405
		},
		connectedHoursAgo: 12,
		lastSeenMinutesAgo: 7,
		latencyMs: 188
	},
	{
		id: 'tv-003',
		codename: 'HALO',
		status: 'online',
		location: {
			city: 'Toronto',
			country: 'Canada',
			countryCode: 'CA',
			latitude: 43.651,
			longitude: -79.383
		},
		connectedHoursAgo: 4,
		lastSeenMinutesAgo: 1,
		latencyMs: 156
	},
	{
		id: 'tv-004',
		codename: 'LYRA',
		status: 'dormant',
		location: {
			city: 'Austin',
			country: 'United States',
			countryCode: 'US',
			latitude: 30.2672,
			longitude: -97.7431
		},
		connectedHoursAgo: 220,
		lastSeenMinutesAgo: 190,
		latencyMs: 0
	},
	{
		id: 'tv-005',
		codename: 'NOVA',
		status: 'online',
		location: {
			city: 'Reykjavik',
			country: 'Iceland',
			countryCode: 'IS',
			latitude: 64.1466,
			longitude: -21.9426
		},
		connectedHoursAgo: 18,
		lastSeenMinutesAgo: 3,
		latencyMs: 172
	},
	{
		id: 'tv-006',
		codename: 'ATLAS',
		status: 'offline',
		location: {
			city: 'Singapore',
			country: 'Singapore',
			countryCode: 'SG',
			latitude: 1.3521,
			longitude: 103.8198
		},
		connectedHoursAgo: 340,
		lastSeenMinutesAgo: 480,
		latencyMs: 0
	},
	{
		id: 'tv-007',
		codename: 'ECHO',
		status: 'idle',
		location: {
			city: 'Tokyo',
			country: 'Japan',
			countryCode: 'JP',
			latitude: 35.6762,
			longitude: 139.6503
		},
		connectedHoursAgo: 60,
		lastSeenMinutesAgo: 12,
		latencyMs: 214
	},
	{
		id: 'tv-008',
		codename: 'QUILL',
		status: 'dormant',
		location: {
			city: 'Tallinn',
			country: 'Estonia',
			countryCode: 'EE',
			latitude: 59.437,
			longitude: 24.7536
		},
		connectedHoursAgo: 150,
		lastSeenMinutesAgo: 160,
		latencyMs: 0
	},
	{
		id: 'tv-009',
		codename: 'POLAR',
		status: 'online',
		location: {
			city: 'Chicago',
			country: 'United States',
			countryCode: 'US',
			latitude: 41.8781,
			longitude: -87.6298
		},
		connectedHoursAgo: 6,
		lastSeenMinutesAgo: 2,
		latencyMs: 132
	},
	{
		id: 'tv-010',
		codename: 'MISTRAL',
		status: 'online',
		location: {
			city: 'Paris',
			country: 'France',
			countryCode: 'FR',
			latitude: 48.8566,
			longitude: 2.3522
		},
		connectedHoursAgo: 30,
		lastSeenMinutesAgo: 9,
		latencyMs: 168
	},
	{
		id: 'tv-011',
		codename: 'SPECTRUM',
		status: 'online',
		location: {
			city: 'Delhi',
			country: 'India',
			countryCode: 'IN',
			latitude: 28.6139,
			longitude: 77.209
		},
		connectedHoursAgo: 10,
		lastSeenMinutesAgo: 4,
		latencyMs: 226
	},
	{
		id: 'tv-012',
		codename: 'ZEPHYR',
		status: 'offline',
		location: {
			city: 'Sydney',
			country: 'Australia',
			countryCode: 'AU',
			latitude: -33.8688,
			longitude: 151.2093
		},
		connectedHoursAgo: 400,
		lastSeenMinutesAgo: 960,
		latencyMs: 0
	}
];

type NewClientTodaySeed = { hoursAgo: number; count: number };
const newClientsTodaySeed: NewClientTodaySeed[] = [
	{ hoursAgo: 24, count: 0 },
	{ hoursAgo: 21, count: 1 },
	{ hoursAgo: 18, count: 1 },
	{ hoursAgo: 15, count: 0 },
	{ hoursAgo: 12, count: 1 },
	{ hoursAgo: 9, count: 0 },
	{ hoursAgo: 6, count: 1 },
	{ hoursAgo: 3, count: 1 },
	{ hoursAgo: 0, count: 0 }
];

const previousDayNewClients = 3;

type NewClientsWeekSeed = { daysAgo: number; count: number };
const newClientsWeekSeed: NewClientsWeekSeed[] = [
	{ daysAgo: 6, count: 1 },
	{ daysAgo: 5, count: 1 },
	{ daysAgo: 4, count: 2 },
	{ daysAgo: 3, count: 1 },
	{ daysAgo: 2, count: 2 },
	{ daysAgo: 1, count: 1 },
	{ daysAgo: 0, count: 1 }
];

const previousWeekNewClients = 7;

type BandwidthSeed = { hoursAgo: number; inboundMb: number; outboundMb: number };
const bandwidthSeed: BandwidthSeed[] = [
	{ hoursAgo: 22, inboundMb: 820, outboundMb: 640 },
	{ hoursAgo: 20, inboundMb: 780, outboundMb: 590 },
	{ hoursAgo: 18, inboundMb: 910, outboundMb: 720 },
	{ hoursAgo: 16, inboundMb: 970, outboundMb: 760 },
	{ hoursAgo: 14, inboundMb: 1020, outboundMb: 800 },
	{ hoursAgo: 12, inboundMb: 1150, outboundMb: 840 },
	{ hoursAgo: 10, inboundMb: 1210, outboundMb: 890 },
	{ hoursAgo: 8, inboundMb: 1280, outboundMb: 930 },
	{ hoursAgo: 6, inboundMb: 1320, outboundMb: 960 },
	{ hoursAgo: 4, inboundMb: 1380, outboundMb: 1010 },
	{ hoursAgo: 2, inboundMb: 1470, outboundMb: 1080 },
	{ hoursAgo: 0, inboundMb: 1520, outboundMb: 1120 }
];

const previousBandwidthTotalMb = 19840;
const bandwidthCapacityMb = 48000;

type LatencySeed = { minutesAgo: number; latencyMs: number };
const latencySeed: LatencySeed[] = [
	{ minutesAgo: 330, latencyMs: 248 },
	{ minutesAgo: 300, latencyMs: 236 },
	{ minutesAgo: 270, latencyMs: 228 },
	{ minutesAgo: 240, latencyMs: 222 },
	{ minutesAgo: 210, latencyMs: 214 },
	{ minutesAgo: 180, latencyMs: 208 },
	{ minutesAgo: 150, latencyMs: 202 },
	{ minutesAgo: 120, latencyMs: 198 },
	{ minutesAgo: 90, latencyMs: 194 },
	{ minutesAgo: 60, latencyMs: 188 },
	{ minutesAgo: 30, latencyMs: 182 },
	{ minutesAgo: 0, latencyMs: 176 }
];

const previousLatencyAverage = 204;

type LogSeed = {
	id: string;
	clientId: string;
	minutesAgo: number;
	action: string;
	description: string;
	severity: DashboardLogEntry['severity'];
};

const logSeeds: LogSeed[] = [
	{
		id: 'log-001',
		clientId: 'tv-003',
		minutesAgo: 6,
		action: 'task:credential-harvest',
		description: 'Credential sweep completed on finance hosts',
		severity: 'info'
	},
	{
		id: 'log-002',
		clientId: 'tv-009',
		minutesAgo: 14,
		action: 'config:beacon-jitter',
		description: 'Adjusted jitter window to 45-90 seconds',
		severity: 'info'
	},
	{
		id: 'log-003',
		clientId: 'tv-011',
		minutesAgo: 22,
		action: 'alert:process-anomaly',
		description: 'Suspicious credential manager fork detected',
		severity: 'warning'
	},
	{
		id: 'log-004',
		clientId: 'tv-006',
		minutesAgo: 41,
		action: 'connection:lost',
		description: 'Agent dropped from Singapore relay',
		severity: 'critical'
	},
	{
		id: 'log-005',
		clientId: 'tv-010',
		minutesAgo: 55,
		action: 'plugin:deploy',
		description: 'Deployed Sightline Recon module',
		severity: 'info'
	},
	{
		id: 'log-006',
		clientId: 'tv-005',
		minutesAgo: 70,
		action: 'transfer:window-opened',
		description: 'Exfiltration window negotiated (sftp)',
		severity: 'warning'
	},
	{
		id: 'log-007',
		clientId: 'tv-001',
		minutesAgo: 95,
		action: 'notes:sync',
		description: 'Operator shared note synced to workspace',
		severity: 'info'
	},
	{
		id: 'log-008',
		clientId: 'tv-008',
		minutesAgo: 120,
		action: 'scheduler:update',
		description: 'Dormant sleeper heartbeat scheduled for tonight',
		severity: 'info'
	}
];

function computeDeltaPercent(current: number, previous: number): number | null {
	if (previous <= 0) {
		return null;
	}
	const delta = ((current - previous) / previous) * 100;
	return Number.isFinite(delta) ? delta : null;
}

function clamp(value: number, min: number, max: number): number {
	return Math.min(Math.max(value, min), max);
}

export function buildDashboardSnapshot(): DashboardSnapshot {
	const now = new Date();
	const generatedAt = now.toISOString();

	const clients: DashboardClient[] = clientSeeds.map((seed) => ({
		id: seed.id,
		codename: seed.codename,
		status: seed.status,
		connectedAt: new Date(now.getTime() - seed.connectedHoursAgo * HOUR).toISOString(),
		lastSeen: new Date(now.getTime() - seed.lastSeenMinutesAgo * MINUTE).toISOString(),
		metrics: {
			latencyMs: seed.latencyMs
		},
		location: seed.location
	}));

	const totals = clients.reduce(
		(acc, client) => {
			acc.total += 1;
			if (client.status === 'offline') {
				acc.offline += 1;
			} else {
				acc.connected += 1;
				if (client.status === 'online') {
					acc.online += 1;
				}
				if (client.status === 'idle') {
					acc.idle += 1;
				}
				if (client.status === 'dormant') {
					acc.dormant += 1;
				}
			}
			return acc;
		},
		{ total: 0, connected: 0, offline: 0, online: 0, idle: 0, dormant: 0 }
	);

	const todaySeries = newClientsTodaySeed
		.map((point) => ({
			timestamp: new Date(now.getTime() - point.hoursAgo * HOUR).toISOString(),
			count: point.count
		}))
		.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());
	const todayTotal = todaySeries.reduce((sum, point) => sum + point.count, 0);
	const todayDeltaPercent = computeDeltaPercent(todayTotal, previousDayNewClients);

	const weekSeries = newClientsWeekSeed
		.map((point) => ({
			timestamp: new Date(now.getTime() - point.daysAgo * DAY).toISOString(),
			count: point.count
		}))
		.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());
	const weekTotal = weekSeries.reduce((sum, point) => sum + point.count, 0);
	const weekDeltaPercent = computeDeltaPercent(weekTotal, previousWeekNewClients);

	const bandwidthSeries = bandwidthSeed
		.map((point) => {
			const timestamp = new Date(now.getTime() - point.hoursAgo * HOUR).toISOString();
			const totalMb = point.inboundMb + point.outboundMb;
			return {
				timestamp,
				inboundMb: point.inboundMb,
				outboundMb: point.outboundMb,
				totalMb
			};
		})
		.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());

	const totalBandwidthMb = bandwidthSeries.reduce((sum, point) => sum + point.totalMb, 0);
	const totalBandwidthGb = totalBandwidthMb / 1024;
	const bandwidthDeltaPercent = computeDeltaPercent(totalBandwidthMb, previousBandwidthTotalMb);
	const peakMb = Math.max(...bandwidthSeries.map((point) => point.totalMb));
	const usagePercent = clamp((totalBandwidthMb / bandwidthCapacityMb) * 100, 0, 100);

	const latencySeries = latencySeed
		.map((point) => ({
			timestamp: new Date(now.getTime() - point.minutesAgo * MINUTE).toISOString(),
			latencyMs: point.latencyMs
		}))
		.sort((a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime());

	const averageLatency =
		latencySeries.reduce((sum, point) => sum + point.latencyMs, 0) /
		Math.max(latencySeries.length, 1);
	const latencyDeltaMs = Number((averageLatency - previousLatencyAverage).toFixed(1));

	const logs: DashboardLogEntry[] = logSeeds
		.map((seed) => {
			const client = clients.find((entry) => entry.id === seed.clientId) ?? null;
			return {
				id: seed.id,
				clientId: seed.clientId,
				codename: client?.codename ?? seed.clientId,
				timestamp: new Date(now.getTime() - seed.minutesAgo * MINUTE).toISOString(),
				action: seed.action,
				description: seed.description,
				severity: seed.severity,
				countryCode: client?.location.countryCode ?? null,
				city: client?.location.city
			} satisfies DashboardLogEntry;
		})
		.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());

	const countryMap = new Map<string, DashboardCountryStat>();
	for (const client of clients) {
		const existing = countryMap.get(client.location.countryCode);
		if (existing) {
			existing.count += 1;
			if (client.status !== 'offline') {
				existing.onlineCount += 1;
			}
		} else {
			countryMap.set(client.location.countryCode, {
				countryCode: client.location.countryCode,
				countryName: client.location.country,
				flag: countryCodeToFlag(client.location.countryCode),
				count: 1,
				onlineCount: client.status === 'offline' ? 0 : 1,
				percentage: 0
			});
		}
	}

	const countries = Array.from(countryMap.values())
		.map((entry) => ({
			...entry,
			percentage: Number(((entry.count / totals.total) * 100).toFixed(1))
		}))
		.sort((a, b) => b.count - a.count);

	return {
		generatedAt,
		totals,
		newClients: {
			today: {
				total: todayTotal,
				deltaPercent: todayDeltaPercent,
				series: todaySeries
			},
			week: {
				total: weekTotal,
				deltaPercent: weekDeltaPercent,
				series: weekSeries
			}
		},
		bandwidth: {
			totalMb: Number(totalBandwidthMb.toFixed(0)),
			totalGb: Number(totalBandwidthGb.toFixed(2)),
			deltaPercent: bandwidthDeltaPercent,
			capacityMb: bandwidthCapacityMb,
			usagePercent: Number(usagePercent.toFixed(1)),
			peakMb: peakMb,
			series: bandwidthSeries
		},
		latency: {
			averageMs: Number(averageLatency.toFixed(1)),
			deltaMs: latencyDeltaMs,
			series: latencySeries
		},
		clients,
		logs,
		countries
	} satisfies DashboardSnapshot;
}
