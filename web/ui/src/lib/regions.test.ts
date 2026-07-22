import { describe, it, expect } from "vitest";
import { projectRegions } from "./regions";
import { CARD_W, estimateCardHeight } from "./autolayout";
import type { Entity } from "../types/api";

const ent = (project: string, name: string): Entity => ({ project, name, aspects: [] });

describe("projectRegions", () => {
  it("bounds a project's cards with padding", () => {
    const entities = [ent("p", "A"), ent("p", "B")];
    const positions = { "p/A": { x: 0, y: 0 }, "p/B": { x: 100, y: 300 } };
    const [r] = projectRegions(entities, positions, [{ slug: "p", name: "p" }]);
    expect(r.x).toBeLessThan(0);
    expect(r.y).toBeLessThan(0);
    expect(r.x + r.w).toBeGreaterThanOrEqual(100 + CARD_W);
    expect(r.y + r.h).toBeGreaterThanOrEqual(300 + estimateCardHeight(entities[1]));
  });

  it("skips projects with no positioned cards", () => {
    expect(projectRegions([], {}, [{ slug: "ghost", name: "ghost" }])).toEqual([]);
  });
});
