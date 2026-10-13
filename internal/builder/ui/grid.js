// Grid maths for the builder. Cells are [col, row, colSpan, rowSpan], 0-based, exactly
// as in settings.json; rectangles use the same integer edges as layout.Grid.Rect in Go.

export function cellRect(grid, cell, w, h) {
  const [c, r, cs, rs] = cell;
  const x0 = Math.floor((c * w) / grid.columns), x1 = Math.floor(((c + cs) * w) / grid.columns);
  const y0 = Math.floor((r * h) / grid.rows), y1 = Math.floor(((r + rs) * h) / grid.rows);
  return { x: x0, y: y0, w: x1 - x0, h: y1 - y0 };
}

export function overlaps(a, b) {
  return a[0] < b[0] + b[2] && b[0] < a[0] + a[2] && a[1] < b[1] + b[3] && b[1] < a[1] + a[3];
}

export function inside(grid, cell) {
  const [c, r, cs, rs] = cell;
  return c >= 0 && r >= 0 && cs >= 1 && rs >= 1 && c + cs <= grid.columns && r + rs <= grid.rows;
}

// fits: inside the grid and free of every other cell (ignore = index of the cell being moved).
export function fits(grid, cells, cell, ignore) {
  return inside(grid, cell) && cells.every((other, i) => i === ignore || !overlaps(cell, other));
}

// smallest span along one axis whose narrowest placement still gives `need` pixels
function minSpan(count, total, need) {
  for (let span = 1; span <= count; span++) {
    let narrowest = Infinity;
    for (let start = 0; start + span <= count; start++) {
      narrowest = Math.min(narrowest, Math.floor(((start + span) * total) / count) - Math.floor((start * total) / count));
    }
    if (narrowest >= need) return span;
  }
  return count;
}

// minSpans: [colSpan, rowSpan] a module needs for its Meta minimum inner size.
export function minSpans(grid, meta, pageW, areaH, padX, padY) {
  return [
    minSpan(grid.columns, pageW, (meta.min_w || 1) + 2 * padX),
    minSpan(grid.rows, areaH, (meta.min_h || 1) + 2 * padY),
  ];
}

export function firstFree(grid, cells, cs, rs) {
  for (let r = 0; r + rs <= grid.rows; r++) {
    for (let c = 0; c + cs <= grid.columns; c++) {
      if (fits(grid, cells, [c, r, cs, rs], -1)) return [c, r, cs, rs];
    }
  }
  return null;
}

// bestFree finds room for a new module: the largest free box no bigger than pref and at
// least min, the topmost-leftmost among equals.
export function bestFree(grid, cells, min, pref) {
  let best = null;
  for (let rs = Math.min(pref[1], grid.rows); rs >= min[1]; rs--) {
    for (let cs = Math.min(pref[0], grid.columns); cs >= min[0]; cs--) {
      if (best && cs * rs <= best[2] * best[3]) continue;
      const cell = firstFree(grid, cells, cs, rs);
      if (cell) best = cell;
    }
  }
  return best;
}

// fromLegacy turns stacked module heights into full-width cells, rows proportional to the
// old heights, at least minRows[i] each; when they do not fit, the tallest shrink first.
export function fromLegacy(heights, areaH, grid, minRows) {
  const spans = heights.map((h, i) => Math.max(minRows[i] || 1, Math.round(((h || 0) / areaH) * grid.rows)));
  let sum = spans.reduce((s, n) => s + n, 0);
  while (sum > grid.rows) {
    let pick = -1;
    spans.forEach((n, i) => {
      if (n > (minRows[i] || 1) && (pick < 0 || n > spans[pick])) pick = i;
    });
    if (pick < 0) break;
    spans[pick]--;
    sum--;
  }
  let row = 0;
  return spans.map(n => {
    const cell = [0, Math.min(row, grid.rows - 1), grid.columns, n];
    row += n;
    return cell;
  });
}

const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));

export function moveCell(cell, dc, dr, grid) {
  const [c, r, cs, rs] = cell;
  return [clamp(c + dc, 0, grid.columns - cs), clamp(r + dr, 0, grid.rows - rs), cs, rs];
}

// resizeCell moves the edges named in `edge` (n, s, e, w or corners like "se").
export function resizeCell(cell, edge, dc, dr, min, grid) {
  let [c, r, cs, rs] = cell;
  const [minC, minR] = min;
  if (edge.includes("e")) cs = clamp(cs + dc, minC, grid.columns - c);
  if (edge.includes("s")) rs = clamp(rs + dr, minR, grid.rows - r);
  if (edge.includes("w")) {
    const nc = clamp(c + dc, 0, c + cs - minC);
    cs -= nc - c;
    c = nc;
  }
  if (edge.includes("n")) {
    const nr = clamp(r + dr, 0, r + rs - minR);
    rs -= nr - r;
    r = nr;
  }
  return [c, r, cs, rs];
}

// autoFont mirrors modules.AutoFontSize.
export function autoFont(rule, innerW, innerH) {
  return Math.max(rule.min, Math.min(rule.max, Math.floor(innerH / rule.h_div), Math.floor(innerW / rule.w_div)));
}

// makeRoom frees space on a full page: the biggest tile that can spare need rows (or
// columns) above its minimum shrinks from the bottom (or right), and the new tile goes
// into the gap. Returns {cells, cell, shrunk} or null.
export function makeRoom(grid, cells, mins, need) {
  const order = cells.map((c, i) => i).sort((a, b) => cells[b][2] * cells[b][3] - cells[a][2] * cells[a][3]);
  for (const i of order) {
    const [c, r, cs, rs] = cells[i];
    const tries = [];
    if (cs >= need[0] && rs - need[1] >= mins[i][1]) tries.push([c, r, cs, rs - need[1]]);
    if (rs >= need[1] && cs - need[0] >= mins[i][0]) tries.push([c, r, cs - need[0], rs]);
    for (const shrunk of tries) {
      const next = cells.map((x, j) => (j === i ? shrunk : x));
      const cell = bestFree(grid, next, need, [grid.columns, need[1]]);
      if (cell) return { cells: next, cell, shrunk: i };
    }
  }
  return null;
}

// nudge applies one phone button to tile i: up, down, left, right, taller, shorter,
// wider, narrower. null when it would leave the grid, overlap or go below the minimum.
export function nudge(grid, cells, mins, i, action) {
  const cell = cells[i];
  const step = {
    up: () => moveCell(cell, 0, -1, grid), down: () => moveCell(cell, 0, 1, grid),
    left: () => moveCell(cell, -1, 0, grid), right: () => moveCell(cell, 1, 0, grid),
    taller: () => resizeCell(cell, "s", 0, 1, mins[i], grid), shorter: () => resizeCell(cell, "s", 0, -1, mins[i], grid),
    wider: () => resizeCell(cell, "e", 1, 0, mins[i], grid), narrower: () => resizeCell(cell, "e", -1, 0, mins[i], grid),
  }[action];
  if (!step) return null;
  const next = step();
  if (String(next) === String(cell) || !fits(grid, cells, next, i)) return null;
  return next;
}
