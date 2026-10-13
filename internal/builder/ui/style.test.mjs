import { test } from "node:test";
import assert from "node:assert/strict";
import { PRESETS, inkOptions, presetsFor, effective, matchPreset, cleanStyle, fontChoice, applyFont } from "./style.js";

test("inkOptions follow the panel", () => {
  assert.deepEqual(inkOptions("bw"), ["white", "black"]);
  assert.deepEqual(inkOptions("bwr"), ["white", "black", "colour"]);
  assert.deepEqual(inkOptions("gray16"), ["white", "black", "colour"]);
});

test("presetsFor drops presets a b/w panel cannot show", () => {
  const bw = presetsFor("bw").map(p => p.id);
  assert.ok(bw.includes("standard") && bw.includes("inverted"));
  assert.ok(!bw.includes("colour-tile"));
  assert.equal(presetsFor("bwr").length, PRESETS.length);
});

test("effective merges tile over page, field by field", () => {
  const page = { font: "serif", scale: 1.2, colors: { accent: "black" } };
  const tile = { colors: { paper: "black", ink: "white" } };
  assert.deepEqual(effective(page, tile), { font: "serif", scale: 1.2, custom_fonts: {}, colors: { paper: "black", ink: "white", accent: "black" } });
  assert.deepEqual(effective(undefined, undefined).colors, { paper: "white", ink: "black", accent: "colour" });
});

test("matchPreset recognises preset colours", () => {
  assert.equal(matchPreset({ paper: "black", ink: "white", accent: "colour" }), "inverted");
  assert.equal(matchPreset({ paper: "white", ink: "colour", accent: "colour" }), "custom");
});

test("cleanStyle drops empty parts so settings.json stays tidy", () => {
  assert.equal(cleanStyle({ font: "", scale: 0, colors: {}, custom_fonts: {} }), undefined);
  assert.deepEqual(cleanStyle({ font: "serif", scale: 1, colors: { paper: "", ink: "white" } }), { font: "serif", scale: 1, colors: { ink: "white" } });
});

test("fonts: a custom file goes into every text role", () => {
  const st = applyFont({}, "fonts/Hand.ttf");
  assert.equal(st.font, "custom");
  assert.deepEqual(st.custom_fonts, { text: "fonts/Hand.ttf", strong: "fonts/Hand.ttf", bold: "fonts/Hand.ttf", display: "fonts/Hand.ttf" });
  assert.equal(fontChoice(st), "fonts/Hand.ttf");
  const back = applyFont(st, "serif");
  assert.equal(back.font, "serif");
  assert.equal(back.custom_fonts, undefined);
  assert.equal(fontChoice(applyFont(st, "")), "");
});
