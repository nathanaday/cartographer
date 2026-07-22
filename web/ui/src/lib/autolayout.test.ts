import { describe, it, expect } from "vitest";
import { autoLayout } from "./autolayout";
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
