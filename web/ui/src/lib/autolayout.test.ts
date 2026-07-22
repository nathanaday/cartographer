import { describe, it, expect } from "vitest";
import { autoLayout, estimateCardHeight } from "./autolayout";
import type { Entity } from "../types/api";

const ent = (project: string, name: string): Entity => ({ project, name, aspects: [] });

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
    // sorted by name: A, B, C stacked in one column
    expect(out["p/B"].y).toBeGreaterThanOrEqual(out["p/A"].y + estimateCardHeight(entities[0]));
    expect(out["p/C"].y).toBeGreaterThanOrEqual(out["p/B"].y + estimateCardHeight(entities[1]));
  });
});
