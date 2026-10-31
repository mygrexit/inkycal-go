import { test } from "node:test";
import assert from "node:assert/strict";
import { coerce, display, visible, missing, stripSecrets } from "./form.js";

test("coerce keeps JSON types: numeric select options stay numbers", () => {
  const sel = { kind: "select", options: [24, 12] };
  assert.equal(coerce(sel, "12"), 12);
  assert.equal(coerce({ kind: "select", options: ["metric", "imperial"] }, "imperial"), "imperial");
  assert.equal(coerce({ kind: "number" }, "3.5"), 3.5);
  assert.equal(coerce({ kind: "number" }, ""), undefined);
  assert.equal(coerce({ kind: "number" }, "abc"), undefined);
  assert.equal(coerce({ kind: "bool" }, true), true);
  assert.equal(coerce({ kind: "text" }, "  Berlin  "), "Berlin");
  assert.equal(coerce({ kind: "text" }, "   "), undefined);
});

test("lists are one entry per line in the form and comma-separated in settings.json", () => {
  const f = { kind: "url_list" };
  assert.equal(coerce(f, "https://a.ics\n\n https://b.ics \n"), "https://a.ics, https://b.ics");
  assert.equal(display(f, "https://a.ics, https://b.ics"), "https://a.ics\nhttps://b.ics");
  assert.equal(display(f, ["x", "y"]), "x\ny");
  assert.equal(display({ kind: "text_list" }, null), "");
  assert.equal(coerce({ kind: "text_list" }, ""), undefined);
});

test("visible honours show_if", () => {
  const f = { key: "url", show_if: { key: "source.type", values: ["json"] } };
  assert.equal(visible(f, { "source.type": "json" }), true);
  assert.equal(visible(f, { "source.type": "csv" }), false);
  assert.equal(visible({ key: "x" }, {}), true);
});

test("missing lists required visible fields without a value", () => {
  const meta = { fields: [
    { key: "api_key", kind: "secret", required: true },
    { key: "location", kind: "text", required: true },
    { key: "units", kind: "select", options: ["metric"] },
  ] };
  assert.deepEqual(missing(meta, { api_key: "k" }), ["location"]);
  assert.deepEqual(missing(meta, { api_key: "k", location: "Berlin" }), []);
});

test("stripSecrets blanks secret fields only", () => {
  const meta = { fields: [{ key: "api_key", kind: "secret" }, { key: "location", kind: "text" }] };
  assert.deepEqual(stripSecrets(meta, { api_key: "k", location: "B", size: [1, 2] }), { api_key: "", location: "B", size: [1, 2] });
});

test("partition puts advanced fields behind More options", async () => {
  const { partition } = await import("./form.js");
  const meta = { fields: [{ key: "a" }, { key: "b", advanced: true }, { key: "c", show_if: { key: "a", values: ["x"] } }] };
  const { basic, advanced } = partition(meta, { a: "y" });
  assert.deepEqual(basic.map(f => f.key), ["a"]);
  assert.deepEqual(advanced.map(f => f.key), ["b"]);
});

test("optionLabel shows format examples", async () => {
  const { optionLabel } = await import("./form.js");
  const examples = { de: { "D MMM": "13 Okt" } };
  assert.equal(optionLabel({ key: "date_format" }, "D MMM", examples, "de"), "13 Okt");
  assert.equal(optionLabel({ key: "units", labels: { metric: "°C, m/s" } }, "metric", examples, "de"), "°C, m/s");
  assert.equal(optionLabel({ key: "x" }, 24, examples, "de"), "24");
});

test("normalize derives the text source of upstream settings", async () => {
  const { normalize } = await import("./form.js");
  assert.deepEqual(normalize("TextToDisplay", { filepath: "https://x/t.txt" }).source, "url");
  assert.deepEqual(normalize("TextToDisplay", { filepath: "/home/pi/t.txt" }).source, "file");
  assert.deepEqual(normalize("TextToDisplay", { text: "Hi" }).source, "text");
  assert.deepEqual(normalize("TextToDisplay", { source: "file", filepath: "/x" }).source, "file");
  assert.deepEqual(normalize("Stocks", { tickers: "A" }), { tickers: "A" });
});

test("normalize keeps upstream weather on OpenWeatherMap", async () => {
  const { normalize } = await import("./form.js");
  assert.equal(normalize("Weather", { api_key: "k" }).provider, "openweathermap");
  assert.equal(normalize("Weather", { latitude: 1 }).provider, "open-meteo");
  assert.equal(normalize("Weather", { provider: "open-meteo", api_key: "k" }).provider, "open-meteo");
  assert.equal(normalize("Fullweather", { api_key: "k", latitude: 1 }).provider, "openweathermap"); // upstream
  assert.equal(normalize("Fullweather", { latitude: 1 }).provider, "open-meteo");
});

test("choosing Other… keeps the current value until something is typed", async () => {
  const { selectRead } = await import("./form.js");
  const f = { key: "date_format", kind: "select", options: ["D MMM"], custom: true };
  assert.deepEqual(selectRead(f, "__custom"), { skip: true });
  assert.deepEqual(selectRead(f, "D MMM"), { value: "D MMM" });
});
