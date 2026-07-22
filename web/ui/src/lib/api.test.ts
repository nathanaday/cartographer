import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../mocks/server";
import { apiGet, ApiError } from "./api";
import type { MapResponse } from "../types/api";

describe("apiGet", () => {
  it("returns parsed json", async () => {
    const res = await apiGet<MapResponse>("/api/map");
    expect(res.fingerprint).toBe("seed");
  });
  it("throws ApiError on non-2xx", async () => {
    server.use(http.get("/api/boom", () => new HttpResponse(null, { status: 500 })));
    await expect(apiGet("/api/boom")).rejects.toBeInstanceOf(ApiError);
  });
});
