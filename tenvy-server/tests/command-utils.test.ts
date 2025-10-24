import { describe, expect, it } from 'vitest';
import { splitCommandLine } from '../../shared/utils/command.ts';

describe('splitCommandLine', () => {
	it('preserves empty arguments created by double quotes', () => {
		expect(splitCommandLine(['cmd', '""', 'arg'].join(' '))).toEqual(['cmd', '', 'arg']);
	});

	it('preserves multiple adjacent empty arguments', () => {
		expect(splitCommandLine(['""', '""'].join(' '))).toEqual(['', '']);
	});
});
