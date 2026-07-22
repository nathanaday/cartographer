import type { Entity, FlowEdge } from "../types/api";

const nodeId = (e: { project: string; name: string }) => `${e.project}/${e.name}`;

// edgesFrom derives relationship edges from fields whose `references` names
// another entity ("OtherEntity" or "OtherEntity.field"). A reference to an
// entity that does not exist in the map is dropped (dangling refs are not drawn).
export function edgesFrom(entities: Entity[]): FlowEdge[] {
  const byName = new Map<string, Entity>();
  for (const e of entities) byName.set(e.name, e);

  const edges: FlowEdge[] = [];
  const seen = new Set<string>();
  for (const e of entities) {
    for (const a of e.aspects) {
      for (const f of a.fields) {
        if (!f.references) continue;
        const targetName = f.references.split(".")[0];
        const target = byName.get(targetName);
        if (!target) continue;
        const id = `${nodeId(e)}->${nodeId(target)}`;
        if (seen.has(id)) continue;
        seen.add(id);
        edges.push({ id, source: nodeId(e), target: nodeId(target) });
      }
    }
  }
  return edges;
}
