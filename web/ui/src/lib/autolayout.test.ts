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
  it("estimates taller cards for more aspects, even collapsed", () => {
    const small = estimateCardHeight(entWithFields("p", "A", []));
    const big = estimateCardHeight(entWithFields("p", "B", [13, 13, 12]));
    expect(big).toBeGreaterThan(small);
  });

  it("estimates taller cards for more content in the filtered aspect", () => {
    const small = estimateCardHeight(entWithFields("p", "A", [0]), "k0");
    const big = estimateCardHeight(entWithFields("p", "B", [13]), "k0");
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

describe("expansion-aware height estimation", () => {
  it("excludes field rows with no filter active", () => {
    // two strips with 10 fields each cost exactly what two empty strips cost:
    // headers only, no field rows reserved
    const loaded = estimateCardHeight(entWithFields("p", "A", [10, 10]));
    const bare = estimateCardHeight(entWithFields("p", "B", [0, 0]));
    expect(loaded).toBe(bare);
  });

  it("includes field rows only for the matching aspect", () => {
    const e = entWithFields("p", "A", [10, 10]);
    const collapsed = estimateCardHeight(e, null);
    const filtered = estimateCardHeight(e, "k0");
    expect(filtered).toBe(collapsed + 10 * 26 + 8);
  });

  it("ignores a filter the entity does not carry", () => {
    const e = entWithFields("p", "A", [10, 10]);
    expect(estimateCardHeight(e, "nope")).toBe(estimateCardHeight(e, null));
  });

  it("packs a tighter map with no filter than with one", () => {
    const entities = genEntities("big", 40);
    const bottom = (out: Record<string, { x: number; y: number }>, f: string | null) =>
      Math.max(...entities.map((e) => out[nodeId(e)].y + estimateCardHeight(e, f)));
    expect(bottom(autoLayout(entities, {}), null)).toBeLessThan(
      bottom(autoLayout(entities, {}, "k0"), "k0"),
    );
  });

  it("never overlaps two cards in a column with a filter active", () => {
    const entities = genEntities("big", 40);
    const out = autoLayout(entities, {}, "k0");
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
        expect(list[i + 1].y).toBeGreaterThanOrEqual(list[i].y + estimateCardHeight(cur, "k0"));
      }
    }
  });

  it("stays deterministic for a given filter", () => {
    const entities = genEntities("big", 40);
    expect(autoLayout(entities, {}, "k0")).toEqual(autoLayout(entities, {}, "k0"));
  });

  it("keeps pinned positions across a filter change", () => {
    const entities = genEntities("big", 40);
    const pinnedId = nodeId(entities[3]);
    const saved = { [pinnedId]: { x: 999, y: 888 } };
    expect(autoLayout(entities, saved, null)[pinnedId]).toEqual({ x: 999, y: 888 });
    expect(autoLayout(entities, saved, "k0")[pinnedId]).toEqual({ x: 999, y: 888 });
  });
});

// The estimate has to match what EntityCard actually draws, or a filtered map
// packs cards on top of each other. These tests pin each constant in
// estimateCardHeight against a literal derived by hand from EntityCard.vue's
// CSS (comment on each `it`). jsdom performs no layout, so this only catches
// a constant drifting from the value below it — it does NOT catch the CSS
// itself changing underneath an unchanged constant. That would need a real
// browser (vitest browser mode), which is out of scope here.
describe("estimate matches rendered card geometry", () => {
  it("reserves a full rendered row per field, plus the field list's padding", () => {
    // .field: 22px line-height + 2px padding top and bottom = 26px
    // .fields: var(--unit) padding top and bottom = 8px
    const e = entWithFields("p", "A", [7]);
    expect(estimateCardHeight(e, "k0") - estimateCardHeight(e, null)).toBe(7 * 26 + 8);
  });

  it("reserves the rendered strip header height per aspect", () => {
    // .strip-head: 16px line box + 6px padding top and bottom + 1px border = 29px
    const one = estimateCardHeight(entWithFields("p", "A", [0]));
    const three = estimateCardHeight(entWithFields("p", "B", [0, 0, 0]));
    expect(three - one).toBe(2 * 29);
  });

  it("reserves the rendered empty-state height for a shell entity", () => {
    // .empty-state: 15px line box + 8px padding top and bottom = 31px
    const shell = estimateCardHeight(entWithFields("p", "A", []));
    const oneStrip = estimateCardHeight(entWithFields("p", "B", [0]));
    expect(shell - oneStrip).toBe(31 - 29);
  });

  it("leaves a real gap between cards in a filtered column", () => {
    // True rendered height of a card with 3 strips and 13 fields in the
    // filtered strip, hand-derived from EntityCard.vue's CSS independently
    // of estimateCardHeight's own constants (so a regression in those
    // constants moves the packer's spacing without moving this floor).
    // Title block assumes one description line, matching TITLE_H's model,
    // which is why the test entities below carry a (short, non-wrapping)
    // description.
    const TRUE_TITLEBLOCK_H = 78; // titleblock padding(16) + row1 + badges + one desc line
    const TRUE_BORDER_H = 2; // 1px card border, top and bottom
    const TRUE_STRIP_HEADER_H = 29; // 16px line box + 6px padding + 1px border
    const TRUE_FIELD_ROW_H = 26; // 22px line-height + 2px padding top and bottom
    const TRUE_FIELDS_PAD_H = 8; // .fields: var(--unit) padding top and bottom
    const trueHeight = (aspectCount: number, filteredFieldCount: number) =>
      TRUE_TITLEBLOCK_H +
      TRUE_BORDER_H +
      aspectCount * TRUE_STRIP_HEADER_H +
      filteredFieldCount * TRUE_FIELD_ROW_H +
      TRUE_FIELDS_PAD_H;

    const entities = Array.from({ length: 40 }, (_, i) => ({
      ...entWithFields("big", `E${String(i).padStart(3, "0")}`, [13, 4, 4]),
      description: "one short line, well under the wrap width",
    }));
    const out = autoLayout(entities, {}, "k0");

    const byCol = new Map<number, number[]>();
    for (const e of entities) {
      const pos = out[nodeId(e)];
      byCol.set(pos.x, [...(byCol.get(pos.x) ?? []), pos.y]);
    }
    for (const ys of byCol.values()) {
      ys.sort((a, b) => a - b);
      for (let i = 0; i + 1 < ys.length; i++) {
        expect(ys[i + 1] - ys[i]).toBeGreaterThanOrEqual(trueHeight(3, 13));
      }
    }
  });
});
