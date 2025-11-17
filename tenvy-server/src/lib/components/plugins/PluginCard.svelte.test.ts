import { render, fireEvent } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';

import PluginCard from './PluginCard.svelte';
import type { Plugin } from '$lib/data/plugin-view.js';

const basePlugin: Plugin = {
	id: 'plugin-1',
	name: 'Endpoint monitor',
	description: 'Collects telemetry from endpoints.',
	version: '1.0.0',
	author: 'Example',
	category: 'operations',
	status: 'active',
	enabled: true,
	autoUpdate: false,
	installations: 24,
	lastDeployed: '2024-01-01',
	lastChecked: '2024-01-02',
	size: '12 MB',
	capabilities: ['Collect telemetry'],
	artifact: 'endpoint-monitor.zip',
	runtime: {
		type: 'native',
		sandboxed: false
	},
	distribution: {
		defaultMode: 'manual',
		allowManualPush: true,
		allowAutoSync: true,
		manualTargets: 4,
		autoTargets: 12,
		lastManualPush: '2024-01-03',
		lastAutoSync: '2024-01-04'
	},
	dependencies: [],
	requiredModules: [],
	approvalStatus: 'approved',
	signature: {
		status: 'trusted',
		trusted: true,
		type: 'ed25519',
		hash: 'abc',
		signer: 'Signer',
		publicKey: 'public-key',
		signedAt: '2024-01-01T00:00:00.000Z',
		checkedAt: '2024-01-02T00:00:00.000Z'
	}
};

describe('PluginCard', () => {
	it('triggers update when toggling plugin enabled state', async () => {
		const updatePlugin = vi.fn();
		const { getByRole } = render(PluginCard, {
			plugin: { ...basePlugin },
			updatePlugin,
			distributionNotice: vi.fn(() => 'Distribution summary')
		});

		const toggle = getByRole('switch', { name: `Toggle ${basePlugin.name}` });
		await fireEvent.click(toggle);

		expect(updatePlugin).toHaveBeenCalledWith(basePlugin.id, {
			enabled: false,
			status: 'disabled'
		});
	});

	it('enables automatic updates via the toggle', async () => {
		const updatePlugin = vi.fn();
		const { getByRole } = render(PluginCard, {
			plugin: { ...basePlugin },
			updatePlugin,
			distributionNotice: vi.fn(() => 'Distribution summary')
		});

		const toggle = getByRole('switch', {
			name: `Toggle auto update for ${basePlugin.name}`
		});
		await fireEvent.click(toggle);

		expect(updatePlugin).toHaveBeenCalledWith(basePlugin.id, { autoUpdate: true });
	});

	it('switches distribution mode when selecting automatic sync', async () => {
		const updatePlugin = vi.fn();
		const { getByRole } = render(PluginCard, {
			plugin: { ...basePlugin },
			updatePlugin,
			distributionNotice: vi.fn(() => 'Distribution summary')
		});

		const button = getByRole('button', { name: 'Automatic sync' });
		await fireEvent.click(button);

		expect(updatePlugin).toHaveBeenCalledWith(basePlugin.id, {
			distribution: { defaultMode: 'automatic' }
		});
	});

	it('renders the provided distribution notice helper', () => {
		const updatePlugin = vi.fn();
		const distributionNotice = vi.fn(() => 'Custom distribution notice');
		const { getByText } = render(PluginCard, {
			plugin: { ...basePlugin },
			updatePlugin,
			distributionNotice
		});

		expect(getByText('Custom distribution notice')).toBeTruthy();
		expect(distributionNotice).toHaveBeenCalled();
	});
});
