import {
  registryListResultSchema,
  registryMutationResultSchema,
  type RegistryHiveName,
  type RegistryValueInput,
} from '$lib/types/registry';

interface FetchRegistryOptions {
  hive?: RegistryHiveName;
  path?: string;
  depth?: number;
  signal?: AbortSignal;
}

interface CreateKeyInput {
  hive: RegistryHiveName;
  parentPath?: string | null;
  name: string;
  signal?: AbortSignal;
}

interface CreateValueInput {
  hive: RegistryHiveName;
  keyPath: string;
  value: RegistryValueInput;
  signal?: AbortSignal;
}

interface UpdateKeyInput {
  hive: RegistryHiveName;
  path: string;
  name: string;
  signal?: AbortSignal;
}

interface UpdateValueInput {
  hive: RegistryHiveName;
  keyPath: string;
  value: RegistryValueInput;
  originalName?: string | null;
  signal?: AbortSignal;
}

interface DeleteKeyInput {
  hive: RegistryHiveName;
  path: string;
  signal?: AbortSignal;
}

interface DeleteValueInput {
  hive: RegistryHiveName;
  keyPath: string;
  name: string;
  signal?: AbortSignal;
}

async function parseError(response: Response): Promise<Error> {
  let message = response.statusText || 'Request failed';
  try {
    const payload = (await response.json()) as { message?: string; error?: string };
    message = payload?.message || payload?.error || message;
  } catch {
    // ignore json parse errors
  }
  return new Error(message);
}

export async function fetchRegistrySnapshot(
  agentId: string,
  options: FetchRegistryOptions = {}
) {
  const params = new URLSearchParams();
  if (options.hive) {
    params.set('hive', options.hive);
  }
  if (options.path) {
    params.set('path', options.path);
  }
  if (typeof options.depth === 'number') {
    params.set('depth', String(options.depth));
  }
  const response = await fetch(
    `/api/agents/${agentId}/registry${params.size > 0 ? `?${params.toString()}` : ''}`,
    { signal: options.signal }
  );
  if (!response.ok) {
    throw await parseError(response);
  }
  const data = await response.json();
  const parsed = registryListResultSchema.parse(data);
  return parsed;
}

async function requestRegistryMutation(
  agentId: string,
  method: 'POST' | 'PATCH' | 'DELETE',
  body: unknown,
  signal?: AbortSignal
) {
  const response = await fetch(`/api/agents/${agentId}/registry`, {
    method,
    signal,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body ?? {}),
  });
  if (!response.ok) {
    throw await parseError(response);
  }
  const data = await response.json();
  const parsed = registryMutationResultSchema.parse(data);
  return parsed;
}

export async function createRegistryKey(agentId: string, input: CreateKeyInput) {
  return requestRegistryMutation(
    agentId,
    'POST',
    {
      operation: 'create',
      target: 'key',
      hive: input.hive,
      parentPath: input.parentPath ?? undefined,
      name: input.name,
    },
    input.signal
  );
}

export async function createRegistryValue(agentId: string, input: CreateValueInput) {
  return requestRegistryMutation(
    agentId,
    'POST',
    {
      operation: 'create',
      target: 'value',
      hive: input.hive,
      keyPath: input.keyPath,
      value: input.value,
    },
    input.signal
  );
}

export async function updateRegistryKey(agentId: string, input: UpdateKeyInput) {
  return requestRegistryMutation(
    agentId,
    'PATCH',
    {
      operation: 'update',
      target: 'key',
      hive: input.hive,
      path: input.path,
      name: input.name,
    },
    input.signal
  );
}

export async function updateRegistryValue(agentId: string, input: UpdateValueInput) {
  return requestRegistryMutation(
    agentId,
    'PATCH',
    {
      operation: 'update',
      target: 'value',
      hive: input.hive,
      keyPath: input.keyPath,
      value: input.value,
      originalName: input.originalName ?? undefined,
    },
    input.signal
  );
}

export async function deleteRegistryKey(agentId: string, input: DeleteKeyInput) {
  return requestRegistryMutation(
    agentId,
    'DELETE',
    {
      operation: 'delete',
      target: 'key',
      hive: input.hive,
      path: input.path,
    },
    input.signal
  );
}

export async function deleteRegistryValue(agentId: string, input: DeleteValueInput) {
  return requestRegistryMutation(
    agentId,
    'DELETE',
    {
      operation: 'delete',
      target: 'value',
      hive: input.hive,
      keyPath: input.keyPath,
      name: input.name,
    },
    input.signal
  );
}
