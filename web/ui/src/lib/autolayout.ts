import type { Entity, XY } from "../types/api";

export const CARD_W = 340;
const COL_WIDTH = 380;
const CLUSTER_GAP = 160;
const CARD_GAP = 40;

const TITLE_H = 68; // title block: name + badges + one description line
const STRIP_HEADER_H = 27; // aspect strip header row
const FIELD_ROW_H = 22; // one field row at full tier
const EMPTY_STATE_H = 26; // "no representations recorded" row
const CARD_PAD = 10;

const nodeId = (e: { project: string; name: string }) => `${e.project}/${e.name}`;

// estimateCardHeight approximates a card's full-tier height from its content,
// so packing leaves room even when every strip is expanded.
export function estimateCardHeight(e: Entity): number {
  let h = TITLE_H + CARD_PAD;
  if (e.aspects.length === 0) return h + EMPTY_STATE_H;
  for (const a of e.aspects) h += STRIP_HEADER_H + a.fields.length * FIELD_ROW_H;
  return h;
}

// autoLayout places cards deterministically: one column cluster per project
// (projects left-to-right by slug, entities top-to-bottom by name), stacked
// by accumulated estimated height so tall expanded cards don't overlap. A
// saved position always wins over the computed one, so user arrangement is
// preserved.
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
    let y = 0;
    for (const e of list) {
      const id = nodeId(e);
      out[id] = saved[id] ?? { x, y };
      y += estimateCardHeight(e) + CARD_GAP;
    }
    x += COL_WIDTH + CLUSTER_GAP;
  }
  return out;
}
