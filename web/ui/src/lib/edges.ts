import type { Entity, FlowEdge } from "../types/api";

const nodeId = (e: { project: string; name: string }) => `${e.project}/${e.name}`;

// edgesFrom derives relationship edges from fields whose `references` names
// another entity ("OtherEntity" or "OtherEntity.field"). References resolve
// within the source entity's own project, since different projects may
// legitimately contain entities with the same name. A reference that does
// not resolve to an existing node in the same project is dropped (dangling
// refs are not drawn).
export function edgesFrom(entities: Entity[]): FlowEdge[] {
  const ids = new Set(entities.map((e) => nodeId(e)));

  const edges: FlowEdge[] = [];
  const seen = new Set<string>();
  for (const e of entities) {
    for (const a of e.aspects) {
      for (const f of a.fields) {
        if (!f.references) continue;
        const targetName = f.references.split(".")[0];
        const targetId = `${e.project}/${targetName}`;
        if (!ids.has(targetId)) continue;
        const id = `${nodeId(e)}->${targetId}`;
        if (seen.has(id)) continue;
        seen.add(id);
        edges.push({ id, source: nodeId(e), target: targetId });
      }
    }
  }
  return edges;
}
