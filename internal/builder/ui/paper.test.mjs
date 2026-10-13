import { test } from "node:test";
import assert from "node:assert/strict";
import { toPaper, PAPER, INK, RED, YELLOW, GREY } from "./paper.js";

const px = (r, g, b) => [r, g, b, 255];

test("toPaper maps the panel colours to e-paper tones", () => {
  const data = new Uint8ClampedArray([...px(255, 255, 255), ...px(0, 0, 0), ...px(255, 0, 0)]);
  toPaper(data, 3, 1);
  const at = i => [data[i * 4], data[i * 4 + 1], data[i * 4 + 2]];
  const near = (got, want) => got.every((v, k) => Math.abs(v - want[k]) <= 6);
  assert.ok(near(at(0), PAPER), `paper ${at(0)}`);
  assert.ok(near(at(1), INK), `ink ${at(1)}`);
  assert.ok(near(at(2), RED), `red ${at(2)}`);
  assert.equal(data[3], 255);
});

test("toPaper grain is deterministic, so the preview does not flicker", () => {
  const a = new Uint8ClampedArray(4 * 64).fill(255), b = new Uint8ClampedArray(4 * 64).fill(255);
  toPaper(a, 8, 8);
  toPaper(b, 8, 8);
  assert.deepEqual(a, b);
  assert.ok(new Set(Array.from(a.filter((_, i) => i % 4 === 0))).size > 1, "paper has some grain");
});

test("toPaper keeps yellow and grey panels truthful", () => {
  const data = new Uint8ClampedArray([...px(230, 190, 0), ...px(64, 64, 64), ...px(240, 240, 240)]);
  toPaper(data, 3, 1);
  const at = i => [data[i * 4], data[i * 4 + 1], data[i * 4 + 2]];
  const near = (got, want) => got.every((v, k) => Math.abs(v - want[k]) <= 6);
  assert.ok(near(at(0), YELLOW), `yellow ${at(0)}`);
  assert.ok(near(at(1), GREY), `grey ${at(1)}`);
  assert.ok(near(at(2), PAPER), `grey panel paper ${at(2)}`);
});
