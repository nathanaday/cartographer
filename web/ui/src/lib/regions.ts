import { CARD_W, estimateCardHeight } from "./autolayout";
import type { Entity, XY } from "../types/api";

export interface Region { slug: string; label: string; x: number; y: number; w: number; h: number }

const PAD = 24;

// projectRegions computes a display-only bounding box per project from its
// cards' positions and estimated sizes.
export function projectRegions(
  entities: Entity[],
  positions: Record<string, XY>,
  projects: { slug: string; name: string }[],
  kindFilter: string | null = null,
): Region[] {
  const out: Region[] = [];
  for (const p of projects) {
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const e of entities) {
      if (e.project !== p.slug) continue;
      const pos = positions[`${p.slug}/${e.name}`];
      if (!pos) continue;
      minX = Math.min(minX, pos.x);
      minY = Math.min(minY, pos.y);
      maxX = Math.max(maxX, pos.x + CARD_W);
      maxY = Math.max(maxY, pos.y + estimateCardHeight(e, kindFilter));
    }
    if (minX === Infinity) continue;
    out.push({
      slug: p.slug,
      label: p.name || p.slug,
      x: minX - PAD,
      y: minY - PAD,
      w: maxX - minX + PAD * 2,
      h: maxY - minY + PAD * 2,
    });
  }
  return out;
}
