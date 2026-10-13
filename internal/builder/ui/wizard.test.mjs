import { test } from "node:test";
import assert from "node:assert/strict";
import { estimateDays, refreshSeconds, SCHEDULES, applySchedule, applyTemplate, templateFits } from "./wizard.js";

const big = { inks: "bwr", inches: 12.48 };
const small = { inks: "bwr", inches: 7.5 };

test("battery estimate: plausible range, fewer wakes last longer", () => {
  const daily = estimateDays({ wakesPerDay: 1, panel: big, capacity: 1200 });
  assert.ok(daily > 60 && daily < 150, `daily ${daily}`);
  const hourly = estimateDays({ wakesPerDay: 17, panel: big, capacity: 1200 });
  assert.ok(hourly < daily / 5, `hourly ${hourly}`);
  assert.ok(estimateDays({ wakesPerDay: 1, panel: big, capacity: 5000 }) > daily * 3.5);
  assert.ok(refreshSeconds(big) > refreshSeconds(small) && refreshSeconds(small) > refreshSeconds({ inks: "bw", inches: 7.5 }));
});

test("schedules become update_times or an interval", () => {
  const s = applySchedule({ update_interval: 60, update_times: ["07:00"] }, SCHEDULES.find(x => x.id === "daily"));
  assert.deepEqual(s.update_times, ["06:00"]);
  const q = applySchedule({ update_times: ["06:00"] }, SCHEDULES.find(x => x.id === "quarter"));
  assert.equal(q.update_times, undefined);
  assert.equal(q.update_interval, 15);
  for (const sc of SCHEDULES) assert.ok(sc.label && (sc.keep || sc.wakesPerDay > 0));
});

test("applyTemplate builds modules with defaults, language and positions", () => {
  const catalog = {
    modules: [
      { name: "Weather", fields: [{ key: "provider", default: "open-meteo" }] },
      { name: "TextToDisplay", fields: [{ key: "source", default: "text" }] },
    ],
    template_padding: [16, 12],
    font_rule: { h_div: 9, w_div: 40, min: 12, max: 32 },
  };
  const tpl = { modules: [{ name: "Weather", cell: [0, 0, 12, 4] }, { name: "TextToDisplay", cell: [0, 4, 6, 4], config: { one_per_day: true } }] };
  const s = applyTemplate({ model: "m", modules: [{ name: "Old" }], keep: 1 }, tpl, catalog, "de", 984, 1304);
  assert.equal(s.keep, 1);
  assert.equal(s.modules.length, 2);
  assert.deepEqual(s.modules.map(m => m.position), [1, 2]);
  assert.deepEqual(s.grid, { columns: 12, rows: 16 });
  const w = s.modules[1].config;
  assert.equal(w.one_per_day, true);
  assert.equal(w.source, "text");
  assert.equal(w.language, "de");
  assert.equal(w.padding_x, 16);
  assert.deepEqual(w.cell, [0, 4, 6, 4]);
  assert.ok(w.fontsize >= 12);
  assert.equal(s.modules[0].config.provider, "open-meteo");
});

test("templateFits checks every module's minimum on the chosen display", () => {
  const catalog = { modules: [{ name: "Greek", min_w: 300, min_h: 60 }], template_padding: [16, 12] };
  const tpl = { modules: [{ name: "Greek", cell: [0, 0, 8, 2] }] };
  assert.equal(templateFits(tpl, catalog, 984, 1304), true);  // 8 cols = 656 px
  assert.equal(templateFits(tpl, catalog, 480, 800), false);  // 8 cols = 320 − 32 < 300
});

test("the assistant keeps a schedule it has no preset for", async () => {
  const { pickSchedule, applySchedule, SCHEDULES } = await import("./wizard.js");
  assert.equal(pickSchedule({ update_times: ["06:00", "18:00"] }, false), "twice");
  assert.equal(pickSchedule({ update_times: ["07:30"] }, false), "keep");
  assert.equal(pickSchedule({ update_interval: 60 }, false), "keep");
  assert.equal(pickSchedule({ update_interval: 60 }, true), "daily", "a new file starts with the battery-friendly default");
  const keep = SCHEDULES.find(s => s.id === "keep");
  const s = { update_times: ["07:30"], update_interval: 60 };
  assert.deepEqual(applySchedule(s, keep), s);
});
