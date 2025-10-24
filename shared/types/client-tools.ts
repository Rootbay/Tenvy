export type ClientToolTarget = "_self" | "_blank" | "dialog";

export interface ClientToolDefinition {
  id: string;
  title: string;
  segments: string[];
  description?: string;
  target?: ClientToolTarget;
}
