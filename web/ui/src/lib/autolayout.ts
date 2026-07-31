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

// estimateCardHeight approximates a card's rendered height. Strips are
// collapsed by default, so only the strip the kind filter expands contributes
// field rows — the packer reserves what the card actually draws.
export function estimateCardHeight(e: Entity, kindFilter: string | null = null): number {
  let h = TITLE_H + CARD_PAD;
  if (e.aspects.length === 0) return h + EMPTY_STATE_H;
  for (const a of e.aspects) {
    h += STRIP_HEADER_H;
    if (a.kind === kindFilter) h += a.fields.length * FIELD_ROW_H;
  }
  return h;
}

// autoLayout places cards deterministically: one masonry cluster per project
// (projects left-to-right by slug, entities top-to-bottom by name). Each
// cluster wraps into enough columns to form a roughly square block, with
// entities greedily placed in the shortest column so tall expanded cards
// don't overlap. A saved position always wins over the computed one, so
// user arrangement is preserved.
export function autoLayout(
  entities: Entity[],
  saved: Record<string, XY>,
  kindFilter: string | null = null,
): Record<string, XY> {
  const byProject = new Map<string, Entity[]>();
  for (const e of entities) {
    const list = byProject.get(e.project) ?? [];
    list.push(e);
    byProject.set(e.project, list);
  }
  const projects = [...byProject.keys()].sort();

  const out: Record<string, XY> = {};
  let clusterX = 0;
  for (const project of projects) {
    const list = byProject.get(project)!.slice().sort((a, b) => a.name.localeCompare(b.name));

    const totalH = list.reduce((sum, e) => sum + estimateCardHeight(e, kindFilter) + CARD_GAP, 0);
    const ncols = Math.max(1, Math.ceil(Math.sqrt(totalH / COL_WIDTH)));
    const colHeights = new Array(ncols).fill(0);

    for (const e of list) {
      let col = 0;
      for (let i = 1; i < ncols; i++) {
        if (colHeights[i] < colHeights[col]) col = i;
      }
      const id = nodeId(e);
      const x = clusterX + col * COL_WIDTH;
      const y = colHeights[col];
      out[id] = saved[id] ?? { x, y };
      colHeights[col] += estimateCardHeight(e, kindFilter) + CARD_GAP;
    }

    clusterX += ncols * COL_WIDTH + CLUSTER_GAP;
  }
  return out;
}
