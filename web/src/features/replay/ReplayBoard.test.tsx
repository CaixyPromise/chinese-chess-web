import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ReplayBoard } from "./ReplayBoard";

describe("read-only replay", () => {
  it("supports first/previous/next/last, move selection and range keyboard without changing source", () => {
    const moves = ["c3c4", "c6c5", "c4c5"];
    render(<ReplayBoard moves={moves} />);
    expect(screen.getByRole("status")).toHaveTextContent("吃卒");
    fireEvent.click(screen.getByRole("button", { name: "开局" }));
    expect(screen.getByRole("slider")).toHaveValue("0");
    expect(screen.getByRole("button", { name: "上一步" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "下一步" }));
    expect(screen.getByRole("slider")).toHaveValue("1");
    fireEvent.keyDown(screen.getByRole("button", { name: "下一步" }), {
      key: "End",
    });
    expect(screen.getByRole("slider")).toHaveValue("3");
    fireEvent.click(screen.getByRole("button", { name: /第 2 手/ }));
    expect(screen.getByRole("slider")).toHaveValue("2");
    fireEvent.change(screen.getByRole("slider"), { target: { value: "1" } });
    expect(screen.getByRole("status")).toHaveTextContent("第 1 手");
    fireEvent.click(screen.getByRole("gridcell", { name: "红兵 c4" }));
    expect(screen.getByRole("slider")).toHaveValue("1");
    expect(
      screen
        .getAllByRole("gridcell")
        .every((cell) => cell.getAttribute("aria-disabled") === "true"),
    ).toBe(true);
    expect(moves).toEqual(["c3c4", "c6c5", "c4c5"]);
  });
});

it("supports a controlled replay without changing the source moves", () => {
  const moves = ["c3c4", "c6c5"];
  const selected: number[] = [];
  const { rerender } = render(
    <ReplayBoard
      moves={moves}
      ply={0}
      onPlyChange={(ply) => selected.push(ply)}
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "下一步" }));
  expect(selected).toEqual([1]);
  expect(screen.getByRole("slider")).toHaveValue("0");
  rerender(
    <ReplayBoard
      moves={moves}
      ply={1}
      onPlyChange={(ply) => selected.push(ply)}
    />,
  );
  expect(screen.getByRole("slider")).toHaveValue("1");
  expect(moves).toEqual(["c3c4", "c6c5"]);
});
