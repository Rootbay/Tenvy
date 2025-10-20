#!/usr/bin/env bun

import { performance } from 'node:perf_hooks';

interface RemoteDesktopSessionResponse {
        session: {
                sessionId: string;
                active: boolean;
        } | null;
}

interface LoadSample {
        latency: number;
        success: boolean;
}

const baseUrl = process.env.TENVY_TEST_BASE_URL ?? 'http://localhost:3000';
const agentId = process.env.TENVY_TEST_AGENT_ID;

if (!agentId) {
        console.error('TENVY_TEST_AGENT_ID is required.');
        process.exit(1);
}

const iterations = Number.parseInt(process.env.TENVY_TEST_ITERATIONS ?? '50', 10);
const concurrency = Math.max(1, Number.parseInt(process.env.TENVY_TEST_CONCURRENCY ?? '4', 10));
const sessionOverride = process.env.TENVY_TEST_SESSION_ID ?? '';

async function resolveSessionId(): Promise<string> {
        if (sessionOverride.trim() !== '') {
                return sessionOverride.trim();
        }
        const response = await fetch(`${baseUrl}/api/agents/${agentId}/remote-desktop/session`, {
                headers: { Accept: 'application/json' }
        });
        if (!response.ok) {
                throw new Error(`Failed to fetch session state (status ${response.status})`);
        }
        const payload = (await response.json()) as RemoteDesktopSessionResponse;
        const sessionId = payload.session?.sessionId ?? '';
        if (!sessionId) {
                throw new Error('No active remote desktop session found.');
        }
        return sessionId;
}

function buildEvents(batch: number) {
        const now = Date.now();
        return Array.from({ length: batch }).map((_, index) => ({
                type: 'mouse-move',
                capturedAt: now + index,
                x: Math.random() * 1920,
                y: Math.random() * 1080,
                normalized: true
        }));
}

async function sendInput(sessionId: string): Promise<LoadSample> {
        const payload = {
                sessionId,
                events: buildEvents(12)
        };
        const start = performance.now();
        const response = await fetch(`${baseUrl}/api/agents/${agentId}/remote-desktop/input`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
        });
        const latency = performance.now() - start;
        return { latency, success: response.ok };
}

async function runLoadTest(sessionId: string) {
        const results: LoadSample[] = [];
        for (let i = 0; i < iterations; i += concurrency) {
                const batch = Array.from({ length: Math.min(concurrency, iterations - i) }).map(() => sendInput(sessionId));
                const resolved = await Promise.all(batch);
                results.push(...resolved);
        }
        return results;
}

function summarize(samples: LoadSample[]) {
        const successes = samples.filter((sample) => sample.success);
        const failures = samples.length - successes.length;
        const latencies = successes.map((sample) => sample.latency);
        const avg = latencies.reduce((acc, value) => acc + value, 0) / (latencies.length || 1);
        const sorted = [...latencies].sort((a, b) => a - b);
        const p95 = sorted.length ? sorted[Math.floor(sorted.length * 0.95) - 1] ?? sorted[sorted.length - 1] : 0;
        return { total: samples.length, failures, avgLatency: avg, p95Latency: p95 };
}

try {
        const sessionId = await resolveSessionId();
        console.log(`Using session ${sessionId} for agent ${agentId}. Running ${iterations} iterations...`);
        const samples = await runLoadTest(sessionId);
        const summary = summarize(samples);
        console.log('Load test summary:', summary);
        if (summary.failures > 0) {
                process.exitCode = 1;
        }
} catch (err) {
        console.error('Remote desktop load test failed:', err);
        process.exitCode = 1;
}
