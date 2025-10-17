declare module 'node:quic' {
	import type { Buffer } from 'node:buffer';
	import { EventEmitter } from 'node:events';

	interface QuicEndpointOptions {
		address?: string;
		port?: number;
	}

	interface QuicSocketOptions {
		endpoint?: QuicEndpointOptions;
	}

	interface QuicServerListenOptions {
		key: string | Buffer;
		cert: string | Buffer;
		alpn?: string[];
	}

	interface QuicStream extends EventEmitter {
		setEncoding?(encoding: string): void;
	}

	type QuicServerSession = EventEmitter;

	interface QuicSocket extends EventEmitter {
		listen?(options: QuicServerListenOptions): Promise<void>;
	}

	export function createQuicSocket(options?: QuicSocketOptions): QuicSocket;
}
