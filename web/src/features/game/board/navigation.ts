const directions: Record<string, [number, number]> = {
  ArrowLeft: [-1, 0],
  ArrowRight: [1, 0],
  ArrowUp: [0, -1],
  ArrowDown: [0, 1],
};

// Read live geometry so a flipped board and resized cells have identical keyboard behavior.
export function spatialTarget(
  current: HTMLElement,
  candidates: HTMLElement[],
  key: string,
): HTMLElement | undefined {
  const direction = directions[key];
  if (!direction) return;
  const [dx, dy] = direction,
    origin = current.getBoundingClientRect();
  const options = candidates
    .filter((el) => el !== current && el.getClientRects().length > 0)
    .map((el) => {
      const r = el.getBoundingClientRect();
      const gap =
        dx > 0
          ? r.left - origin.right
          : dx < 0
            ? origin.left - r.right
            : dy > 0
              ? r.top - origin.bottom
              : origin.top - r.bottom;
      const overlap = dx
        ? r.top < origin.bottom && r.bottom > origin.top
        : r.left < origin.right && r.right > origin.left;
      const offset = dx
        ? Math.abs((r.top + r.bottom - origin.top - origin.bottom) / 2)
        : Math.abs((r.left + r.right - origin.left - origin.right) / 2);
      return { el, gap, overlap, offset };
    })
    .filter((x) => x.gap >= -2);
  options.sort(
    (a, b) =>
      Number(b.overlap) - Number(a.overlap) ||
      a.gap + 3 * a.offset - (b.gap + 3 * b.offset),
  );
  return options[0]?.el;
}
