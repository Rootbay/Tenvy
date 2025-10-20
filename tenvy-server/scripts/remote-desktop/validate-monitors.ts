#!/usr/bin/env bun

interface RemoteDesktopMonitor {
	id: number;
	label: string;
	width: number;
	height: number;
}

interface RemoteDesktopSessionDiagnostics {
	sessionId: string;
	active: boolean;
	monitors?: RemoteDesktopMonitor[];
	transportDiagnostics?: {
		transport: string;
		codec?: string;
		currentBitrateKbps?: number;
		rttMs?: number;
	} | null;
}

interface RemoteDesktopSessionResponse {
	session: RemoteDesktopSessionDiagnostics | null;
}

const baseUrl = process.env.TENVY_TEST_BASE_URL ?? 'http://localhost:3000';
const agentId = process.env.TENVY_TEST_AGENT_ID;

if (!agentId) {
	console.error('TENVY_TEST_AGENT_ID is required.');
	process.exit(1);
}

async function fetchSession(): Promise<RemoteDesktopSessionDiagnostics> {
	const response = await fetch(`${baseUrl}/api/agents/${agentId}/remote-desktop/session`, {
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) {
		throw new Error(`Failed to fetch session state (status ${response.status})`);
	}
	const payload = (await response.json()) as RemoteDesktopSessionResponse;
	if (!payload.session) {
		throw new Error('Remote desktop session is not active.');
	}
	return payload.session;
}

try {
	const session = await fetchSession();
	console.log(`Session ${session.sessionId} is active: ${session.active}`);
	const monitors = session.monitors ?? [];
	if (monitors.length === 0) {
		console.warn('No monitor metadata reported.');
	} else {
		console.log(`Detected ${monitors.length} monitor(s):`);
		for (const monitor of monitors) {
			console.log(`- #${monitor.id} ${monitor.label} (${monitor.width}x${monitor.height})`);
		}
	}
	if (session.transportDiagnostics) {
		console.log('Transport diagnostics:', session.transportDiagnostics);
	}
	if (monitors.length < 2) {
		console.warn('Multi-monitor validation: fewer than two monitors reported.');
	}
} catch (err) {
	console.error('Remote desktop validation failed:', err);
	process.exitCode = 1;
}
