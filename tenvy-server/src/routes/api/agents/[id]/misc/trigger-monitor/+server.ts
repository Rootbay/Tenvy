import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import {
  dispatchTriggerMonitorCommand,
  TriggerMonitorAgentError,
} from '$lib/server/rat/trigger-monitor';
import {
  triggerMonitorCommandRequestSchema,
  type TriggerMonitorStatus,
} from '$lib/types/trigger-monitor';

export const GET: RequestHandler = async ({ params, locals }) => {
  const id = params.id;
  if (!id) {
    throw error(400, 'Missing agent identifier');
  }

  requireViewer(locals.user);

  try {
    const status = await dispatchTriggerMonitorCommand(id, { action: 'status' });
    return json(status satisfies TriggerMonitorStatus);
  } catch (err) {
    if (err instanceof TriggerMonitorAgentError) {
      throw error(err.status, err.message);
    }
    throw error(500, 'Failed to load trigger monitor status');
  }
};

export const POST: RequestHandler = async ({ params, request, locals }) => {
  const id = params.id;
  if (!id) {
    throw error(400, 'Missing agent identifier');
  }

  const user = requireOperator(locals.user);

  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    throw error(400, 'Invalid trigger monitor payload');
  }

  const command = triggerMonitorCommandRequestSchema.parse(payload);
  if (command.action !== 'configure') {
    throw error(405, 'Unsupported trigger monitor operation');
  }

  const normalized = {
    ...command,
    config: {
      ...command.config,
      refreshSeconds: Math.max(1, Math.min(command.config.refreshSeconds, 3600)),
    },
  } as typeof command;

  try {
    const status = await dispatchTriggerMonitorCommand(id, normalized, {
      operatorId: user.id,
    });
    return json(status satisfies TriggerMonitorStatus);
  } catch (err) {
    if (err instanceof TriggerMonitorAgentError) {
      throw error(err.status, err.message);
    }
    throw error(500, 'Failed to update trigger monitor configuration');
  }
};

