import { test } from "node:test";
import assert from "node:assert/strict";
import { groupPanels, ORIENTATIONS } from "./panels.js";

const list = [
  { model: "epd_4_in_2", name: "Waveshare 4.2″ · black, white", group: "4″", inches: 4.2 },
  { model: "epd_7_in_5_v2", name: "Waveshare 7.5″ V2 · black, white", group: "7–8″", inches: 7.5 },
  { model: "epd_7_in_5_v2_colour", name: "Waveshare 7.5″ V2 (B) · black, white, red", group: "7–8″", inches: 7.5 },
  { model: "epd_12_in_48_colour_V2", name: "Waveshare 12.48″ V2 · black, white, red", group: "12–13″", inches: 12.48 },
];

test("groupPanels keeps the size order and groups", () => {
  const g = groupPanels(list, "");
  assert.deepEqual(g.map(x => x.group), ["4″", "7–8″", "12–13″"]);
  assert.equal(g[1].panels.length, 2);
});

test("groupPanels searches names and model codes, ignoring case and spaces", () => {
  assert.deepEqual(groupPanels(list, "red").flatMap(x => x.panels.map(p => p.model)), ["epd_7_in_5_v2_colour", "epd_12_in_48_colour_V2"]);
  assert.deepEqual(groupPanels(list, "12.48").flatMap(x => x.panels.map(p => p.model)), ["epd_12_in_48_colour_V2"]);
  assert.deepEqual(groupPanels(list, "EPD_4").flatMap(x => x.panels.map(p => p.model)), ["epd_4_in_2"]);
  assert.deepEqual(groupPanels(list, "7.5 red").flatMap(x => x.panels.map(p => p.model)), ["epd_7_in_5_v2_colour"]);
});

test("orientations are worded for people", () => {
  assert.deepEqual(ORIENTATIONS.map(o => o.value), [0, 180]);
});
