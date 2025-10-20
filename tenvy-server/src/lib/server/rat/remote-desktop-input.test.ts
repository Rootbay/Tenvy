import { describe, expect, it } from 'vitest';

import { createQuicTokenMatcher, parseQuicTokenConfiguration } from './remote-desktop-input';

describe('parseQuicTokenConfiguration', () => {
	it('returns an empty array for falsy values', () => {
		expect(parseQuicTokenConfiguration(undefined)).toEqual([]);
		expect(parseQuicTokenConfiguration(null)).toEqual([]);
		expect(parseQuicTokenConfiguration('')).toEqual([]);
	});

	it('normalizes comma or whitespace separated strings', () => {
		expect(parseQuicTokenConfiguration(' alpha, beta  ,gamma ')).toEqual([
			'alpha',
			'beta',
			'gamma'
		]);
		expect(parseQuicTokenConfiguration(' one\n two\tthree ')).toEqual(['one', 'two', 'three']);
	});

	it('deduplicates tokens and trims array entries', () => {
		expect(parseQuicTokenConfiguration([' foo ', 'bar', 'foo', ''])).toEqual(['foo', 'bar']);
	});
});

describe('createQuicTokenMatcher', () => {
	it('allows any token when none are configured', () => {
		const matcher = createQuicTokenMatcher([]);
		expect(matcher('')).toBe(true);
		expect(matcher(null)).toBe(true);
		expect(matcher('anything')).toBe(true);
	});

	it('validates tokens using a constant-time comparison', () => {
		const matcher = createQuicTokenMatcher(['secret-token', 'another']);
		expect(matcher('secret-token')).toBe(true);
		expect(matcher('another')).toBe(true);
		expect(matcher('SECRET-TOKEN')).toBe(false);
		expect(matcher('')).toBe(false);
		expect(matcher(null)).toBe(false);
	});

	it('ignores surrounding whitespace in the presented token', () => {
		const matcher = createQuicTokenMatcher(['trim-me']);
		expect(matcher(' trim-me ')).toBe(true);
	});
});
