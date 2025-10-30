import { describe, expect, it } from 'vitest';
import { agentModules } from '../../../../../shared/modules/index.js';
import { buildRuntimeConfigPayload, normalizeBuildRequestPayload } from './normalizer.js';

const [firstModule, secondModule] = agentModules;

describe('normalizeBuildRequestPayload module handling', () => {
	it('normalizes duplicate selections and preserves order', () => {
		const selections = [firstModule?.id ?? 'remote-desktop'];
		selections.push(firstModule?.id ?? 'remote-desktop');
		if (secondModule) {
			selections.push(secondModule.id);
		}

		const normalized = normalizeBuildRequestPayload({
			host: 'controller.tenvy.local',
			modules: selections
		});

		const expected = [firstModule?.id ?? 'remote-desktop'];
		if (secondModule) {
			expected.push(secondModule.id);
		}

		expect(normalized.modules).toEqual(expected);

		const runtime = buildRuntimeConfigPayload(normalized);
		expect(runtime?.modules).toEqual(normalized.modules);
	});

	it('omits modules from the runtime payload when not provided', () => {
		const normalized = normalizeBuildRequestPayload({ host: 'controller.tenvy.local' });
		expect(normalized.modules).toEqual([]);
		const runtime = buildRuntimeConfigPayload(normalized);
		expect(runtime).toBeNull();
	});
});
