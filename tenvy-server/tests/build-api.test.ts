import { describe, expect, it } from 'vitest';
import type { HttpError } from '@sveltejs/kit';
import { normalizeBuildRequestPayload } from '../src/routes/api/build/normalizer.js';

describe('normalizeBuildRequestPayload', () => {
	it('normalizes a minimal valid payload', () => {
		const payload = {
			host: 'controller.tenvy.local',
			port: 2444,
			outputFilename: 'tenvy-agent',
			outputExtension: '.exe',
			targetOS: 'windows',
			targetArch: 'amd64',
			installationPath: 'C\\\\ProgramData\\\\Tenvy',
			meltAfterRun: true,
			startupOnBoot: false,
			developerMode: false,
			mutexName: 'Global\\tenvy-test',
			compressBinary: true,
			forceAdmin: false
		};

		const normalized = normalizeBuildRequestPayload(payload);

		expect(normalized.host).toBe('controller.tenvy.local');
		expect(normalized.port).toBe('2444');
		expect(normalized.outputExtension).toBe('.exe');
		expect(normalized.outputFilename).toBe('tenvy-agent.exe');
		expect(normalized.targetOS).toBe('windows');
		expect(normalized.targetArch).toBe('amd64');
	});

	it('rejects unsupported extensions for the target OS', () => {
		try {
			normalizeBuildRequestPayload({
				host: 'example.local',
				outputFilename: 'payload',
				outputExtension: '.msi',
				targetOS: 'windows',
				targetArch: 'amd64'
			});
			throw new Error('Expected payload validation to fail');
		} catch (error) {
			const err = error as HttpError;
			expect(err.status).toBe(400);
			const message =
				typeof err.body === 'object' && err.body && 'message' in err.body
					? String(err.body.message)
					: String(err);
			expect(message).toContain('Extension .msi is not supported');
		}
	});

        it('rejects payloads containing unsupported fields', () => {
                try {
                        normalizeBuildRequestPayload({
                                host: 'example.local',
                                binder: { enabled: true }
                        });
                        throw new Error('Expected payload validation to fail');
                } catch (error) {
                        const err = error as HttpError;
                        expect(err.status).toBe(400);
                        const message =
                                typeof err.body === 'object' && err.body && 'message' in err.body
                                        ? String(err.body.message)
                                        : String(err);
                        expect(message).toContain('binder');
                }
        });

        it('rejects ports outside the allowed range', () => {
                try {
                        normalizeBuildRequestPayload({
                                host: 'example.local',
                                port: 70000
                        });
                        throw new Error('Expected payload validation to fail');
                } catch (error) {
                        const err = error as HttpError;
                        expect(err.status).toBe(400);
                        const message =
                                typeof err.body === 'object' && err.body && 'message' in err.body
                                        ? String(err.body.message)
                                        : String(err);
                        expect(message).toContain('Port must be between 1 and 65535');
                }
        });
});
