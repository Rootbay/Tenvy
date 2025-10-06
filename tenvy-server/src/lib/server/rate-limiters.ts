import { RateLimiterMemory } from 'rate-limiter-flexible';

const voucherLimiter = new RateLimiterMemory({ points: 5, duration: 60 });
const webauthnLimiter = new RateLimiterMemory({ points: 10, duration: 60 });

async function consume(limiter: RateLimiterMemory, key: string) {
	try {
		await limiter.consume(key);
	} catch (error) {
		throw Object.assign(new Error('Too many attempts. Please slow down.'), { status: 429 });
	}
}

export async function limitVoucherRedeem(key: string) {
	await consume(voucherLimiter, key);
}

export async function limitWebAuthn(key: string) {
	await consume(webauthnLimiter, key);
}
