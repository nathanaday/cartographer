import { describe, it, expect } from "vitest";
import { autoLayout, estimateCardHeight, CARD_W } from "./autolayout";
import type { Entity } from "../types/api";

const ent = (project: string, name: string): Entity => ({ project, name, aspects: [] });
const nodeId = (e: { project: string; name: string }) => `${e.project}/${e.name}`;

describe("autoLayout", () => {
  it("is deterministic and clusters by project", () => {
    const entities = [ent("b-proj", "Zeta"), ent("a-proj", "Beta"), ent("a-proj", "Alpha")];
    const a = autoLayout(entities, {});
    const b = autoLayout(entities, {});
    expect(a).toEqual(b);
    // a-proj is column 0; Alpha above Beta
    expect(a["a-proj/Alpha"].x).toBe(0);
    expect(a["a-proj/Alpha"].y).toBeLessThan(a["a-proj/Beta"].y);
    // b-proj is a later column
    expect(a["b-proj/Zeta"].x).toBeGreaterThan(0);
  });

  it("saved position overrides computed", () => {
    const entities = [ent("p", "A")];
    const out = autoLayout(entities, { "p/A": { x: 999, y: 888 } });
    expect(out["p/A"]).toEqual({ x: 999, y: 888 });
  });
});

const entWithFields = (project: string, name: string, fieldCounts: number[]): Entity => ({
  project,
  name,
  aspects: fieldCounts.map((n, i) => ({
    kind: `k${i}`,
    fields: Array.from({ length: n }, (_, j) => ({ name: `f${j}`, type: "string", nullable: false })),
  })),
});

describe("height-aware packing", () => {
  it("estimates taller cards for more content", () => {
    const small = estimateCardHeight(entWithFields("p", "A", []));
    const big = estimateCardHeight(entWithFields("p", "B", [13, 13, 12]));
    expect(big).toBeGreaterThan(small + 200);
  });

  it("stacks without overlap using estimated heights", () => {
    const entities = [
      entWithFields("p", "A", [13, 13, 12]), // tall
      entWithFields("p", "B", [2]),          // short
      entWithFields("p", "C", []),           // shell
    ];
    const out = autoLayout(entities, {});
    // with 3 entities the cluster may span one or two columns; the only
    // guaranteed invariant is that cards sharing a column don't overlap.
    const byCol = new Map<number, { name: string; y: number }[]>();
    for (const [id, e] of entities.map((e) => [nodeId(e), e] as const)) {
      const pos = out[id];
      const list = byCol.get(pos.x) ?? [];
      list.push({ name: e.name, y: pos.y });
      byCol.set(pos.x, list);
    }
    for (const list of byCol.values()) {
      list.sort((a, b) => a.y - b.y);
      for (let i = 0; i + 1 < list.length; i++) {
        const cur = entities.find((e) => e.name === list[i].name)!;
        expect(list[i + 1].y).toBeGreaterThanOrEqual(list[i].y + estimateCardHeight(cur));
      }
    }
  });
});

// genEntities builds `count` entities in `project`, each with a few aspects,
// named so sort order is stable (A00, A01, ...).
function genEntities(project: string, count: number): Entity[] {
  return Array.from({ length: count }, (_, i) =>
    entWithFields(project, `E${String(i).padStart(3, "0")}`, [3, 4]),
  );
}

describe("multi-column masonry", () => {
  it("wraps a large project into multiple columns", () => {
    const entities = genEntities("big", 40);
    const totalH = entities.reduce((sum, e) => sum + estimateCardHeight(e) + 40, 0);
    const out = autoLayout(entities, {});

    const xs = new Set(entities.map((e) => out[nodeId(e)].x));
    expect(xs.size).toBeGreaterThan(1);

    const maxColHeight = new Map<number, number>();
    for (const e of entities) {
      const pos = out[nodeId(e)];
      const bottom = pos.y + estimateCardHeight(e);
      maxColHeight.set(pos.x, Math.max(maxColHeight.get(pos.x) ?? 0, bottom));
    }
    const tallestCol = Math.max(...maxColHeight.values());
    expect(tallestCol).toBeLessThan(totalH / 2);
  });

  it("never overlaps two cards within the same column", () => {
    const entities = genEntities("big", 40);
    const out = autoLayout(entities, {});

    const byCol = new Map<number, { name: string; y: number }[]>();
    for (const e of entities) {
      const pos = out[nodeId(e)];
      const list = byCol.get(pos.x) ?? [];
      list.push({ name: e.name, y: pos.y });
      byCol.set(pos.x, list);
    }
    for (const list of byCol.values()) {
      list.sort((a, b) => a.y - b.y);
      for (let i = 0; i + 1 < list.length; i++) {
        const cur = entities.find((e) => e.name === list[i].name)!;
        expect(list[i + 1].y).toBeGreaterThanOrEqual(list[i].y + estimateCardHeight(cur));
      }
    }
  });

  it("is deterministic across repeated calls", () => {
    const entities = genEntities("big", 40);
    const a = autoLayout(entities, {});
    const b = autoLayout(entities, {});
    expect(a).toEqual(b);
  });

  it("offsets the next project beyond the first cluster's columns", () => {
    const entities = [...genEntities("p1-big", 40), ent("p2-small", "Only")];
    const out = autoLayout(entities, {});

    const p1Xs = genEntities("p1-big", 40).map((e) => out[nodeId(e)].x);
    const p1MaxX = Math.max(...p1Xs);
    const p2X = out["p2-small/Only"].x;

    expect(p2X).toBeGreaterThanOrEqual(p1MaxX + CARD_W);
  });
});
