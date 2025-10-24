import { describe, expect, it } from 'vitest';

import { isLikelyPrivateIp } from './ip.js';

describe('isLikelyPrivateIp', () => {
	it('treats IPv4 link-local addresses as private', () => {
		expect(isLikelyPrivateIp('169.254.10.20')).toBe(true);
	});

	it('treats IPv4-mapped link-local IPv6 addresses as private', () => {
		expect(isLikelyPrivateIp('::ffff:169.254.42.99')).toBe(true);
	});

	it('does not treat public IPv4 addresses as private', () => {
		expect(isLikelyPrivateIp('8.8.8.8')).toBe(false);
	});
});
