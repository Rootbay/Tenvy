import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { SystemInfoReport } from '$lib/types/system-info';

const queueCommand = vi.fn();
const getAgent = vi.fn();

class MockRegistryError extends Error {
	status: number;

	constructor(message: string, status: number) {
		super(message);
		this.status = status;
	}
}

vi.mock('./store', () => ({
	registry: {
		queueCommand,
		getAgent
	},
	RegistryError: MockRegistryError
}));

describe('requestSystemInfoSnapshot', () => {
	const baseReport: SystemInfoReport = {
		collectedAt: new Date().toISOString(),
		host: {
			hostname: 'test-host',
			hostId: 'host-123',
			ipAddress: '192.168.1.10'
		},
		os: {
			platform: 'linux',
			version: '6.8.0'
		},
		hardware: {
			architecture: 'amd64',
			logicalCores: 8
		},
		memory: {
			totalBytes: 16_000_000_000,
			usedBytes: 8_000_000_000
		},
		storage: [
			{
				device: '/dev/sda1',
				mountpoint: '/',
				filesystem: 'ext4',
				totalBytes: 500_000_000_000,
				usedBytes: 320_000_000_000
			}
		],
		network: [
			{
				name: 'eth0',
				mtu: 1500,
				macAddress: 'aa:bb:cc:dd:ee:ff',
				addresses: [
					{ address: '192.168.1.10', family: 'ipv4' },
					{ address: 'fe80::1', family: 'ipv6' }
				]
			}
		],
		runtime: {
			goVersion: 'go1.22.5',
			goOs: 'linux',
			goArch: 'amd64',
			logicalCpus: 8,
			goMaxProcs: 8,
			goroutines: 32,
			process: {
				pid: 1234,
				commandLine: '/opt/tenvy/agent',
				workingDirectory: '/opt/tenvy',
				createTime: new Date().toISOString()
			}
		},
		environment: {
			username: 'agent',
			homeDir: '/home/agent',
			shell: '/bin/bash',
			pathSeparator: '/',
			tempDir: '/tmp',
			environmentCount: 42
		},
		agent: {
			id: 'agent-1',
			version: '1.2.3',
			startTime: new Date().toISOString(),
			uptimeSeconds: 3600
		}
	};

	const completedAt = new Date().toISOString();

	let module: typeof import('./system-info');

	beforeEach(async () => {
		queueCommand.mockReset();
		getAgent.mockReset();

		queueCommand.mockReturnValue({
			command: {
				id: 'cmd-1',
				name: 'system-info',
				payload: {},
				createdAt: new Date().toISOString()
			},
			delivery: 'queued'
		});

		getAgent.mockReturnValue({
			recentResults: [
				{
					commandId: 'cmd-1',
					success: true,
					output: JSON.stringify(baseReport),
					completedAt
				}
			]
		});

		vi.resetModules();
		module = await import('./system-info');
	});

	afterEach(() => {
		vi.resetModules();
	});

	it('queues a system-info command and returns the parsed snapshot', async () => {
		const snapshot = await module.requestSystemInfoSnapshot('agent-1');

		expect(queueCommand).toHaveBeenCalledWith(
			'agent-1',
			{ name: 'system-info', payload: {} },
			{ operatorId: undefined }
		);

		expect(snapshot.agentId).toBe('agent-1');
		expect(snapshot.requestId).toBe('cmd-1');
		expect(snapshot.receivedAt).toBe(completedAt);
		expect(snapshot.report.hardware.architecture).toBe('amd64');
		expect(snapshot.report.runtime.goVersion).toBe('go1.22.5');
	});

	it('passes the refresh flag when requested', async () => {
		const refreshedSnapshot = await module.requestSystemInfoSnapshot('agent-1', { refresh: true });
		expect(queueCommand).toHaveBeenCalledWith(
			'agent-1',
			{ name: 'system-info', payload: { refresh: true } },
			{ operatorId: undefined }
		);
		expect(refreshedSnapshot.report.collectedAt).toBe(baseReport.collectedAt);
	});

	it('throws when the agent reports an error', async () => {
		getAgent.mockReturnValueOnce({
			recentResults: [
				{
					commandId: 'cmd-1',
					success: false,
					error: 'module disabled',
					completedAt
				}
			]
		});

		await expect(module.requestSystemInfoSnapshot('agent-1')).rejects.toThrowError(
			module.SystemInfoAgentError
		);
	});

	it('throws when the payload cannot be parsed', async () => {
		getAgent.mockReturnValueOnce({
			recentResults: [
				{
					commandId: 'cmd-1',
					success: true,
					output: '{"collectedAt":1}',
					completedAt
				}
			]
		});

		await expect(module.requestSystemInfoSnapshot('agent-1')).rejects.toThrowError(
			module.SystemInfoAgentError
		);
	});

	it('re-throws registry errors from queueCommand', async () => {
		queueCommand.mockReset();
		queueCommand.mockImplementation(() => {
			throw new MockRegistryError('Agent not found', 404);
		});

		vi.resetModules();
		module = await import('./system-info');

		await expect(module.requestSystemInfoSnapshot('agent-404')).rejects.toMatchObject({
			status: 404,
			message: 'Agent not found'
		});
	});
});
