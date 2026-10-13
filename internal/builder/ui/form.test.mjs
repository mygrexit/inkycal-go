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
