export type ClientToolTarget = "_self" | "_blank" | "dialog";

export interface ClientToolDefinition {
  id: string;
  title: string;
  description: string;
  segments: string[];
  target?: ClientToolTarget;
}
