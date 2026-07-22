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
    );
  });

  it("loads entities, computes positions and edges", async () => {
    const m = useMap();
    await m.load();
    expect(m.entities.value).toHaveLength(2);
    expect(Object.keys(m.positions.value)).toContain("p3-edge/DataObject");
    expect(m.edges.value).toHaveLength(1);
    expect(m.edges.value[0].source).toBe("p3-edge/DataObject");
  });
});
