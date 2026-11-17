import { describe, expect, it, vi } from "vitest";
import { safeReturnPath } from "./navigation";
describe("safe account return paths", () => {
  it("accepts only local paths and removes fragments", () => {
    expect(safeReturnPath("/join/abc?x=1#secret")).toBe("/join/abc?x=1");
    for (const v of [
      "https://example.com",
      "//example.com",
      "/\\example.com",
      "/\u0000x",
      "",
      "javascript:alert(1)",
      "/%2f%2fevil.test",
      "/%5cevil.test",
    ])
      expect(safeReturnPath(v)).toBe("/account");
  });
});
