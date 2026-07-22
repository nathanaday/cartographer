import type { Entity, XY } from "../types/api";

const COL_WIDTH = 380;
const ROW_HEIGHT = 220;
const CLUSTER_GAP = 120;

const nodeId = (e: { project: string; name: string }) => `${e.project}/${e.name}`;

// autoLayout places cards deterministically: one column cluster per project
// (projects left-to-right by slug, entities top-to-bottom by name). A saved
// position always wins over the computed one, so user arrangement is preserved.
export function autoLayout(entities: Entity[], saved: Record<string, XY>): Record<string, XY> {
  const byProject = new Map<string, Entity[]>();
  for (const e of entities) {
    const list = byProject.get(e.project) ?? [];
    list.push(e);
    byProject.set(e.project, list);
  }
  const projects = [...byProject.keys()].sort();

  const out: Record<string, XY> = {};
  let x = 0;
  for (const project of projects) {
    const list = byProject.get(project)!.slice().sort((a, b) => a.name.localeCompare(b.name));
    list.forEach((e, i) => {
      const id = nodeId(e);
      out[id] = saved[id] ?? { x, y: i * ROW_HEIGHT };
    });
    x += COL_WIDTH + CLUSTER_GAP;
  }
  return out;
}
