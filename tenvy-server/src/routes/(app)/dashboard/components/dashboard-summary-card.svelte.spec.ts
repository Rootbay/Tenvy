import { page } from '@vitest/browser/context';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';

import DashboardSummaryCard from './dashboard-summary-card.svelte';

const percentageFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });

describe('dashboard-summary-card', () => {
	it('renders the primary dashboard metrics', async () => {
		render(DashboardSummaryCard, {
			props: {
				totals: { total: 128, connected: 96 },
				newClients: {
					today: { total: 7, deltaPercent: 0.12, series: [] },
					week: { total: 32, deltaPercent: -0.08, series: [] }
				},
				bandwidth: {
					totalMb: 5120,
					totalGb: 5,
					deltaPercent: 0.15,
					capacityMb: 0,
					usagePercent: 0,
					peakMb: 0,
					series: []
				},
				latency: {
					averageMs: 182.4,
					deltaMs: -12.3,
					series: []
				},
				percentageFormatter
			}
		});

		await expect.element(page.getByText('Total clients')).toBeInTheDocument();
		await expect.element(page.getByText('128')).toBeInTheDocument();
		await expect.element(page.getByText('New clients')).toBeInTheDocument();
		await expect.element(page.getByText('7')).toBeInTheDocument();
		await expect.element(page.getByText('Bandwidth usage')).toBeInTheDocument();
		await expect.element(page.getByText('5')).toBeInTheDocument();
		await expect.element(page.getByText('Latency')).toBeInTheDocument();
		await expect.element(page.getByText('182.4')).toBeInTheDocument();
	});
});
