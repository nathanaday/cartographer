import { describe, it, expect } from "vitest";
import { edgesFrom } from "./edges";
import type { Entity } from "../types/api";

const ent = (project: string, name: string, refs: string[]): Entity => ({
  project,
  name,
  aspects: [
    { kind: "sql", fields: refs.map((r, i) => ({ name: `f${i}`, type: "string", nullable: false, references: r })) },
  ],
});

describe("edgesFrom", () => {
  it("derives edges from references and drops dangling ones", () => {
    const entities = [
      ent("p3-edge", "DataObject", ["Geometry.kind", "Ghost.x"]),
      ent("p3-edge", "Geometry", []),
    ];
    const edges = edgesFrom(entities);
    expect(edges).toHaveLength(1);
    expect(edges[0].source).toBe("p3-edge/DataObject");
    expect(edges[0].target).toBe("p3-edge/Geometry");
  });

  it("dedupes multiple references to the same target", () => {
    const entities = [ent("p", "A", ["B.x", "B.y"]), ent("p", "B", [])];
    expect(edgesFrom(entities)).toHaveLength(1);
  });

  it("resolves references within the source project", () => {
    const entities = [
      ent("p3-edge", "DataObject", ["Geometry.kind"]),
      ent("p3-edge", "Geometry", []),
      ent("p3-cloud", "Geometry", []),
    ];
    const edges = edgesFrom(entities);
    expect(edges).toHaveLength(1);
    expect(edges[0].target).toBe("p3-edge/Geometry");
    expect(edges[0].target).not.toBe("p3-cloud/Geometry");
  });
});
