import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { newTile, placeTile, autoFontOf } from "./tile.js";
import { autoFont } from "./grid.js";

const catalog = {
  modules: [{ name: "Weather", fields: [{ key: "provider", default: "open-meteo" }, { key: "units", default: "metric" }] }],
  tile_padding: [16, 12],
  grid: { columns: 12, rows: 16 },
  font_rule: { h_div: 9, w_div: 40, min: 12, max: 32 },
};

test("autoFont gives the sizes Go's modules.AutoFont gives (shared fixture)", () => {
  const fx = JSON.parse(readFileSync(new URL("./testdata/autofont.json", import.meta.url)));
  for (const [w, h, want] of fx.cases) assert.equal(autoFont(fx.rule, w, h), want, `${w}×${h}`);
});

test("newTile: field defaults, then the preset, then the catalog's padding and the language", () => {
  const t = newTile(catalog, "Weather", { units: "imperial", padding_x: 99, language: "fr" }, "de");
  assert.equal(t.name, "Weather");
  assert.deepEqual(t.config, { provider: "open-meteo", units: "imperial", padding_x: 16, padding_y: 12, language: "de" });
  assert.equal(t.config.cell, undefined);
  assert.deepEqual(newTile(catalog, "Unknown", {}, "en").config, { padding_x: 16, padding_y: 12, language: "en" });
});

test("placeTile sets cell, pixel size and the automatic font size", () => {
  const t = newTile(catalog, "Weather", {}, "en");
  const cell = [0, 0, 12, 4];
  placeTile(catalog, catalog.grid, t.config, cell, 984, 1304);
  assert.deepEqual(t.config.cell, cell);
  assert.notEqual(t.config.cell, cell, "a copy, not the caller's array");
  assert.deepEqual(t.config.size, [984, 326]);
  assert.equal(t.config.fontsize, autoFont(catalog.font_rule, 984 - 32, 326 - 24));
  assert.equal(autoFontOf(catalog, catalog.grid, t.config, 984, 1304), t.config.fontsize);
});
