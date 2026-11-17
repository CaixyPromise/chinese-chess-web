import { describe, expect, it } from "vitest";
import { recordRules } from "./record";
import { ruleLabel } from "./labels";

describe("recorded rules", () => {
  it("uses mode-specific defaults only when the version is absent", () => {
    expect(recordRules(undefined, "pve")).toBe("legacy-java-v1");
    expect(recordRules(undefined, "pvp")).toBe("casual-xiangqi-v1");
    expect(recordRules("", "pve")).toBe("legacy-java-v1");
    expect(recordRules("", "pvp")).toBe("casual-xiangqi-v1");
    expect(recordRules("future-version", "pvp")).toBe("future-version");
    expect(ruleLabel(recordRules("future-version", "pvp"))).toBe("其他规则");
  });
});
