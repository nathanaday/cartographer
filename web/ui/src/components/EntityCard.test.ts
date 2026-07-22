import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import EntityCard from "./EntityCard.vue";
import type { Entity } from "../types/api";

const entity: Entity = {
  project: "p3-edge",
  name: "DataObject",
  description: "envelope",
  aspects: [
    {
      kind: "protobuf",
      fields: [
        { name: "track_id", type: "string", nullable: false },
        { name: "geometry", type: "Geometry", nullable: false, references: "Geometry.kind" },
      ],
    },
    { kind: "json", fields: [{ name: "ts", type: "string", nullable: true }] },
  ],
};

describe("EntityCard", () => {
  it("renders name, aspect strips with counts, and typed fields at full LOD", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: null } } });
    expect(w.text()).toContain("DataObject");
    expect(w.text()).toContain("protobuf");
    expect(w.text()).toContain("2"); // protobuf field count
    expect(w.text()).toContain("track_id");
    expect(w.text()).toContain("Aa"); // string sigil
    expect(w.text()).toContain("()"); // Geometry object sigil
  });

  it("at badges LOD shows name and kind badges but not field rows", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "badges", kindFilter: null } } });
    expect(w.text()).toContain("DataObject");
    expect(w.text()).not.toContain("track_id");
  });
});
