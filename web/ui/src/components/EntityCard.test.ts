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
  it("renders name, aspect strips with counts, and typed fields when expanded at full LOD", async () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: null } } });
    expect(w.text()).toContain("DataObject");
    expect(w.text()).toContain("protobuf");
    expect(w.text()).toContain("2"); // protobuf field count
    await w.findAll(".strip-head")[0].trigger("click");
    expect(w.text()).toContain("track_id");
    expect(w.text()).toContain("Aa"); // string sigil
    expect(w.text()).toContain("()"); // Geometry object sigil
  });

  it("renders strips collapsed by default at full LOD", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: null } } });
    expect(w.text()).toContain("protobuf");
    expect(w.text()).toContain("2"); // header count still shows
    expect(w.find(".fields").exists()).toBe(false);
    expect(w.text()).not.toContain("track_id");
  });

  it("expands exactly the strip matching the kind filter", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: "protobuf" } } });
    expect(w.text()).toContain("track_id"); // protobuf expanded
    expect(w.text()).not.toContain("ts");   // json stays collapsed
  });

  it("manual toggle opens a strip with no filter active", async () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: null } } });
    await w.findAll(".strip-head")[0].trigger("click");
    expect(w.text()).toContain("track_id");
  });

  it("manual toggle closes a strip the filter expanded", async () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: "protobuf" } } });
    expect(w.text()).toContain("track_id");
    await w.findAll(".strip-head")[0].trigger("click");
    expect(w.text()).not.toContain("track_id");
  });

  it("keeps field rows hidden at headers LOD even when the filter matches", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "headers", kindFilter: "protobuf" } } });
    expect(w.text()).toContain("protobuf");
    expect(w.text()).not.toContain("track_id");
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

  it("shows the empty state row at headers tier for zero-aspect entities", () => {
    const empty: Entity = { project: "p", name: "Shell", aspects: [] };
    const w = mount(EntityCard, { props: { data: { entity: empty, lod: "headers", kindFilter: null } } });
    expect(w.text()).toContain("no representations recorded");
  });
});
