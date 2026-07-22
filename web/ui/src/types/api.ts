export interface Field {
  name: string;
  type: string;
  nullable: boolean;
  tags?: string[];
  references?: string;
}
export interface Aspect {
  kind: string;
  source_ref?: string;
  fields: Field[];
}
export interface Entity {
  project: string;
  name: string;
  description?: string;
  aspects: Aspect[];
}
export interface ProjectRef {
  slug: string;
  name: string;
}
export interface MapResponse {
  fingerprint: string;
  projects: ProjectRef[];
  entities: Entity[];
}
export interface XY {
  x: number;
  y: number;
}
export interface FlowEdge {
  id: string;
  source: string;
  target: string;
}
