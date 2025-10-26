import {
  triggerMonitorStatusSchema,
  triggerMonitorCommandRequestSchema,
} from '$lib/types/trigger-monitor';

interface FetchTriggerMonitorOptions {
  signal?: AbortSignal;
}

interface UpdateTriggerMonitorInput {
  feed: 'live' | 'batch';
  refreshSeconds: number;
  includeScreenshots: boolean;
  includeCommands: boolean;
  signal?: AbortSignal;
}

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

export async function updateTriggerMonitorConfig(agentId: string, input: UpdateTriggerMonitorInput) {
  const body = triggerMonitorCommandRequestSchema.parse({
    action: 'configure',
    config: {
      feed: input.feed,
      refreshSeconds: input.refreshSeconds,
      includeScreenshots: input.includeScreenshots,
      includeCommands: input.includeCommands,
    },
  });

  const response = await fetch(`/api/agents/${agentId}/misc/trigger-monitor`, {
    method: 'POST',
    signal: input.signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  if (!response.ok) {
    throw await parseError(response);
  }

  const data = await response.json();
  return triggerMonitorStatusSchema.parse(data);
}

