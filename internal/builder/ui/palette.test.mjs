import { test } from "node:test";
import assert from "node:assert/strict";
import { paletteGroups, tileTitle } from "./palette.js";

test("the palette groups modules in a fixed order and skips empty groups", () => {
  const catalog = {
    modules: [
      { name: "Calendar", title: "Calendar", group: "calendar", icon: "📅" },
      { name: "Weather", title: "Weather", group: "weather", icon: "🌤" },
      { name: "Odd", title: "Odd", group: "unknown", icon: "?" },
    ],
  };
  const groups = paletteGroups(catalog);
  assert.deepEqual(groups.map(g => g.id), ["weather", "calendar", "info"]);
  assert.equal(groups[2].entries[0].title, "Odd", "unknown groups land in info");
  assert.deepEqual(groups[0].entries[0].config, {});
});

test("tileTitle is the module's title, or its name when unknown", () => {
  assert.equal(tileTitle({ title: "Calendar" }, { name: "Calendar", config: {} }), "Calendar");
  assert.equal(tileTitle(undefined, { name: "Webshot", config: {} }), "Webshot");
});
