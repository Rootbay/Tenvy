import { beforeAll, describe, expect, it, vi } from 'vitest';
import type { RemoteDesktopInputEvent } from '$lib/types/remote-desktop';

vi.mock('$app/environment', () => ({
	browser: true,
	dev: false,
	building: false
}));

let createInputChannel: (typeof import('./input-channel'))['createInputChannel'];

beforeAll(async () => {
	({ createInputChannel } = await import('./input-channel'));
});

describe('createInputChannel', () => {
	it('requeues events when dispatch resolves false', async () => {
		const callbacks: ((time: number) => void)[] = [];
		const raf = vi.fn<(cb: (time: number) => void) => number>((cb) => {
			callbacks.push(cb);
			return callbacks.length;
		});
		const cancelRaf = vi.fn();
		const onDispatchFailure = vi.fn();
		const event: RemoteDesktopInputEvent = {
			type: 'mouse-move',
			capturedAt: Date.now(),
			x: 1,
			y: 2,
			normalized: false
		};

		const dispatch = vi
			.fn<(events: RemoteDesktopInputEvent[]) => Promise<boolean>>()
			.mockResolvedValueOnce(false)
			.mockResolvedValue(true);

		const channel = createInputChannel({
			dispatch,
			onDispatchFailure,
			raf,
			cancelRaf
		});

		channel.enqueue(event);

		expect(raf).toHaveBeenCalledTimes(1);
		expect(callbacks).toHaveLength(1);

		callbacks.shift()?.(0);
		await Promise.resolve();

		expect(dispatch).toHaveBeenCalledTimes(1);
		expect(onDispatchFailure).toHaveBeenCalledTimes(1);
		expect(onDispatchFailure.mock.calls[0]?.[0]).toHaveLength(1);
		expect(raf).toHaveBeenCalledTimes(2);
		expect(callbacks).toHaveLength(1);

		callbacks.shift()?.(16);
		await Promise.resolve();

		expect(dispatch).toHaveBeenCalledTimes(2);
		expect(onDispatchFailure).toHaveBeenCalledTimes(1);
		expect(callbacks).toHaveLength(0);
		expect(dispatch.mock.calls[0]?.[0]).toHaveLength(1);
		expect(dispatch.mock.calls[1]?.[0]).toHaveLength(1);
		expect(dispatch.mock.calls[1]?.[0]?.[0]).toBe(dispatch.mock.calls[0]?.[0]?.[0]);
	});
});
