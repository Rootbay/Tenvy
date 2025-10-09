export type PluginStatus = 'active' | 'disabled' | 'update' | 'error';
export type PluginCategory =
        | 'collection'
        | 'operations'
        | 'persistence'
        | 'exfiltration'
        | 'transport'
        | 'recovery';

export type PluginDeliveryMode = 'manual' | 'automatic';

export type PluginDistribution = {
        defaultMode: PluginDeliveryMode;
        allowManualPush: boolean;
        allowAutoSync: boolean;
        manualTargets: number;
        autoTargets: number;
        lastManualPush: string;
        lastAutoSync: string;
};

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
        artifact: string;
        distribution: PluginDistribution;
};

type PluginTemplate = {
        id: string;
        name: string;
        description: string;
        category: PluginCategory;
        capabilities: string[];
        version: {
                major: number;
                minor: number;
                basePatch: number;
                patchVariance: number;
        };
        authors: string[];
        statusCycle: PluginStatus[];
        enabledCycle?: boolean[];
        autoUpdateCycle?: boolean[];
        baseInstallations: number;
        installationVariance: number;
        deployedMinutesRange: [number, number];
        checkedMinutesRange: [number, number];
        sizeMb: number;
        sizeVariance: number;
        artifact: string;
        distribution: {
                defaultModeCycle: PluginDeliveryMode[];
                manualBase: number;
                manualVariance: number;
                autoBase: number;
                autoVariance: number;
                manualRangeMinutes: [number, number];
                autoRangeMinutes: [number, number];
                allowManualPushCycle?: boolean[];
                allowAutoSyncCycle?: boolean[];
        };
};

const now = new Date();
const minutesSinceMidnight = now.getHours() * 60 + now.getMinutes();
const dayOfYear = (() => {
        const start = new Date(Date.UTC(now.getUTCFullYear(), 0, 0));
        const diff = now.getTime() - start.getTime();
        return Math.floor(diff / (1000 * 60 * 60 * 24));
})();

function hashCode(input: string): number {
        let hash = 0;
        for (let index = 0; index < input.length; index += 1) {
                hash = (hash * 31 + input.charCodeAt(index)) >>> 0;
        }
        return hash;
}

function cycleValue<T>(values: readonly T[], seed: number, step: number): T {
        return values[(seed + step) % values.length];
}

function jitter(base: number, variance: number, seed: number, step = 0): number {
        if (variance === 0) return base;
        const span = variance * 2 + 1;
        const offset = (seed + dayOfYear + step) % span;
        return base + offset - variance;
}

function randomInRange(seed: number, min: number, max: number, step = 0): number {
        if (min >= max) return min;
        const range = max - min;
        const offset = (seed + dayOfYear + step) % (range + 1);
        return min + offset;
}

function formatRelative(minutes: number): string {
        const clamped = Math.max(0, Math.round(minutes));
        if (clamped <= 1) return 'just now';

        if (clamped < 60) {
                return `${clamped} minute${clamped === 1 ? '' : 's'} ago`;
        }

        const hours = Math.floor(clamped / 60);
        if (hours < 24) {
                return `${hours} hour${hours === 1 ? '' : 's'} ago`;
        }

        const days = Math.floor(hours / 24);
        if (days < 14) {
                return `${days} day${days === 1 ? '' : 's'} ago`;
        }

        const weeks = Math.floor(days / 7);
        return `${weeks} week${weeks === 1 ? '' : 's'} ago`;
}

function createPlugin(template: PluginTemplate): Plugin {
        const seed = hashCode(template.id);
        const timeBucket = Math.floor(minutesSinceMidnight / 15);

        const versionPatch = template.version.basePatch + ((seed + timeBucket) % (template.version.patchVariance + 1));
        const version = `${template.version.major}.${template.version.minor}.${versionPatch}`;

        const author = cycleValue(template.authors, seed, timeBucket % template.authors.length);
        const status = cycleValue(template.statusCycle, seed, Math.floor(minutesSinceMidnight / 20) % template.statusCycle.length);
        const enabled = status !== 'disabled' && cycleValue(template.enabledCycle ?? [true], seed, timeBucket % (template.enabledCycle?.length ?? 1));
        const autoUpdate = cycleValue(template.autoUpdateCycle ?? [true], seed, timeBucket % (template.autoUpdateCycle?.length ?? 1));

        const manualTargets = Math.max(
                0,
                template.distribution.manualBase +
                        jitter(0, template.distribution.manualVariance, seed, timeBucket)
        );
        const autoTargets = Math.max(
                0,
                template.distribution.autoBase + jitter(0, template.distribution.autoVariance, seed, timeBucket + 3)
        );

        const installations = Math.max(
                manualTargets + autoTargets,
                template.baseInstallations + jitter(0, template.installationVariance, seed, timeBucket + 6)
        );

        const sizeOffset = jitter(0, Math.round(template.sizeVariance * 100), seed, timeBucket + 9) / 100;
        const size = `${(template.sizeMb + sizeOffset).toFixed(1)} MB`;

        return {
                id: template.id,
                name: template.name,
                description: template.description,
                version,
                author,
                category: template.category,
                status,
                enabled,
                autoUpdate,
                installations,
                lastDeployed: formatRelative(
                        randomInRange(seed, template.deployedMinutesRange[0], template.deployedMinutesRange[1], timeBucket)
                ),
                lastChecked: formatRelative(
                        randomInRange(seed, template.checkedMinutesRange[0], template.checkedMinutesRange[1], timeBucket + 1)
                ),
                size,
                capabilities: template.capabilities,
                artifact: template.artifact,
                distribution: {
                        defaultMode: cycleValue(
                                template.distribution.defaultModeCycle,
                                seed,
                                Math.floor(minutesSinceMidnight / 60)
                        ),
                        allowManualPush: cycleValue(
                                template.distribution.allowManualPushCycle ?? [true],
                                seed,
                                timeBucket % (template.distribution.allowManualPushCycle?.length ?? 1)
                        ),
                        allowAutoSync: cycleValue(
                                template.distribution.allowAutoSyncCycle ?? [true],
                                seed,
                                timeBucket % (template.distribution.allowAutoSyncCycle?.length ?? 1)
                        ),
                        manualTargets,
                        autoTargets,
                        lastManualPush: formatRelative(
                                randomInRange(
                                        seed,
                                        template.distribution.manualRangeMinutes[0],
                                        template.distribution.manualRangeMinutes[1],
                                        timeBucket + 4
                                )
                        ),
                        lastAutoSync: formatRelative(
                                randomInRange(
                                        seed,
                                        template.distribution.autoRangeMinutes[0],
                                        template.distribution.autoRangeMinutes[1],
                                        timeBucket + 5
                                )
                        )
                }
        };
}

const pluginTemplates: PluginTemplate[] = [
        {
                id: 'plugin-recon-telemetry',
                name: 'Telemetry Recon',
                description:
                        'Continuously profiles host activity and surfaces anomalies across operator dashboards.',
                category: 'collection',
                capabilities: ['process insight', 'network sweep', 'baseline diff', 'telemetry tagging'],
                version: { major: 2, minor: 7, basePatch: 0, patchVariance: 6 },
                authors: ['Tenvy Ops Team', 'Axis Research', 'Nightglass'],
                statusCycle: ['active', 'update', 'active', 'active'],
                autoUpdateCycle: [true, true, false, true],
                baseInstallations: 140,
                installationVariance: 40,
                deployedMinutesRange: [6, 48],
                checkedMinutesRange: [1, 6],
                sizeMb: 6.6,
                sizeVariance: 0.5,
                artifact: 'telemetry-recon.dll',
                distribution: {
                        defaultModeCycle: ['automatic', 'automatic', 'manual'],
                        manualBase: 30,
                        manualVariance: 10,
                        autoBase: 130,
                        autoVariance: 35,
                        manualRangeMinutes: [10, 95],
                        autoRangeMinutes: [3, 40],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [true, true, true, false]
                }
        },
        {
                id: 'plugin-surface-profiler',
                name: 'Surface Profiler',
                description:
                        'Assembles a full inventory of operating system, hardware, and session context for each client.',
                category: 'collection',
                capabilities: ['hardware census', 'session catalog', 'OS fingerprint'],
                version: { major: 1, minor: 4, basePatch: 1, patchVariance: 5 },
                authors: ['SpecterWorks', 'Axis Research'],
                statusCycle: ['active', 'active', 'update'],
                autoUpdateCycle: [true, true, true, false],
                baseInstallations: 120,
                installationVariance: 30,
                deployedMinutesRange: [15, 75],
                checkedMinutesRange: [2, 9],
                sizeMb: 3.6,
                sizeVariance: 0.3,
                artifact: 'surface-profiler.dll',
                distribution: {
                        defaultModeCycle: ['automatic', 'manual', 'automatic'],
                        manualBase: 18,
                        manualVariance: 6,
                        autoBase: 110,
                        autoVariance: 25,
                        manualRangeMinutes: [12, 120],
                        autoRangeMinutes: [4, 55],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [true]
                }
        },
        {
                id: 'plugin-lateral-pivot',
                name: 'Pivot Automator',
                description:
                        'Automates credential replay and network pivoting with guardrails for segmented environments.',
                category: 'operations',
                capabilities: ['credential replay', 'path discovery', 'just-in-time elevation'],
                version: { major: 1, minor: 9, basePatch: 4, patchVariance: 4 },
                authors: ['Axis Research', 'Obsidian Works'],
                statusCycle: ['update', 'active', 'active', 'error'],
                enabledCycle: [true, true, false, true],
                autoUpdateCycle: [false, false, true],
                baseInstallations: 90,
                installationVariance: 28,
                deployedMinutesRange: [25, 110],
                checkedMinutesRange: [8, 25],
                sizeMb: 4.2,
                sizeVariance: 0.4,
                artifact: 'pivot-automator.dll',
                distribution: {
                        defaultModeCycle: ['manual', 'automatic', 'manual'],
                        manualBase: 40,
                        manualVariance: 12,
                        autoBase: 55,
                        autoVariance: 18,
                        manualRangeMinutes: [18, 140],
                        autoRangeMinutes: [10, 80],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [false, true]
                }
        },
        {
                id: 'plugin-ops-orchestrator',
                name: 'Ops Orchestrator',
                description:
                        'Schedules command execution, throttles task concurrency, and tracks audit notes per campaign.',
                category: 'operations',
                capabilities: ['task scheduling', 'rate limiting', 'audit trail'],
                version: { major: 2, minor: 3, basePatch: 0, patchVariance: 6 },
                authors: ['Tenvy Ops Team', 'SpecterWorks'],
                statusCycle: ['active', 'active', 'update'],
                enabledCycle: [true],
                autoUpdateCycle: [true, true, true, false],
                baseInstallations: 135,
                installationVariance: 32,
                deployedMinutesRange: [35, 140],
                checkedMinutesRange: [6, 18],
                sizeMb: 5.0,
                sizeVariance: 0.4,
                artifact: 'ops-orchestrator.dll',
                distribution: {
                        defaultModeCycle: ['automatic', 'automatic', 'manual'],
                        manualBase: 24,
                        manualVariance: 8,
                        autoBase: 122,
                        autoVariance: 28,
                        manualRangeMinutes: [20, 160],
                        autoRangeMinutes: [8, 75],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [true, true, false]
                }
        },
        {
                id: 'plugin-ops-remediator',
                name: 'Remediation Sandbox',
                description:
                        'Stage cleanup actions, sandbox risky automations, and require multi-analyst approval when flagged.',
                category: 'operations',
                capabilities: ['rollback queue', 'sandboxing', 'approval workflow'],
                version: { major: 0, minor: 9, basePatch: 7, patchVariance: 5 },
                authors: ['Nightglass', 'Axis Research'],
                statusCycle: ['error', 'update', 'active', 'error'],
                enabledCycle: [false, true, true, false],
                autoUpdateCycle: [false, false, true],
                baseInstallations: 40,
                installationVariance: 16,
                deployedMinutesRange: [12, 65],
                checkedMinutesRange: [4, 20],
                sizeMb: 6.0,
                sizeVariance: 0.5,
                artifact: 'remediation-sandbox.dll',
                distribution: {
                        defaultModeCycle: ['manual', 'manual', 'automatic'],
                        manualBase: 26,
                        manualVariance: 9,
                        autoBase: 14,
                        autoVariance: 10,
                        manualRangeMinutes: [16, 155],
                        autoRangeMinutes: [20, 190],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [false, false, true]
                }
        },
        {
                id: 'plugin-recovery-anchor',
                name: 'Recovery Anchor',
                description:
                        'Deploys resilient recovery implants, credential caches, and cold-start beacons for compromised hosts.',
                category: 'recovery',
                capabilities: ['credential harvest', 'restore pipelines', 'failsafe beacon'],
                version: { major: 1, minor: 2, basePatch: 3, patchVariance: 6 },
                authors: ['Nightglass', 'Tenvy Ops Team'],
                statusCycle: ['active', 'update', 'active', 'active', 'error'],
                enabledCycle: [true, true, true, false, true],
                autoUpdateCycle: [true, false, true],
                baseInstallations: 98,
                installationVariance: 26,
                deployedMinutesRange: [9, 90],
                checkedMinutesRange: [3, 22],
                sizeMb: 7.2,
                sizeVariance: 0.6,
                artifact: 'recovery-anchor.dll',
                distribution: {
                        defaultModeCycle: ['automatic', 'manual', 'automatic'],
                        manualBase: 34,
                        manualVariance: 11,
                        autoBase: 80,
                        autoVariance: 22,
                        manualRangeMinutes: [14, 120],
                        autoRangeMinutes: [6, 65],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [true, true, false]
                }
        },
        {
                id: 'plugin-persistence-sentinel',
                name: 'Persistence Sentinel',
                description:
                        'Establishes resilient footholds with health checks to recover from tampering or removal attempts.',
                category: 'persistence',
                capabilities: ['implant rotation', 'tamper repair', 'redundant beacons'],
                version: { major: 3, minor: 1, basePatch: 0, patchVariance: 5 },
                authors: ['Nightglass', 'Tenvy Ops Team'],
                statusCycle: ['active', 'active', 'update'],
                autoUpdateCycle: [true, true, false],
                baseInstallations: 125,
                installationVariance: 34,
                deployedMinutesRange: [45, 180],
                checkedMinutesRange: [10, 28],
                sizeMb: 8.0,
                sizeVariance: 0.5,
                artifact: 'persistence-sentinel.dll',
                distribution: {
                        defaultModeCycle: ['automatic', 'automatic', 'manual'],
                        manualBase: 20,
                        manualVariance: 7,
                        autoBase: 130,
                        autoVariance: 32,
                        manualRangeMinutes: [25, 180],
                        autoRangeMinutes: [15, 110],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [true]
                }
        },
        {
                id: 'plugin-persistence-cascade',
                name: 'Cascade Entrenchment',
                description:
                        'Automates layered persistence across registry, scheduled tasks, and recovery partitions.',
                category: 'persistence',
                capabilities: ['registry foothold', 'task scheduler', 'partition seeding'],
                version: { major: 1, minor: 6, basePatch: 2, patchVariance: 4 },
                authors: ['Obsidian Works', 'Axis Research'],
                statusCycle: ['update', 'active', 'active'],
                enabledCycle: [true, true, true],
                autoUpdateCycle: [false, true, false],
                baseInstallations: 80,
                installationVariance: 22,
                deployedMinutesRange: [60, 210],
                checkedMinutesRange: [14, 36],
                sizeMb: 9.0,
                sizeVariance: 0.6,
                artifact: 'cascade-entrenchment.dll',
                distribution: {
                        defaultModeCycle: ['manual', 'manual', 'automatic'],
                        manualBase: 48,
                        manualVariance: 15,
                        autoBase: 44,
                        autoVariance: 18,
                        manualRangeMinutes: [35, 220],
                        autoRangeMinutes: [30, 210],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [false, true]
                }
        },
        {
                id: 'plugin-exfil-hollow',
                name: 'Hollow Channel',
                description:
                        'Low-and-slow exfiltration engine that blends into SaaS traffic with adaptive envelopes.',
                category: 'exfiltration',
                capabilities: ['packet shaping', 'SaaS mimicry', 'dead drop delivery'],
                version: { major: 2, minor: 3, basePatch: 0, patchVariance: 5 },
                authors: ['Obsidian Works', 'SpecterWorks'],
                statusCycle: ['disabled', 'update', 'active'],
                enabledCycle: [false, true, true],
                autoUpdateCycle: [false, true],
                baseInstallations: 55,
                installationVariance: 18,
                deployedMinutesRange: [80, 260],
                checkedMinutesRange: [35, 180],
                sizeMb: 5.6,
                sizeVariance: 0.4,
                artifact: 'hollow-channel.dll',
                distribution: {
                        defaultModeCycle: ['manual', 'manual', 'automatic'],
                        manualBase: 40,
                        manualVariance: 14,
                        autoBase: 22,
                        autoVariance: 12,
                        manualRangeMinutes: [40, 260],
                        autoRangeMinutes: [60, 320],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [false, true]
                }
        },
        {
                id: 'plugin-exfil-scribe',
                name: 'Scribe Relay',
                description:
                        'Streams clipboard, keystroke, and document deltas through compliant exfiltration channels.',
                category: 'exfiltration',
                capabilities: ['delta compression', 'policy aware routing', 'clipboard mirroring'],
                version: { major: 1, minor: 2, basePatch: 4, patchVariance: 4 },
                authors: ['Axis Research', 'Tenvy Ops Team'],
                statusCycle: ['active', 'active', 'update'],
                autoUpdateCycle: [true, true, false],
                baseInstallations: 60,
                installationVariance: 16,
                deployedMinutesRange: [40, 160],
                checkedMinutesRange: [5, 24],
                sizeMb: 4.5,
                sizeVariance: 0.3,
                artifact: 'scribe-relay.dll',
                distribution: {
                        defaultModeCycle: ['automatic', 'automatic', 'manual'],
                        manualBase: 16,
                        manualVariance: 5,
                        autoBase: 62,
                        autoVariance: 20,
                        manualRangeMinutes: [18, 140],
                        autoRangeMinutes: [7, 60],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [true]
                }
        },
        {
                id: 'plugin-relay-transporter',
                name: 'Relay Transporter',
                description:
                        'Encrypted relays with automatic domain fronting rotation for unstable environments.',
                category: 'transport',
                capabilities: ['domain fronting', 'mesh relays', 'packet padding'],
                version: { major: 1, minor: 6, basePatch: 0, patchVariance: 6 },
                authors: ['Tenvy Ops Team', 'SpecterWorks'],
                statusCycle: ['error', 'update', 'active'],
                enabledCycle: [false, true, true],
                autoUpdateCycle: [false, false, true],
                baseInstallations: 65,
                installationVariance: 20,
                deployedMinutesRange: [120, 300],
                checkedMinutesRange: [20, 75],
                sizeMb: 7.5,
                sizeVariance: 0.5,
                artifact: 'relay-transporter.dll',
                distribution: {
                        defaultModeCycle: ['manual', 'automatic', 'automatic'],
                        manualBase: 28,
                        manualVariance: 10,
                        autoBase: 46,
                        autoVariance: 18,
                        manualRangeMinutes: [55, 280],
                        autoRangeMinutes: [18, 95],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [false, true]
                }
        },
        {
                id: 'plugin-transport-horizon',
                name: 'Horizon Tunneler',
                description:
                        'Maintains multipath tunnels with jitter avoidance and predictive failover scheduling.',
                category: 'transport',
                capabilities: ['multipath routing', 'jitter avoidance', 'predictive failover'],
                version: { major: 2, minor: 0, basePatch: 1, patchVariance: 5 },
                authors: ['SpecterWorks', 'Axis Research'],
                statusCycle: ['active', 'active', 'active', 'update'],
                autoUpdateCycle: [true, true, true, false],
                baseInstallations: 100,
                installationVariance: 24,
                deployedMinutesRange: [70, 200],
                checkedMinutesRange: [4, 18],
                sizeMb: 6.7,
                sizeVariance: 0.4,
                artifact: 'horizon-tunneler.dll',
                distribution: {
                        defaultModeCycle: ['automatic', 'automatic', 'manual'],
                        manualBase: 22,
                        manualVariance: 9,
                        autoBase: 96,
                        autoVariance: 26,
                        manualRangeMinutes: [30, 195],
                        autoRangeMinutes: [8, 70],
                        allowManualPushCycle: [true],
                        allowAutoSyncCycle: [true]
                }
        }
];

export const plugins: Plugin[] = pluginTemplates.map((template) => createPlugin(template));

export const pluginStatusLabels: Record<PluginStatus, string> = {
	active: 'Active',
	disabled: 'Disabled',
	update: 'Update available',
	error: 'Attention required'
};

export const pluginStatusStyles: Record<PluginStatus, string> = {
	active: 'border border-emerald-500/20 bg-emerald-500/10 text-emerald-600',
	disabled: 'border border-slate-500/20 bg-slate-500/10 text-slate-600',
	update: 'border border-amber-500/20 bg-amber-500/10 text-amber-600',
	error: 'border border-red-500/20 bg-red-500/10 text-red-600'
};

export const pluginCategoryLabels: Record<PluginCategory, string> = {
        collection: 'Collection',
        operations: 'Operations',
        persistence: 'Persistence',
        exfiltration: 'Exfiltration',
        transport: 'Transport',
        recovery: 'Recovery'
};

export const pluginCategories = Object.keys(pluginCategoryLabels) as PluginCategory[];

export const pluginDeliveryModeLabels: Record<PluginDeliveryMode, string> = {
        manual: 'Manual download',
        automatic: 'Auto on connect'
};
