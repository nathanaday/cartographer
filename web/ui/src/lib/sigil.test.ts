import { describe, it, expect } from "vitest";
import { sigilFor } from "./sigil";

describe("sigilFor", () => {
  it("classifies strings", () => {
    for (const t of ["string", "std::string", "str", "VARCHAR(255)", "TEXT", "Mapped[str]"]) {
      expect(sigilFor(t)).toBe("Aa");
    }
  });
  it("classifies numbers", () => {
    for (const t of ["int32", "float", "DOUBLE", "number", "Mapped[float]"]) {
      expect(sigilFor(t)).toBe("#");
    }
  });
  it("classifies booleans", () => {
    for (const t of ["bool", "boolean", "Mapped[bool]"]) expect(sigilFor(t)).toBe("01");
  });
  it("classifies timestamps", () => {
    for (const t of ["timestamp", "datetime", "Mapped[datetime]"]) expect(sigilFor(t)).toBe("◷");
  });
  it("classifies arrays", () => {
    for (const t of ["repeated Property", "std::vector<Property>", "Mapped[list]"]) {
      expect(sigilFor(t)).toBe("[]");
    }
  });
  it("classifies objects/messages", () => {
    for (const t of ["Geometry", "Command", "Mapped[dict]", "Mapped[uuid.UUID]"]) {
      expect(sigilFor(t)).toBe("()");
    }
  });
  it("falls back to ? honestly", () => {
    expect(sigilFor("")).toBe("?");
    expect(sigilFor("¯\\_(ツ)_/¯")).toBe("?");
  });
});
