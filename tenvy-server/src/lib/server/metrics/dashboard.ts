import { registry } from '$lib/server/rat/store';
import { db } from '$lib/server/db';
import { auditEvent as auditEventTable } from '$lib/server/db/schema';
import { countryCodeToFlag } from '$lib/utils/location';
import type {
	DashboardBandwidthSnapshot,
	DashboardClient,
	DashboardCountryStat,
	DashboardLatencySnapshot,
	DashboardLogEntry,
	DashboardNewClientSnapshot,
	DashboardSnapshot
} from '$lib/data/dashboard';
import type { ClientStatus } from '$lib/data/clients';
import type { AgentSnapshot } from '../../../../../shared/types/agent';

const HOUR_MS = 3_600_000;
const DAY_MS = 86_400_000;
const TODAY_INTERVAL_COUNT = 8;
const TODAY_INTERVAL_HOURS = 3;
const WEEK_INTERVAL_DAYS = 7;
const LATENCY_WINDOW_COUNT = 12;
const LATENCY_WINDOW_MINUTES = 30;
const BANDWIDTH_WINDOW_COUNT = 12;
const BANDWIDTH_WINDOW_HOURS = 2;

const numberFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });

type AuditRow = typeof auditEventTable.$inferSelect;

type ParsedAuditRow = {
	agentId: string;
	commandId: string;
	commandName: string;
	queuedAt: Date | null;
	executedAt: Date | null;
	result: string | null;
};

type ParsedResult = {
	success: boolean | null;
	outputLength: number;
};

function coerceDate(value: unknown): Date | null {
	if (!value) {
		return null;
	}
	if (value instanceof Date) {
		return Number.isFinite(value.getTime()) ? value : null;
	}
	if (typeof value === 'number') {
		const date = new Date(value);
		return Number.isFinite(date.getTime()) ? date : null;
	}
	if (typeof value === 'string') {
		const date = new Date(value);
		return Number.isFinite(date.getTime()) ? date : null;
	}
	return null;
}

function parseAuditRow(row: AuditRow): ParsedAuditRow {
	return {
		agentId: row.agentId,
		commandId: row.commandId,
		commandName: row.commandName,
		queuedAt: coerceDate(row.queuedAt),
		executedAt: coerceDate(row.executedAt),
		result: row.result ?? null
	} satisfies ParsedAuditRow;
}

function parseResult(value: string | null): ParsedResult {
	if (!value || value.trim().length === 0) {
		return { success: null, outputLength: 0 };
	}

	try {
		const parsed = JSON.parse(value) as {
			success?: unknown;
			output?: unknown;
			error?: unknown;
		};
		const success = typeof parsed.success === 'boolean' ? parsed.success : null;
		const outputBytes =
			typeof parsed.output === 'string' ? Buffer.byteLength(parsed.output, 'utf-8') : 0;
		const errorBytes =
			typeof parsed.error === 'string' ? Buffer.byteLength(parsed.error, 'utf-8') : 0;
		return { success, outputLength: outputBytes + errorBytes };
	} catch {
		return { success: null, outputLength: 0 };
	}
}

function computeDeltaPercent(current: number, previous: number): number | null {
	if (!Number.isFinite(previous) || previous <= 0) {
		return null;
	}
	const delta = ((current - previous) / previous) * 100;
	return Number.isFinite(delta) ? delta : null;
}

function deriveClientStatus(agent: AgentSnapshot, now: Date): ClientStatus {
	const lastSeen = coerceDate(agent.lastSeen);
	if (agent.status === 'offline') {
		return 'offline';
	}
	if (agent.status === 'error') {
		return 'idle';
	}
	if (lastSeen) {
		const diffMinutes = (now.getTime() - lastSeen.getTime()) / 60_000;
		if (diffMinutes > 240) {
			return 'dormant';
		}
		if (diffMinutes > 10) {
			return 'idle';
		}
	}
	return 'online';
}

function resolveCodename(agent: AgentSnapshot): string {
	const hostname = agent.metadata.hostname?.trim();
	if (hostname && hostname.length > 0) {
		return hostname.toUpperCase();
	}
	const group = agent.metadata.group?.trim();
	if (group && group.length > 0) {
		return group.toUpperCase();
	}
	return agent.id.toUpperCase();
}

function resolveCoordinate(value: unknown): number {
	if (typeof value === 'number' && Number.isFinite(value)) {
		return value;
	}
	if (typeof value === 'string') {
		const parsed = Number.parseFloat(value);
		if (Number.isFinite(parsed)) {
			return parsed;
		}
	}
	return Number.NaN;
}

function buildClientLocation(agent: AgentSnapshot): DashboardClient['location'] {
	const location = agent.metadata.location ?? {};
	const city =
		typeof location.city === 'string' && location.city.trim().length > 0
			? location.city.trim()
			: 'Unknown';
	const country =
		typeof location.country === 'string' && location.country.trim().length > 0
			? location.country.trim()
			: typeof location.region === 'string' && location.region.trim().length > 0
				? location.region.trim()
				: 'Unknown';
	const countryCode =
		typeof location.countryCode === 'string' && location.countryCode.trim().length > 0
			? location.countryCode.trim().toUpperCase()
			: 'UN';
	const latitude = resolveCoordinate((location as { latitude?: unknown }).latitude);
	const longitude = resolveCoordinate((location as { longitude?: unknown }).longitude);
	return { city, country, countryCode, latitude, longitude };
}

function resolveLatency(agent: AgentSnapshot): number {
	const latency = agent.metrics?.latencyMs ?? agent.metrics?.pingMs;
	if (typeof latency === 'number' && Number.isFinite(latency)) {
		return latency;
	}
	return 0;
}

function describeLog(
	event: ParsedAuditRow,
	result: ParsedResult
): { description: string; severity: DashboardLogEntry['severity'] } {
	if (!event.executedAt) {
		return { description: 'Command queued for delivery', severity: 'warning' };
	}
	if (result.success === false) {
		return { description: 'Command execution reported a failure', severity: 'critical' };
	}
	if (result.outputLength > 0) {
		return {
			description: `Command completed (${numberFormatter.format(result.outputLength)} bytes returned)`,
			severity: 'info'
		};
	}
	return { description: 'Command executed successfully', severity: 'info' };
}

function buildNewClientsTodaySnapshot(
	agents: AgentSnapshot[],
	now: Date
): DashboardNewClientSnapshot {
	const series: { timestamp: string; count: number }[] = [];
	for (let index = 0; index < TODAY_INTERVAL_COUNT; index += 1) {
		const windowEnd = new Date(
			now.getTime() - (TODAY_INTERVAL_COUNT - 1 - index) * TODAY_INTERVAL_HOURS * HOUR_MS
		);
		const windowStart = new Date(windowEnd.getTime() - TODAY_INTERVAL_HOURS * HOUR_MS);
		const count = agents.filter((agent) => {
			const connectedAt = coerceDate(agent.connectedAt);
			return connectedAt && connectedAt >= windowStart && connectedAt < windowEnd;
		}).length;
		series.push({ timestamp: windowEnd.toISOString(), count });
	}
	const total = series.reduce((sum, entry) => sum + entry.count, 0);
	const previousDay = agents.filter((agent) => {
		const connectedAt = coerceDate(agent.connectedAt);
		if (!connectedAt) {
			return false;
		}
		const diff = now.getTime() - connectedAt.getTime();
		return diff >= DAY_MS && diff < DAY_MS * 2;
	}).length;
	return {
		total,
		deltaPercent: computeDeltaPercent(total, previousDay),
		series
	} satisfies DashboardNewClientSnapshot;
}

function buildNewClientsWeekSnapshot(
	agents: AgentSnapshot[],
	now: Date
): DashboardNewClientSnapshot {
	const series: { timestamp: string; count: number }[] = [];
	for (let day = 0; day < WEEK_INTERVAL_DAYS; day += 1) {
		const windowEnd = new Date(now.getTime() - (WEEK_INTERVAL_DAYS - 1 - day) * DAY_MS);
		const windowStart = new Date(windowEnd.getTime() - DAY_MS);
		const count = agents.filter((agent) => {
			const connectedAt = coerceDate(agent.connectedAt);
			return connectedAt && connectedAt >= windowStart && connectedAt < windowEnd;
		}).length;
		series.push({ timestamp: windowEnd.toISOString(), count });
	}
	const total = series.reduce((sum, entry) => sum + entry.count, 0);
	const previousWeek = agents.filter((agent) => {
		const connectedAt = coerceDate(agent.connectedAt);
		if (!connectedAt) {
			return false;
		}
		const diff = now.getTime() - connectedAt.getTime();
		return diff >= WEEK_INTERVAL_DAYS * DAY_MS && diff < WEEK_INTERVAL_DAYS * 2 * DAY_MS;
	}).length;
	return {
		total,
		deltaPercent: computeDeltaPercent(total, previousWeek),
		series
	} satisfies DashboardNewClientSnapshot;
}

function buildBandwidthSnapshot(events: ParsedAuditRow[], now: Date): DashboardBandwidthSnapshot {
	const horizonStart = new Date(
		now.getTime() - BANDWIDTH_WINDOW_COUNT * BANDWIDTH_WINDOW_HOURS * HOUR_MS
	);
	const relevantEvents = events.filter((event) => {
		const executedAt = event.executedAt?.getTime() ?? Number.NEGATIVE_INFINITY;
		return executedAt >= horizonStart.getTime();
	});
	const previousWindowStart = new Date(
		horizonStart.getTime() - BANDWIDTH_WINDOW_COUNT * BANDWIDTH_WINDOW_HOURS * HOUR_MS
	);
	const previousEvents = events.filter((event) => {
		const executedAt = event.executedAt?.getTime() ?? Number.NEGATIVE_INFINITY;
		return executedAt >= previousWindowStart.getTime() && executedAt < horizonStart.getTime();
	});

	const series: DashboardBandwidthSnapshot['series'] = [];
	for (let index = 0; index < BANDWIDTH_WINDOW_COUNT; index += 1) {
		const windowEnd = new Date(
			now.getTime() - (BANDWIDTH_WINDOW_COUNT - 1 - index) * BANDWIDTH_WINDOW_HOURS * HOUR_MS
		);
		const windowStart = new Date(windowEnd.getTime() - BANDWIDTH_WINDOW_HOURS * HOUR_MS);
		const windowEvents = relevantEvents.filter((event) => {
			const executedAt = event.executedAt?.getTime();
			return (
				typeof executedAt === 'number' &&
				executedAt >= windowStart.getTime() &&
				executedAt < windowEnd.getTime()
			);
		});
		const inboundBytes = windowEvents.reduce(
			(sum, event) => sum + parseResult(event.result).outputLength,
			0
		);
		const inboundMb = inboundBytes / (1024 * 1024);
		series.push({
			timestamp: windowEnd.toISOString(),
			inboundMb: Number(inboundMb.toFixed(2)),
			outboundMb: 0,
			totalMb: Number(inboundMb.toFixed(2))
		});
	}

	const totalBandwidthMb = series.reduce((sum, entry) => sum + entry.totalMb, 0);
	const totalBandwidthGb = totalBandwidthMb / 1024;
	const previousBandwidthMb =
		previousEvents.reduce((sum, event) => sum + parseResult(event.result).outputLength, 0) /
		(1024 * 1024);
	const bandwidthDeltaPercent = computeDeltaPercent(totalBandwidthMb, previousBandwidthMb);
	const peakMb = series.reduce((max, entry) => Math.max(max, entry.totalMb), 0);

	const capacityMb = 0;
	const usagePercent = capacityMb > 0 ? Math.min(100, (totalBandwidthMb / capacityMb) * 100) : 0;

	return {
		totalMb: Number(totalBandwidthMb.toFixed(2)),
		totalGb: Number(totalBandwidthGb.toFixed(3)),
		deltaPercent: bandwidthDeltaPercent,
		capacityMb,
		usagePercent: Number(usagePercent.toFixed(1)),
		peakMb: Number(peakMb.toFixed(2)),
		series
	} satisfies DashboardBandwidthSnapshot;
}

function buildLatencySnapshot(events: ParsedAuditRow[], now: Date): DashboardLatencySnapshot {
	const series: DashboardLatencySnapshot['series'] = [];
	const allSamples: number[] = [];
	for (let index = 0; index < LATENCY_WINDOW_COUNT; index += 1) {
		const windowEnd = new Date(
			now.getTime() - (LATENCY_WINDOW_COUNT - 1 - index) * LATENCY_WINDOW_MINUTES * 60_000
		);
		const windowStart = new Date(windowEnd.getTime() - LATENCY_WINDOW_MINUTES * 60_000);
		const windowSamples = events
			.filter((event) => {
				const executedAt = event.executedAt?.getTime();
				return (
					typeof executedAt === 'number' &&
					executedAt >= windowStart.getTime() &&
					executedAt < windowEnd.getTime() &&
					event.queuedAt
				);
			})
			.map((event) => {
				const queuedAt = event.queuedAt!.getTime();
				const executedAt = event.executedAt!.getTime();
				return Math.max(0, executedAt - queuedAt);
			});
		windowSamples.forEach((sample) => allSamples.push(sample));
		const average =
			windowSamples.length > 0
				? windowSamples.reduce((sum, sample) => sum + sample, 0) / windowSamples.length
				: 0;
		series.push({
			timestamp: windowEnd.toISOString(),
			latencyMs: Number((average / 1).toFixed(1))
		});
	}
	const averageLatency =
		allSamples.length > 0
			? allSamples.reduce((sum, sample) => sum + sample, 0) / allSamples.length
			: 0;
	const last = series.at(-1)?.latencyMs ?? 0;
	const previous = series.length > 1 ? (series.at(-2)?.latencyMs ?? 0) : last;
	const deltaMs = Number((last - previous).toFixed(1));

	return {
		averageMs: Number(averageLatency.toFixed(1)),
		deltaMs,
		series
	} satisfies DashboardLatencySnapshot;
}

export function buildDashboardSnapshot(): DashboardSnapshot {
	const now = new Date();
	const agents = registry.listAgents();

	const clients: DashboardClient[] = agents.map((agent) => {
		const status = deriveClientStatus(agent, now);
		return {
			id: agent.id,
			codename: resolveCodename(agent),
			status,
			connectedAt: coerceDate(agent.connectedAt)?.toISOString() ?? agent.connectedAt,
			lastSeen: coerceDate(agent.lastSeen)?.toISOString() ?? agent.lastSeen,
			metrics: { latencyMs: resolveLatency(agent) },
			location: buildClientLocation(agent)
		} satisfies DashboardClient;
	});

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

	const rawEvents = db.select().from(auditEventTable).all() as AuditRow[];
	const parsedEvents = rawEvents.map(parseAuditRow);

	const newClientsToday = buildNewClientsTodaySnapshot(agents, now);
	const newClientsWeek = buildNewClientsWeekSnapshot(agents, now);
	const bandwidth = buildBandwidthSnapshot(parsedEvents, now);
	const latency = buildLatencySnapshot(parsedEvents, now);

	const clientsById = new Map(clients.map((client) => [client.id, client] as const));
	const logs: DashboardLogEntry[] = parsedEvents
		.map((event) => {
			const client = clientsById.get(event.agentId) ?? null;
			const result = parseResult(event.result);
			const { description, severity } = describeLog(event, result);
			const timestamp = event.executedAt ?? event.queuedAt ?? new Date();
			return {
				id: `audit-${event.commandId}`,
				clientId: event.agentId,
				codename: client?.codename ?? event.agentId,
				timestamp: timestamp.toISOString(),
				action: `command:${event.commandName}`,
				description,
				severity,
				countryCode: client?.location.countryCode ?? null,
				city: client?.location.city
			} satisfies DashboardLogEntry;
		})
		.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
		.slice(0, 50);

	const countryMap = new Map<string, DashboardCountryStat>();
	for (const client of clients) {
		const code = client.location.countryCode || 'UN';
		const existing = countryMap.get(code);
		if (existing) {
			existing.count += 1;
			if (client.status !== 'offline') {
				existing.onlineCount += 1;
			}
			continue;
		}
		countryMap.set(code, {
			countryCode: code,
			countryName: client.location.country,
			flag: countryCodeToFlag(code),
			count: 1,
			onlineCount: client.status === 'offline' ? 0 : 1,
			percentage: 0
		});
	}

	const countries: DashboardCountryStat[] = Array.from(countryMap.values())
		.map((entry) => ({
			...entry,
			percentage: totals.total > 0 ? Number(((entry.count / totals.total) * 100).toFixed(1)) : 0
		}))
		.sort((a, b) => b.count - a.count);

	return {
		generatedAt: now.toISOString(),
		totals,
		newClients: {
			today: newClientsToday,
			week: newClientsWeek
		},
		bandwidth,
		latency,
		clients,
		logs,
		countries
	} satisfies DashboardSnapshot;
}
