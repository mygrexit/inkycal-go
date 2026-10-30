// app.js is the DOM glue and cannot run under node; these checks catch the breakage a
// refactor of it would otherwise only show in the browser.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

const read = name => readFileSync(new URL(name, import.meta.url), "utf8");
const app = read("./app.js");
const html = read("./index.html");

test("every #id app.js looks up exists in index.html or is set by app.js", () => {
  const ids = new Set([...html.matchAll(/\sid="([^"]+)"/g), ...app.matchAll(/\.id = "([\w-]+)";/g)].map(m => m[1]));
  const used = new Set([...app.matchAll(/\$\("#([\w-]+)"\)/g)].map(m => m[1]));
  assert.ok(used.size > 10, `only ${used.size} lookups found; did the $() helper change?`);
  const missing = [...used].filter(id => !ids.has(id));
  assert.deepEqual(missing, [], "ids missing from index.html");
});

test("every name app.js imports is exported by its module", async () => {
  const imports = [...app.matchAll(/^import \{([^}]+)\} from "\.\/([\w-]+\.js)";$/gm)];
  assert.ok(imports.length >= 7, `only ${imports.length} imports found`);
  for (const [, names, file] of imports) {
    const mod = await import(`./${file}`);
    for (const name of names.split(",").map(n => n.trim()).filter(Boolean)) {
      assert.notEqual(mod[name], undefined, `${file} does not export ${name}`);
    }
  }
});

test("every API route app.js calls is served by the builder", () => {
  const routes = new Set([...read("../builder.go").matchAll(/mux\.Handle(?:Func)?\("([A-Z]+ \/[^"]*)"/g)].map(m => m[1]));
  const calls = [...app.matchAll(/api\("([A-Z]+)", *[`"](\/api\/[\w-]+)/g)].map(m => `${m[1]} ${m[2]}`);
  assert.ok(calls.length >= 5, `only ${calls.length} API calls found`);
  // the font upload uses fetch directly: PUT /api/fonts/{name}
  calls.push("PUT /api/fonts/{name}");
  const missing = calls.filter(c => !routes.has(c));
  assert.deepEqual(missing, [], "routes missing from builder.go");
});
