import { page } from '@vitest/browser/context';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { get, writable } from 'svelte/store';

import type { DashboardCountryStat } from '$lib/data/dashboard';

import DashboardCountryList from './dashboard-country-list.svelte';

const percentageFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 1 });

const countries: DashboardCountryStat[] = [
	{
		countryCode: 'US',
		countryName: 'United States',
		flag: '🇺🇸',
		count: 10,
		onlineCount: 8,
		percentage: 52.4
	},
	{
		countryCode: 'CA',
		countryName: 'Canada',
		flag: '🇨🇦',
		count: 4,
		onlineCount: 3,
		percentage: 21.7
	}
];

describe('dashboard-country-list', () => {
	it('renders countries and toggles the selected country', async () => {
		const selectedCountry = writable<string | null>(null);
                const { unmount } = render(DashboardCountryList, {
                        countries,
                        selectedCountry,
                        percentageFormatter
                });

		await expect.element(page.getByText('United States')).toBeInTheDocument();
		await expect.element(page.getByText('21.7%')).toBeInTheDocument();

		await page.getByRole('button', { name: /Canada/ }).click();
		expect(get(selectedCountry)).toBe('CA');

		await page.getByRole('button', { name: /Canada/ }).click();
		expect(get(selectedCountry)).toBeNull();
                unmount();
        });
});
