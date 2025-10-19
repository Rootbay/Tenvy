import type { AgentSnapshot } from './agent';
import type { NoteEnvelope } from './notes';
import type { Command, CommandDeliveryMode } from './messages';

export type AgentRegistryEvent =
  | { type: 'agents'; agents: AgentSnapshot[] }
  | { type: 'agent'; agent: AgentSnapshot }
  | { type: 'notes'; agentId: string; notes: NoteEnvelope[] }
  | { type: 'command'; agentId: string; command: Command; delivery: CommandDeliveryMode };
