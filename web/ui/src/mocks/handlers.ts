import { http, HttpResponse } from "msw";

export const handlers = [
  http.get("/api/version", () => HttpResponse.json({ fingerprint: "seed" })),
  http.get("/api/map", () =>
    HttpResponse.json({ fingerprint: "seed", projects: [], entities: [] }),
  ),
];
