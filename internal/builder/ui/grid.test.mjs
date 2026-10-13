import { test } from "node:test";
import assert from "node:assert/strict";
import { cellRect, overlaps, fits, minSpans, firstFree, bestFree, fromLegacy, moveCell, resizeCell, autoFont } from "./grid.js";

const g = { columns: 12, rows: 16 };

test("cellRect matches layout.Grid.Rect (integer edges, exact tiling)", () => {
  assert.deepEqual(cellRect(g, [6, 8, 6, 8], 984, 1304), { x: 492, y: 652, w: 492, h: 652 });
  const last = cellRect(g, [11, 15, 1, 1], 981, 1304);
  assert.equal(last.x + last.w, 981);
  assert.equal(last.y + last.h, 1304);
  const a = cellRect(g, [3, 0, 1, 1], 981, 100), b = cellRect(g, [4, 0, 1, 1], 981, 100);
  assert.equal(a.x + a.w, b.x);
});

test("overlaps ignores touching edges", () => {
  assert.equal(overlaps([0, 0, 6, 4], [6, 0, 6, 4]), false);
  assert.equal(overlaps([0, 0, 6, 4], [5, 3, 6, 4]), true);
});

test("fits checks bounds and other cells, ignoring the moved one", () => {
  const cells = [[0, 0, 6, 4], [6, 0, 6, 4]];
  assert.equal(fits(g, cells, [0, 4, 12, 2], -1), true);
  assert.equal(fits(g, cells, [5, 0, 2, 2], -1), false);
  assert.equal(fits(g, cells, [1, 0, 5, 4], 0), true);
  assert.equal(fits(g, cells, [8, 0, 6, 4], 1), false); // past the right edge
});

test("minSpans: smallest spans whose inner size reaches the module minimum", () => {
  // 984 / 12 = 82 px per column; Calendar needs 70 inner + 2×40 padding = 150 → 2 columns
  assert.deepEqual(minSpans(g, { min_w: 70, min_h: 100 }, 984, 1304, 40, 18), [2, 2]);
  assert.deepEqual(minSpans(g, { min_w: 10, min_h: 10 }, 984, 1304, 0, 0), [1, 1]);
  assert.deepEqual(minSpans(g, { min_w: 5000, min_h: 10 }, 984, 1304, 0, 0), [12, 1]);
});

test("firstFree scans rows top to bottom, then columns", () => {
  assert.deepEqual(firstFree(g, [[0, 0, 12, 4]], 6, 2), [0, 4, 6, 2]);
  assert.deepEqual(firstFree(g, [[0, 0, 6, 16]], 6, 3), [6, 0, 6, 3]);
  assert.equal(firstFree(g, [[0, 0, 12, 16]], 1, 1), null);
});

test("fromLegacy stacks full-width tiles proportional to the old heights", () => {
  // the user's real dashboard on the 12.48": 290 + 660 + 180 + 174 = 1304
  const cells = fromLegacy([290, 660, 180, 174], 1304, g, [1, 1, 1, 1]);
  assert.deepEqual(cells.map(c => [c[0], c[2]]), [[0, 12], [0, 12], [0, 12], [0, 12]]);
  assert.equal(cells[0][1], 0);
  for (let i = 1; i < cells.length; i++) assert.equal(cells[i][1], cells[i - 1][1] + cells[i - 1][3]);
  const sum = cells.reduce((s, c) => s + c[3], 0);
  assert.ok(sum <= 16, `rows ${sum}`);
  assert.ok(cells[1][3] > cells[0][3], "calendar is the tallest");
  // too many modules for the rows: every one keeps at least its minimum, the largest shrink
  const many = fromLegacy([500, 500, 500, 500], 1304, { columns: 12, rows: 4 }, [1, 1, 1, 1]);
  assert.deepEqual(many.map(c => c[3]), [1, 1, 1, 1]);
});

test("moveCell and resizeCell stay inside the grid and above the minimum", () => {
  assert.deepEqual(moveCell([10, 0, 4, 2], 5, 0, g), [8, 0, 4, 2]);
  assert.deepEqual(moveCell([0, 0, 4, 2], -3, -1, g), [0, 0, 4, 2]);
  assert.deepEqual(resizeCell([2, 2, 4, 4], "se", 2, 1, [1, 1], g), [2, 2, 6, 5]);
  assert.deepEqual(resizeCell([2, 2, 4, 4], "nw", -1, -1, [1, 1], g), [1, 1, 5, 5]);
  assert.deepEqual(resizeCell([2, 2, 4, 4], "w", 5, 0, [2, 1], g), [4, 2, 2, 4]);
  assert.deepEqual(resizeCell([2, 2, 4, 4], "e", 20, 0, [1, 1], g), [2, 2, 10, 4]);
});

test("autoFont follows modules.AutoFontSize", () => {
  const rule = { h_div: 9, w_div: 40, min: 12, max: 32 };
  assert.equal(autoFont(rule, 904, 624), 22);
  assert.equal(autoFont(rule, 300, 60), 12);
  assert.equal(autoFont(rule, 2000, 2000), 32);
});

test("bestFree picks the largest free box up to the preferred size, at least the minimum", () => {
  const g = { columns: 12, rows: 16 };
  // the user's case: a 6×2 hole next to a half-width module at the bottom
  const cells = [[0, 0, 12, 14], [0, 14, 6, 2]];
  assert.deepEqual(bestFree(g, cells, [2, 1], [12, 4]), [6, 14, 6, 2]);
  // empty page: the preferred size at the top
  assert.deepEqual(bestFree(g, [], [2, 1], [12, 4]), [0, 0, 12, 4]);
  // no room even for the minimum
  assert.equal(bestFree(g, [[0, 0, 12, 16]], [1, 1], [12, 4]), null);
});

test("makeRoom shrinks the biggest tile that can spare the space", async () => {
  const { makeRoom } = await import("./grid.js");
  const g = { columns: 12, rows: 16 };
  // the user's converted dashboard: 4 + 8 + 2 + 2 rows, all full width
  const cells = [[0, 0, 12, 4], [0, 4, 12, 8], [0, 12, 12, 2], [0, 14, 12, 2]];
  const mins = [[2, 1], [2, 2], [2, 1], [2, 1]];
  const r = makeRoom(g, cells, mins, [2, 2]);
  assert.deepEqual(r.cells[1], [0, 4, 12, 6], "the calendar gives two rows");
  assert.deepEqual(r.cell, [0, 10, 12, 2]);
  assert.equal(r.shrunk, 1);
  // nothing can give: null
  assert.equal(makeRoom(g, [[0, 0, 12, 16]], [[12, 16]], [1, 1]), null);
});

test("nudge moves and resizes by one cell, refusing overlaps and the minimum", async () => {
  const { nudge } = await import("./grid.js");
  const g = { columns: 12, rows: 16 };
  const cells = [[0, 0, 12, 4], [0, 4, 6, 4]];
  const mins = [[2, 2], [2, 2]];
  assert.deepEqual(nudge(g, cells, mins, 1, "right"), [1, 4, 6, 4]);
  assert.equal(nudge(g, cells, mins, 1, "up"), null, "would overlap the top tile");
  assert.deepEqual(nudge(g, cells, mins, 1, "taller"), [0, 4, 6, 5]);
  assert.deepEqual(nudge(g, cells, mins, 1, "wider"), [0, 4, 7, 4]);
  assert.deepEqual(nudge(g, cells, [[2, 2], [2, 4]], 1, "shorter"), null, "already at the minimum");
  assert.deepEqual(nudge(g, cells, mins, 0, "left"), null, "already at the edge");
});
