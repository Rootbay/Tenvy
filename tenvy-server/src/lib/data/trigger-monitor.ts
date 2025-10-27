import {
  triggerMonitorStatusSchema,
  triggerMonitorCommandRequestSchema,
} from '$lib/types/trigger-monitor';
import type { TriggerMonitorConfigInput } from '$lib/types/trigger-monitor';

interface FetchTriggerMonitorOptions {
  signal?: AbortSignal;
}

type UpdateTriggerMonitorInput = TriggerMonitorConfigInput & {
  signal?: AbortSignal;
};

async function parseError(response: Response) {
  let message = response.statusText || 'Request failed';
  try {
    const payload = (await response.json()) as { message?: string; error?: string };
    message = payload?.message || payload?.error || message;
  } catch {
    // ignore JSON parse errors
  }
  return new Error(message);
}

export async function fetchTriggerMonitorStatus(agentId: string, options: FetchTriggerMonitorOptions = {}) {
  const response = await fetch(`/api/agents/${agentId}/misc/trigger-monitor`, {
    signal: options.signal,
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  const data = await response.json();
  return triggerMonitorStatusSchema.parse(data);
}

export async function updateTriggerMonitorConfig(
  agentId: string,
  input: UpdateTriggerMonitorInput,
) {
  const { signal, ...config } = input;
  const body = triggerMonitorCommandRequestSchema.parse({
    action: 'configure',
    config,
  });

  const response = await fetch(`/api/agents/${agentId}/misc/trigger-monitor`, {
    method: 'POST',
    signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw await parseError(response);
  }

  const data = await response.json();
  return triggerMonitorStatusSchema.parse(data);
}

