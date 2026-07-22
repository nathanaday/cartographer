import { afterAll, afterEach, beforeAll } from "vitest";
import { server } from "./src/mocks/server";

// jsdom has no ResizeObserver; Vue Flow needs one to observe node/pane size.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= ResizeObserverStub as unknown as typeof ResizeObserver;

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => server.resetHandlers());
afterAll(() => server.close());
