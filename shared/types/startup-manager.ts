export type StartupScope = "machine" | "user" | "scheduled-task";

export type StartupSource = "registry" | "startup-folder" | "scheduled-task" | "service" | "other";

export type StartupImpact = "low" | "medium" | "high" | "not-measured";

export interface StartupTelemetrySummary {
  total: number;
  enabled: number;
  disabled: number;
  impactCounts: Partial<Record<StartupImpact, number>>;
  scopeCounts: Partial<Record<StartupScope, number>>;
}

export interface StartupEntry {
  id: string;
  name: string;
  path: string;
  arguments?: string;
  enabled: boolean;
  scope: StartupScope;
  source: StartupSource;
  impact: StartupImpact;
  publisher?: string;
  description?: string;
  location: string;
  startupTime: number;
  lastEvaluatedAt: string;
  lastRunAt?: string | null;
  metadata?: Record<string, unknown>;
}

export interface StartupInventoryResponse {
  entries: StartupEntry[];
  generatedAt: string;
  summary?: StartupTelemetrySummary;
}

export interface StartupEntryDefinition {
  name: string;
  path: string;
  arguments?: string;
  scope: StartupScope;
  source: StartupSource;
  location: string;
  enabled?: boolean;
  publisher?: string;
  description?: string;
}

export interface StartupListCommandRequest {
  operation: "list";
  refresh?: boolean;
}

export interface StartupToggleCommandRequest {
  operation: "toggle";
  entryId: string;
  enabled: boolean;
}

export interface StartupCreateCommandRequest {
  operation: "create";
  definition: StartupEntryDefinition;
}

export interface StartupRemoveCommandRequest {
  operation: "remove";
  entryId: string;
}

export type StartupCommandRequest =
  | StartupListCommandRequest
  | StartupToggleCommandRequest
  | StartupCreateCommandRequest
  | StartupRemoveCommandRequest;

export interface StartupCommandPayload {
  request: StartupCommandRequest;
}

export type StartupCommandResponse =
  | { operation: "list"; status: "ok"; result: StartupInventoryResponse }
  | { operation: "toggle"; status: "ok"; result: StartupEntry }
  | { operation: "create"; status: "ok"; result: StartupEntry }
  | { operation: "remove"; status: "ok"; result: { entryId: string } }
  | {
      operation: StartupCommandRequest["operation"];
      status: "error";
      error: string;
      code?: string;
      details?: unknown;
    };
