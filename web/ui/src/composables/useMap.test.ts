import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { useMap } from "./useMap";
import type { Entity } from "../types/api";

const dataObject: Entity = {
  project: "p3-edge",
  name: "DataObject",
  aspects: [{ kind: "protobuf", fields: [{ name: "geometry", type: "Geometry", nullable: false, references: "Geometry.kind" }] }],
};
const geometry: Entity = { project: "p3-edge", name: "Geometry", aspects: [{ kind: "protobuf", fields: [] }] };

describe("useMap", () => {
  beforeEach(() => {
    server.use(
      http.get("/api/version", () => HttpResponse.json({ fingerprint: "v1" })),
      http.get("/api/map", () =>
        HttpResponse.json({ fingerprint: "v1", projects: [{ slug: "p3-edge", name: "p3-edge" }], entities: [dataObject, geometry] }),
      ),
      http.get("/api/projects/p3-edge/layout", () => HttpResponse.json({})),
      http.put("/api/projects/p3-edge/layout", () => HttpResponse.json({ saved: true })),
    );
    useMap().pinned.value = {};
  });

  it("loads entities, computes positions and edges", async () => {
    const m = useMap();
    await m.load();
    expect(m.entities.value).toHaveLength(2);
    expect(Object.keys(m.positions.value)).toContain("p3-edge/DataObject");
    expect(m.edges.value).toHaveLength(1);
    expect(m.edges.value[0].source).toBe("p3-edge/DataObject");
  });

  it("savePosition persists only pinned cards", async () => {
    let putBody: unknown = null;
    server.use(
      http.put("/api/projects/p3-edge/layout", async ({ request }) => {
        putBody = await request.json();
        return HttpResponse.json({ saved: true });
      }),
    );
    const m = useMap();
    await m.load();
    await m.savePosition("p3-edge/DataObject", { x: 10, y: 20 });
    // only the dragged card is in the body — Geometry stays auto
    expect(putBody).toEqual({ DataObject: { x: 10, y: 20 } });
    expect(m.positions.value["p3-edge/DataObject"]).toEqual({ x: 10, y: 20 });
  });

  it("unpinned cards reflow when the entity set changes; pinned stay put", async () => {
    const m = useMap();
    await m.load();
    await m.savePosition("p3-edge/DataObject", { x: 999, y: 999 });
    const geometryBefore = m.positions.value["p3-edge/Geometry"];
    // simulate a re-scan adding an entity that sorts before Geometry
    m.entities.value = [
      ...m.entities.value,
      { project: "p3-edge", name: "Aaa", aspects: [] },
    ];
    expect(m.positions.value["p3-edge/DataObject"]).toEqual({ x: 999, y: 999 }); // pinned
    expect(m.positions.value["p3-edge/Geometry"]).not.toEqual(geometryBefore);   // reflowed
  });

  it("pinPositions merges multiple updates into one PUT per project", async () => {
    let putBody: unknown = null;
    let putCount = 0;
    server.use(
      http.put("/api/projects/p3-edge/layout", async ({ request }) => {
        putCount += 1;
        putBody = await request.json();
        return HttpResponse.json({ saved: true });
      }),
    );
    const m = useMap();
    await m.load();
    await m.pinPositions({
      "p3-edge/DataObject": { x: 10, y: 20 },
      "p3-edge/Geometry": { x: 30, y: 40 },
    });
    expect(putCount).toBe(1);
    expect(putBody).toEqual({
      DataObject: { x: 10, y: 20 },
      Geometry: { x: 30, y: 40 },
    });
    expect(m.positions.value["p3-edge/DataObject"]).toEqual({ x: 10, y: 20 });
    expect(m.positions.value["p3-edge/Geometry"]).toEqual({ x: 30, y: 40 });
  });

  it("resetLayout clears pins and PUTs empty maps", async () => {
    const bodies: Record<string, unknown> = {};
    server.use(
      http.put("/api/projects/:slug/layout", async ({ params, request }) => {
        bodies[params.slug as string] = await request.json();
        return HttpResponse.json({ saved: true });
      }),
    );
    const m = useMap();
    await m.load();
    await m.savePosition("p3-edge/DataObject", { x: 1, y: 2 });
    await m.resetLayout();
    expect(bodies["p3-edge"]).toEqual({});
    expect(m.positions.value["p3-edge/DataObject"]).not.toEqual({ x: 1, y: 2 });
  });
});
