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
