import { paraglideVitePlugin } from '@inlang/paraglide-js';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';
import type { AliasOptions } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { fileURLToPath } from 'node:url';

function resolvePort(value?: string | null): number {
	if (!value) {
		return 2332;
	}

	const parsed = Number.parseInt(value, 10);
	if (Number.isNaN(parsed) || parsed <= 0 || parsed > 65_535) {
		return 2332;
	}

	return parsed;
}

function resolveHost(value?: string | null): string | boolean {
	if (!value) {
		return '0.0.0.0';
	}

	const trimmed = value.trim();
	if (trimmed === '') {
		return '0.0.0.0';
	}

	if (trimmed.toLowerCase() === 'true') {
		return true;
	}

	return trimmed;
}

const serverPort = resolvePort(process.env.TENVY_SERVER_PORT ?? process.env.PORT ?? null);
const serverHost = resolveHost(process.env.TENVY_SERVER_HOST ?? process.env.HOST ?? null);
const isVitest = process.env.VITEST === 'true';

const testAliases: AliasOptions = isVitest
        ? [
                        {
                                find: '$env/dynamic/private',
                                replacement: fileURLToPath(
                                        new URL('./tests/mocks/env-dynamic-private.ts', import.meta.url)
                                )
                        }
                ]
        : [];

const enableBrowserTests = process.env.ENABLE_BROWSER_TESTS === 'true';

const serverTestConfig = {
	environment: 'node' as const,
	include: ['src/**/*.{test,spec}.{js,ts}', 'tests/**/*.{test,spec}.{js,ts}'],
	exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
};

const browserProjects = [
	{
		test: {
			name: 'client',
			environment: 'browser' as const,
			browser: {
				enabled: true,
				provider: 'playwright',
				instances: [{ browser: 'chromium' }]
			},
			include: ['src/**/*.svelte.{test,spec}.{js,ts}'],
			exclude: ['src/lib/server/**'],
			setupFiles: ['./vitest-setup-client.ts']
		}
	},
	{
		test: {
			name: 'server',
			...serverTestConfig
		}
	}
];

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit(),
		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide'
		})
	],
	server: {
		host: serverHost,
		port: serverPort,
		strictPort: true
	},
	preview: {
		host: typeof serverHost === 'string' ? serverHost : '0.0.0.0',
		port: serverPort,
		strictPort: true
	},
        resolve: {
                alias: testAliases
        },
	test: enableBrowserTests
		? {
				expect: { requireAssertions: true },
				projects: browserProjects
			}
		: {
				expect: { requireAssertions: true },
				...serverTestConfig
			}
});
