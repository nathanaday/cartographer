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

describe("regions track the packed view", () => {
  const withFields = (name: string): Entity => ({
    project: "p",
    name,
    aspects: [
      { kind: "sql", fields: Array.from({ length: 12 }, (_, i) => ({ name: `f${i}`, type: "string", nullable: false })) },
    ],
  });
  const entities = [withFields("A")];
  const projects = [{ slug: "p", name: "P" }];
  const positions = { "p/A": { x: 0, y: 0 } };

  it("is shorter with collapsed strips than with a filter expanding them", () => {
    const collapsed = projectRegions(entities, positions, projects, null)[0];
    const expanded = projectRegions(entities, positions, projects, "sql")[0];
    expect(collapsed.h).toBeLessThan(expanded.h);
  });

  it("actually contains the card's estimated bottom edge under each filter", () => {
    // The trivial "collapsed < expanded" check above doesn't prove the box
    // fits the card — a region that's merely shorter, but still too short
    // to reach the card's bottom edge, would also pass it. Check containment
    // and that the growth matches the field rows the filter reserves.
    const collapsed = projectRegions(entities, positions, projects, null)[0];
    const expanded = projectRegions(entities, positions, projects, "sql")[0];

    expect(collapsed.y + collapsed.h).toBeGreaterThanOrEqual(
      positions["p/A"].y + estimateCardHeight(entities[0], null),
    );
    expect(expanded.y + expanded.h).toBeGreaterThanOrEqual(
      positions["p/A"].y + estimateCardHeight(entities[0], "sql"),
    );
    expect(expanded.h - collapsed.h).toBe(
      estimateCardHeight(entities[0], "sql") - estimateCardHeight(entities[0], null),
    );
  });
});
