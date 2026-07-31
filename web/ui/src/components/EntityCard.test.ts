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
    // only the protobuf strip's 2 fields render; the json strip stays collapsed
    expect(w.findAll(".field")).toHaveLength(2);
    expect(w.findAll(".strip")[1].find(".fields").exists()).toBe(false);
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

  it("expands a strip when the filter changes on an already-mounted card", async () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: null } } });
    expect(w.text()).not.toContain("track_id");
    await w.setProps({ data: { entity, lod: "full", kindFilter: "protobuf" } });
    expect(w.text()).toContain("track_id");
  });

  it("a manual toggle stays sticky when a filter change would otherwise re-collapse it (spec: manual toggle always wins)", async () => {
    // §1 (manual toggle wins) and §2 (clearing the filter collapses
    // everything back to default) conflict once a strip has been toggled.
    // The intended resolution is that userToggled is sticky forever: once a
    // user acts on a strip, no later filter change moves it again.
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: "protobuf" } } });
    expect(w.text()).toContain("track_id"); // opened by the filter

    await w.findAll(".strip-head")[0].trigger("click"); // user closes it
    expect(w.text()).not.toContain("track_id");

    await w.setProps({ data: { entity, lod: "full", kindFilter: null } }); // filter cleared
    expect(w.text()).not.toContain("track_id"); // stays closed: the manual toggle is sticky, not the filter

    await w.setProps({ data: { entity, lod: "full", kindFilter: "protobuf" } }); // filter re-applied
    expect(w.text()).not.toContain("track_id"); // still closed: the toggle outranks the filter every time
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

  it("hard-dims a card whose entity lacks the filtered kind", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: "sql" } } });
    expect(w.find(".card").classes()).toContain("dim-hard");
  });

  it("leaves a matching card undimmed", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: "protobuf" } } });
    expect(w.find(".card").classes()).not.toContain("dim-hard");
  });

  it("does not dim any card when the filter is cleared", () => {
    const w = mount(EntityCard, { props: { data: { entity, lod: "full", kindFilter: null } } });
    expect(w.find(".card").classes()).not.toContain("dim-hard");
  });

  it("hard-dims a non-matching chip and emphasizes the matching chip's spine", () => {
    const miss = mount(EntityCard, { props: { data: { entity, lod: "chip", kindFilter: "sql" } } });
    expect(miss.find(".chip").classes()).toContain("dim-hard");

    const hit = mount(EntityCard, { props: { data: { entity, lod: "chip", kindFilter: "protobuf" } } });
    expect(hit.find(".chip").classes()).not.toContain("dim-hard");
    expect(hit.find(".chip").classes()).toContain("matched");
    expect(hit.findAll(".seg-match")).toHaveLength(1);
  });

  // A field row has to stay one line tall or the packer's height estimate is
  // wrong and filtered cards overlap. Long types are clipped, so the full text
  // has to survive somewhere the reader can reach it.
  it("keeps the full field type available when the row clips it", () => {
    const long: Entity = {
      project: "p",
      name: "Rule",
      aspects: [{ kind: "sql", fields: [{ name: "window_seconds", type: "INTEGER NOT NULL DEFAULT 0", nullable: false }] }],
    };
    const w = mount(EntityCard, { props: { data: { entity: long, lod: "full", kindFilter: "sql" } } });
    expect(w.find(".ftype").attributes("title")).toBe("INTEGER NOT NULL DEFAULT 0");
    expect(w.find(".fname").attributes("title")).toBe("window_seconds");
  });

  it("hard-dims a hollow chip under any filter", () => {
    const empty: Entity = { project: "p", name: "Shell", aspects: [] };
    const w = mount(EntityCard, { props: { data: { entity: empty, lod: "chip", kindFilter: "json" } } });
    expect(w.find(".chip").classes()).toContain("hollow");
    expect(w.find(".chip").classes()).toContain("dim-hard");
  });
});
