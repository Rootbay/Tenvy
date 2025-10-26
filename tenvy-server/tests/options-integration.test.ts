import { beforeEach, describe, expect, it, vi } from 'vitest';
import { spawn } from 'node:child_process';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

vi.mock('$env/dynamic/private', () => import('./mocks/env-dynamic-private.ts'));

const requireOperator = vi.fn(
	(user: { id: string; role: string } | null | undefined) =>
		user ?? { id: 'operator', role: 'operator' }
);
const requireViewer = vi.fn(
	(user: { id: string; role: string } | null | undefined) =>
		user ?? { id: 'viewer', role: 'viewer' }
);

vi.mock('../src/lib/server/authorization.js', () => ({
	requireOperator,
	requireViewer
}));

const storeModulePromise = import('../src/lib/server/rat/store.js');
const commandsModulePromise = import('../src/routes/api/agents/[id]/commands/+server.js');

import type { Command, CommandQueueResponse, CommandResult } from '../../shared/types/messages.js';

const repoRoot = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const agentModuleRoot = join(repoRoot, 'tenvy-client');

async function resetRegistry() {
	const { registry } = await storeModulePromise;
	(
		registry as unknown as {
			agents?: Map<string, unknown>;
			fingerprints?: Map<string, unknown>;
			sessionTokens?: Map<string, unknown>;
		}
	).agents?.clear();
	(registry as unknown as { fingerprints?: Map<string, unknown> }).fingerprints?.clear();
	(registry as unknown as { sessionTokens?: Map<string, unknown> }).sessionTokens?.clear();
	(registry as unknown as { logCommandQueued?: (...args: unknown[]) => void }).logCommandQueued =
		() => {};
}

function runAgentCommand(command: Command): Promise<CommandResult> {
	return new Promise((resolve, reject) => {
		const child = spawn('go', ['run', './internal/operations/options/integrationhelper'], {
			cwd: agentModuleRoot
		});

		let stdout = '';
		let stderr = '';

		child.stdout.setEncoding('utf8');
		child.stdout.on('data', (chunk) => {
			stdout += chunk;
		});
		child.stderr.setEncoding('utf8');
		child.stderr.on('data', (chunk) => {
			stderr += chunk;
		});

		child.on('error', (error) => {
			reject(error);
		});

		child.on('close', (code) => {
			if (code !== 0) {
				reject(new Error(`agent helper exited with code ${code}: ${stderr.trim()}`));
				return;
			}
			try {
				const parsed = JSON.parse(stdout) as CommandResult;
				resolve(parsed);
			} catch (error) {
				reject(
					new Error(
						`failed to parse agent result: ${
							error instanceof Error ? error.message : String(error)
						}`
					)
				);
			}
		});

		child.stdin.write(JSON.stringify(command));
		child.stdin.end();
	});
}

describe('options integration', () => {
	beforeEach(async () => {
		requireOperator.mockClear();
		requireViewer.mockClear();
		await resetRegistry();
	});

	it('routes option command results from the agent', async () => {
		const { registry } = await storeModulePromise;
		const { POST } = await commandsModulePromise;
		if (!POST) throw new Error('POST handler missing');

		const registration = registry.registerAgent({
			metadata: {
				hostname: 'integration-host',
				username: 'tester',
				os: 'windows',
				architecture: 'x86_64'
			}
		});

		const commandRequest = {
			name: 'tool-activation',
			payload: {
				toolId: 'options',
				action: 'operation:defender-exclusion',
				metadata: { enabled: true }
			}
		};

		const postResponse = await POST({
			params: { id: registration.agentId },
			request: new Request('https://controller.test/api', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(commandRequest)
			}),
			locals: { user: { id: 'operator', role: 'operator' } }
		});

		expect(postResponse.status).toBe(200);
		const queued = (await postResponse.json()) as CommandQueueResponse;
		expect(queued.command.name).toBe('tool-activation');

		const syncResponse = await registry.syncAgent(registration.agentId, registration.agentKey, {
			status: 'online',
			timestamp: new Date().toISOString()
		});
		expect(syncResponse.commands).toHaveLength(1);

		const result = await runAgentCommand(syncResponse.commands[0] as Command);
		expect(result.commandId).toBe(queued.command.id);
		expect(result.success).toBe(true);
		expect(result.output).toContain('Stub defender exclusion enabled');

		await registry.syncAgent(registration.agentId, registration.agentKey, {
			status: 'online',
			timestamp: new Date().toISOString(),
			results: [result]
		});

		const snapshot = registry.getAgent(registration.agentId);
		const recent = snapshot.recentResults.find((entry) => entry.commandId === queued.command.id);
		expect(recent?.output).toContain('Stub defender exclusion enabled');
	});
});
