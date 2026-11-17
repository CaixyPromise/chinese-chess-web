import { describe, expect, it } from "vitest";
import { replayPositions } from "./position";
import { INITIAL, parseFen, siteOf } from "../game/board/position";

describe("replay positions", () => {
  it("reconstructs every prefix including captures without mutating the record", () => {
    const moves = Object.freeze(["c3c4", "c6c5", "c4c5"]);
    const states = replayPositions(moves);
    expect(states).toHaveLength(4);
    expect(states[0].fen).toBe(INITIAL);
    expect(states[3].capture).toBe("卒");
    expect(parseFen(states[2].fen)[siteOf("c5")]).toBe("p");
    expect(parseFen(states[3].fen)[siteOf("c5")]).toBe("P");
    expect(parseFen(states[3].fen)[siteOf("c4")]).toBeNull();
    expect(states[1].fen.split(" ")[1]).toBe("b");
    expect(states[2].fen.split(" ")[1]).toBe("r");
    expect(moves).toEqual(["c3c4", "c6c5", "c4c5"]);
  });

  it("rejects corrupt coordinates and empty origins", () => {
    for (const moves of [["z0a0"], ["a4a5"], ["a0a0"]])
      expect(() => replayPositions(moves)).toThrow("INVALID_RECORD");
  });
});
