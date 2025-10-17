export type ActivitySummaryTone = 'positive' | 'warning' | 'neutral';

export type ActivitySummaryMetric = {
	id: string;
	label: string;
	value: string;
	delta: string;
	tone: ActivitySummaryTone;
};

export type ActivityTimelinePoint = {
	timestamp: string;
	active: number;
	idle: number;
	suppressed: number;
};

export type ActivityModuleEntry = {
	module: string;
	executed: number;
	queued: number;
};

export type ActivityLatencyPoint = {
	timestamp: string;
	p50: number;
	p95: number;
};

export type ActivityFlaggedSession = {
	client: string;
	reason: string;
	region: string;
	interactions: number;
	status: 'open' | 'review' | 'suppressed';
};

export type ActivitySnapshot = {
	generatedAt: string;
	summary: ActivitySummaryMetric[];
	timeline: ActivityTimelinePoint[];
	moduleActivity: ActivityModuleEntry[];
	latency: {
		windowLabel: string;
		points: ActivityLatencyPoint[];
		granularityMinutes: number;
	};
	flaggedSessions: ActivityFlaggedSession[];
};

const WINDOW_MINUTES = 45;
const WINDOW_COUNT = 8;

const numberFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 });

function formatNumber(value: number): string {
	return numberFormatter.format(value);
}

function formatDelta(
	value: number,
	comparison: string
): { text: string; tone: ActivitySummaryTone } {
	if (value === 0) {
		return { text: `Stable vs ${comparison}`, tone: 'neutral' };
	}

	const tone: ActivitySummaryTone = value > 0 ? 'positive' : 'warning';
	const sign = value > 0 ? '+' : '−';
	return { text: `${sign}${formatNumber(Math.abs(value))} vs ${comparison}`, tone };
}

export function buildActivitySnapshot(): ActivitySnapshot {
	const now = new Date();

	const timelineSeed: Omit<ActivityTimelinePoint, 'timestamp'>[] = [
		{ active: 12, idle: 6, suppressed: 1 },
		{ active: 14, idle: 6, suppressed: 2 },
		{ active: 16, idle: 5, suppressed: 2 },
		{ active: 19, idle: 6, suppressed: 3 },
		{ active: 22, idle: 7, suppressed: 3 },
		{ active: 20, idle: 6, suppressed: 3 },
		{ active: 21, idle: 5, suppressed: 2 },
		{ active: 24, idle: 4, suppressed: 2 }
	];

	const timeline: ActivityTimelinePoint[] = timelineSeed.map((window, index) => {
		const minutesAgo = WINDOW_MINUTES * (WINDOW_COUNT - 1 - index);
		const timestamp = new Date(now.getTime() - minutesAgo * 60_000);
		return {
			...window,
			timestamp: timestamp.toISOString()
		};
	});

	type ModuleSeed = ActivityModuleEntry & {
		previousExecuted: number;
		previousQueued: number;
	};

	const moduleSeed: ModuleSeed[] = [
		{ module: 'Reconnaissance', executed: 42, queued: 8, previousExecuted: 35, previousQueued: 6 },
		{
			module: 'Credential access',
			executed: 31,
			queued: 6,
			previousExecuted: 27,
			previousQueued: 5
		},
		{ module: 'Persistence', executed: 26, queued: 5, previousExecuted: 22, previousQueued: 4 },
		{ module: 'Collection', executed: 18, queued: 7, previousExecuted: 16, previousQueued: 6 },
		{ module: 'Exfiltration', executed: 11, queued: 6, previousExecuted: 9, previousQueued: 5 }
	];

	const moduleActivity: ActivityModuleEntry[] = moduleSeed.map(
		({ previousExecuted, previousQueued, ...rest }) => rest
	);

	const latencySeed: ActivityLatencyPoint[] = [
		{ timestamp: new Date(now.getTime() - 315 * 60_000).toISOString(), p50: 152, p95: 402 },
		{ timestamp: new Date(now.getTime() - 270 * 60_000).toISOString(), p50: 164, p95: 427 },
		{ timestamp: new Date(now.getTime() - 225 * 60_000).toISOString(), p50: 172, p95: 441 },
		{ timestamp: new Date(now.getTime() - 180 * 60_000).toISOString(), p50: 184, p95: 463 },
		{ timestamp: new Date(now.getTime() - 135 * 60_000).toISOString(), p50: 176, p95: 438 },
		{ timestamp: new Date(now.getTime() - 90 * 60_000).toISOString(), p50: 168, p95: 411 },
		{ timestamp: new Date(now.getTime() - 45 * 60_000).toISOString(), p50: 162, p95: 392 },
		{ timestamp: now.toISOString(), p50: 158, p95: 376 }
	];

	const flaggedSessions: ActivityFlaggedSession[] = [
		{
			client: 'vela-239',
			reason: 'Command flood throttled by safeguard',
			region: 'AMS • 54.210.90.12',
			interactions: 47,
			status: 'open'
		},
		{
			client: 'lyra-082',
			reason: 'Credential cache extracted',
			region: 'FRA • 185.54.32.77',
			interactions: 29,
			status: 'review'
		},
		{
			client: 'solace-441',
			reason: 'Multiple privilege escalations',
			region: 'SFO • 34.90.221.14',
			interactions: 22,
			status: 'open'
		},
		{
			client: 'nadir-116',
			reason: 'Dormant beacon rehydrated',
			region: 'SIN • 103.6.46.220',
			interactions: 18,
			status: 'suppressed'
		}
	];

	const liveBeacons = timeline[timeline.length - 1]?.active ?? 0;
	const previousLiveBeacons = timeline[timeline.length - 2]?.active ?? liveBeacons;
	const { text: liveBeaconDeltaText, tone: liveBeaconTone } = formatDelta(
		liveBeacons - previousLiveBeacons,
		'previous window'
	);

	const totalExecuted = moduleSeed.reduce((total, entry) => total + entry.executed, 0);
	const totalQueued = moduleSeed.reduce((total, entry) => total + entry.queued, 0);

	const escalationsOpen = flaggedSessions.filter((session) => session.status === 'open').length;
	const escalationsReview = flaggedSessions.filter((session) => session.status === 'review').length;

	const newClientsToday = 11;
	const voucherProvisionPercent = 0.86;

	const summary: ActivitySummaryMetric[] = [
		{
			id: 'live-beacons',
			label: 'Live beacons',
			value: formatNumber(liveBeacons),
			delta: liveBeaconDeltaText,
			tone: liveBeaconTone
		},
		{
			id: 'tasks-dispatched',
			label: 'Tasks dispatched',
			value: formatNumber(totalExecuted),
			delta: `${formatNumber(totalQueued)} queued downstream`,
			tone: totalQueued > totalExecuted * 0.4 ? 'warning' : 'neutral'
		},
		{
			id: 'escalations',
			label: 'Escalations',
			value: `${formatNumber(escalationsOpen)} open`,
			delta:
				escalationsReview > 0
					? `${formatNumber(escalationsReview)} awaiting analyst review`
					: 'No pending reviews',
			tone: escalationsOpen > 0 ? 'warning' : 'positive'
		},
		{
			id: 'new-clients',
			label: 'New clients today',
			value: formatNumber(newClientsToday),
			delta: `${Math.round(voucherProvisionPercent * 100)}% provisioned via vouchers`,
			tone: 'neutral'
		}
	];

	return {
		generatedAt: now.toISOString(),
		summary,
		timeline,
		moduleActivity,
		latency: {
			windowLabel: 'Last 8 intervals',
			points: latencySeed,
			granularityMinutes: WINDOW_MINUTES
		},
		flaggedSessions
	} satisfies ActivitySnapshot;
}
