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

  it("at chip LOD shows the name but not field rows", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "chip", kindFilter: null } } });
    expect(w.text()).toContain("DataObject");
    expect(w.text()).not.toContain("track_id");
  });

  it("chip is hollow for zero-aspect entities", () => {
    const empty: Entity = { project: "p", name: "Shell", aspects: [] };
    const w = mount(EntityCard, { props: { data: { entity: empty, lod: "chip", kindFilter: null } } });
    expect(w.find(".chip").classes()).toContain("hollow");
  });

  it("shows the empty state row at full tier for zero-aspect entities", () => {
    const empty: Entity = { project: "p", name: "Shell", aspects: [] };
    const w = mount(EntityCard, { props: { data: { entity: empty, lod: "full", kindFilter: null } } });
    expect(w.text()).toContain("no representations recorded");
  });
});
