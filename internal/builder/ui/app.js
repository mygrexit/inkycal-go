// inkycal builder: places modules on the grid and shows the page as the panel renders it.
import { cellRect, fits, minSpans, bestFree, fromLegacy, moveCell, resizeCell, autoFont } from "./grid.js";
import { coerce, display, visible, missing } from "./form.js";
import { toPaper } from "./paper.js";

const $ = sel => document.querySelector(sel);
const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
const LANGS = ["en", "de"];
const COMMON_KEYS = ["size", "cell", "padding_x", "padding_y", "fontsize", "language"];

const state = {
  catalog: null,
  settings: null,
  selected: -1,
  history: [],
  future: [],
  saved: "",
  errors: {}, // module index → error text
  view: "paper",
  zoom: "fit",
  scale: 1,
  lastImage: null,
};

// ---------- API ----------

async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: { "X-Inkycal-Token": token, "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) throw new Error((await res.text()).trim() || res.statusText);
  return res;
}

// ---------- geometry ----------

const grid = () => state.settings.grid || state.catalog.grid;
const modules = () => state.settings.modules;
const metaOf = name => state.catalog.modules.find(m => m.name === name);

function page() {
  const [w, h] = state.catalog.models[state.settings.model] || [800, 480];
  const pageW = h, pageH = w; // portrait, as the panel lays it out
  const info = state.settings.info_section ? Number(state.settings.info_section_height) || 70 : 0;
  return { pageW, pageH, info, areaH: pageH - info };
}

function spansFor(entry) {
  const p = page(), meta = metaOf(entry.name) || { min_w: 1, min_h: 1 };
  const c = entry.config;
  return minSpans(grid(), meta, p.pageW, p.areaH, Number(c.padding_x) || 0, Number(c.padding_y) || 0);
}

function innerSize(entry) {
  const p = page(), r = cellRect(grid(), entry.config.cell, p.pageW, p.areaH);
  const c = entry.config;
  return [r.w - 2 * (Number(c.padding_x) || 0), r.h - 2 * (Number(c.padding_y) || 0)];
}

// syncSizes writes each module's pixel size next to its cell (upstream Inkycal reads it).
function syncSizes() {
  const p = page();
  for (const m of modules()) {
    if (!m.config.cell) continue;
    const r = cellRect(grid(), m.config.cell, p.pageW, p.areaH);
    m.config.size = [r.w, r.h];
  }
}

// ---------- history ----------

function snapshot() {
  return JSON.stringify(state.settings);
}

// commit records the current settings as one undo step and re-renders.
function commit() {
  syncSizes();
  const snap = snapshot();
  if (state.history[state.history.length - 1] !== snap) {
    state.history.push(snap);
    if (state.history.length > 100) state.history.shift();
    state.future = [];
  }
  renderAll();
  schedulePreview();
}

function restore(snap) {
  state.settings = JSON.parse(snap);
  if (state.selected >= modules().length) state.selected = -1;
  renderAll();
  schedulePreview();
}

function undo() {
  if (state.history.length < 2) return;
  state.future.push(state.history.pop());
  restore(state.history[state.history.length - 1]);
}

function redo() {
  if (!state.future.length) return;
  const snap = state.future.pop();
  state.history.push(snap);
  restore(snap);
}

// ---------- preview ----------

let previewTimer = 0, inFlight = false, pending = false, refreshNext = false;

function schedulePreview(delay = 300) {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(runPreview, delay);
}

async function runPreview() {
  if (inFlight) {
    pending = true;
    return;
  }
  inFlight = true;
  $("#busy").hidden = false;
  const refresh = refreshNext;
  refreshNext = false;
  try {
    const res = await api("POST", "/api/preview" + (refresh ? "?refresh=1" : ""), state.settings);
    const data = await res.json();
    state.errors = {};
    for (const e of data.errors) state.errors[e.position - 1] = e.error;
    showImage(data.image);
    setStatus(isDirty() ? "unsaved changes" : "saved", isDirty() ? "dirty" : "");
  } catch (err) {
    setStatus(err.message, "error");
  } finally {
    inFlight = false;
    $("#busy").hidden = true;
    if (!drag) renderTiles(); // never pull the tile out from under a drag
    if (state.selected >= 0) renderModuleError();
    if (pending) {
      pending = false;
      runPreview();
    }
  }
}

function showImage(src) {
  const img = $("#pixels");
  img.onload = () => {
    const canvas = $("#paper");
    canvas.width = img.naturalWidth;
    canvas.height = img.naturalHeight;
    const ctx = canvas.getContext("2d", { willReadFrequently: true });
    ctx.drawImage(img, 0, 0);
    const data = ctx.getImageData(0, 0, canvas.width, canvas.height);
    toPaper(data.data, canvas.width, canvas.height);
    ctx.putImageData(data, 0, 0);
  };
  img.src = src;
}

// ---------- rendering ----------

function setStatus(text, kind = "") {
  const s = $("#status");
  s.textContent = text;
  s.className = "muted " + kind;
}

const isDirty = () => snapshot() !== state.saved;

function layoutPage() {
  const p = page();
  const stage = $("#stage");
  const fit = Math.min((stage.clientWidth - 80) / p.pageW, (stage.clientHeight - 80) / p.pageH);
  state.scale = state.zoom === "fit" ? Math.max(0.1, fit) : Number(state.zoom);
  const el = $("#page");
  el.style.width = `${p.pageW * state.scale}px`;
  el.style.height = `${p.pageH * state.scale}px`;
  const info = $("#info-area");
  info.hidden = p.info === 0;
  info.style.height = `${p.info * state.scale}px`;
  const lines = $("#gridlines");
  lines.replaceChildren();
  const g = grid();
  for (let c = 0; c < g.columns; c++) {
    for (let r = 0; r < g.rows; r++) {
      const rect = cellRect(g, [c, r, 1, 1], p.pageW, p.areaH);
      const d = document.createElement("div");
      place(d, rect);
      lines.append(d);
    }
  }
}

function place(el, rect) {
  const s = state.scale;
  el.style.left = `${rect.x * s}px`;
  el.style.top = `${rect.y * s}px`;
  el.style.width = `${rect.w * s}px`;
  el.style.height = `${rect.h * s}px`;
}

function renderTiles() {
  const p = page();
  const box = $("#tiles");
  box.replaceChildren();
  modules().forEach((m, i) => {
    if (!m.config.cell) return;
    const meta = metaOf(m.name);
    const t = document.createElement("div");
    t.className = "tile" + (i === state.selected ? " selected" : "") + (state.errors[i] ? " error" : "");
    t.tabIndex = 0;
    t.setAttribute("role", "button");
    const [c, r, cs, rs] = m.config.cell;
    t.setAttribute("aria-label", `${meta ? meta.title : m.name}, column ${c + 1}, row ${r + 1}, ${cs} × ${rs}`);
    t.dataset.index = i;
    if (state.errors[i]) t.title = state.errors[i];
    place(t, cellRect(grid(), m.config.cell, p.pageW, p.areaH));
    const label = document.createElement("span");
    label.className = "label";
    label.textContent = (meta ? meta.title : m.name) + (state.errors[i] ? " — error" : "");
    t.append(label);
    for (const edge of ["n", "s", "e", "w", "ne", "nw", "se", "sw"]) {
      const h = document.createElement("div");
      h.className = "handle";
      h.dataset.edge = edge;
      t.append(h);
    }
    t.addEventListener("pointerdown", startDrag);
    t.addEventListener("focus", () => {
      if (state.selected !== i) select(i);
    });
    box.append(t);
  });
}

function renderPalette() {
  const list = $("#module-list");
  list.replaceChildren();
  for (const meta of state.catalog.modules) {
    const li = document.createElement("li");
    li.draggable = true;
    li.tabIndex = 0;
    li.setAttribute("role", "button");
    const title = document.createElement("strong");
    title.textContent = meta.title;
    const desc = document.createElement("span");
    desc.textContent = meta.description;
    li.append(title, desc);
    li.addEventListener("click", () => addModule(meta, null));
    li.addEventListener("keydown", e => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        addModule(meta, null);
      }
    });
    li.addEventListener("dragstart", e => e.dataTransfer.setData("text/x-inkycal-module", meta.name));
    list.append(li);
  }
}

function renderAll() {
  $("#undo").disabled = state.history.length < 2;
  $("#redo").disabled = !state.future.length;
  setStatus(isDirty() ? "unsaved changes" : "saved", isDirty() ? "dirty" : "");
  layoutPage();
  renderTiles();
  renderInspector();
}

// ---------- adding, selecting, deleting ----------

function select(i) {
  state.selected = i;
  renderTiles();
  renderInspector();
  if (i >= 0) document.querySelector(`.tile[data-index="${i}"]`)?.focus({ preventScroll: true });
}

function defaultLanguage() {
  const used = modules().map(m => m.config.language).find(Boolean);
  if (used) return used;
  return navigator.language && navigator.language.startsWith("de") ? "de" : "en";
}

function addModule(meta, at) {
  const config = { ...Object.fromEntries((meta.fields || []).filter(f => f.default !== undefined).map(f => [f.key, f.default])) };
  config.padding_x = 20;
  config.padding_y = 16;
  config.language = defaultLanguage();
  const entry = { position: 0, name: meta.name, config };
  const [cs, rs] = spansFor({ name: meta.name, config });
  const cells = modules().map(m => m.config.cell);
  let cell = at ? [at[0], at[1], cs, rs] : null;
  if (cell) cell = moveCell(cell, 0, 0, grid());
  // a click (or a drop on a taken spot) gets the largest free box up to a full-width band
  if (!cell || !fits(grid(), cells, cell, -1)) cell = bestFree(grid(), cells, [cs, rs], [grid().columns, Math.max(rs, 4)]);
  if (!cell) {
    setStatus(`no free space for ${meta.title} (needs ${cs} × ${rs} cells)`, "error");
    return;
  }
  config.cell = cell;
  modules().push(entry);
  renumber();
  config.fontsize = autoFont(state.catalog.font_rule, ...innerSize(entry));
  state.selected = modules().length - 1;
  commit();
}

function renumber() {
  modules().forEach((m, i) => (m.position = i + 1));
}

function removeSelected() {
  if (state.selected < 0) return;
  modules().splice(state.selected, 1);
  renumber();
  state.selected = -1;
  commit();
}

// ---------- drag and resize ----------

let drag = null;

function startDrag(e) {
  if (e.button !== 0) return;
  const tile = e.currentTarget;
  const i = Number(tile.dataset.index);
  const entry = modules()[i];
  if (state.selected !== i) select(i);
  const edge = e.target.classList.contains("handle") ? e.target.dataset.edge : "";
  drag = { i, edge, x: e.clientX, y: e.clientY, from: entry.config.cell, to: entry.config.cell, min: spansFor(entry), tile: null };
  const t = document.querySelector(`.tile[data-index="${i}"]`);
  drag.tile = t;
  t.setPointerCapture(e.pointerId);
  t.addEventListener("pointermove", moveDrag);
  t.addEventListener("pointerup", endDrag);
  t.addEventListener("pointercancel", cancelDrag);
  document.body.classList.add("dragging");
  e.preventDefault();
}

function moveDrag(e) {
  if (!drag) return;
  const p = page(), g = grid();
  const dc = Math.round((e.clientX - drag.x) / ((p.pageW / g.columns) * state.scale));
  const dr = Math.round((e.clientY - drag.y) / ((p.areaH / g.rows) * state.scale));
  drag.to = drag.edge ? resizeCell(drag.from, drag.edge, dc, dr, drag.min, g) : moveCell(drag.from, dc, dr, g);
  const ok = fits(g, modules().map(m => m.config.cell), drag.to, drag.i);
  place(drag.tile, cellRect(g, drag.to, p.pageW, p.areaH));
  drag.tile.classList.toggle("invalid", !ok);
}

function endDrag() {
  if (!drag) return;
  const { i, from, to } = drag;
  const ok = fits(grid(), modules().map(m => m.config.cell), to, i);
  finishDrag();
  if (ok && String(from) !== String(to)) applyCell(i, to);
  else renderTiles();
}

function cancelDrag() {
  finishDrag();
  renderTiles();
}

function finishDrag() {
  drag.tile.removeEventListener("pointermove", moveDrag);
  drag.tile.removeEventListener("pointerup", endDrag);
  drag.tile.removeEventListener("pointercancel", cancelDrag);
  drag = null;
  document.body.classList.remove("dragging");
}

// applyCell moves or resizes a module; a font size that was automatic stays automatic.
function applyCell(i, cell) {
  const entry = modules()[i];
  const wasAuto = Number(entry.config.fontsize) === autoFont(state.catalog.font_rule, ...innerSize(entry));
  entry.config.cell = cell;
  if (wasAuto) entry.config.fontsize = autoFont(state.catalog.font_rule, ...innerSize(entry));
  commit();
}

function cellAt(clientX, clientY) {
  const p = page(), g = grid();
  const box = $("#page").getBoundingClientRect();
  const x = (clientX - box.left) / state.scale, y = (clientY - box.top) / state.scale;
  return [Math.floor((x / p.pageW) * g.columns), Math.floor((y / p.areaH) * g.rows)];
}

// ---------- inspector ----------

function fieldInput(field, value, onChange) {
  const wrap = document.createElement("label");
  wrap.className = "field" + (field.kind === "bool" ? " check" : "");
  const title = document.createElement("span");
  title.textContent = field.label + (field.required ? " *" : "");
  let input;
  const shown = display(field, value);
  switch (field.kind) {
    case "bool":
      input = document.createElement("input");
      input.type = "checkbox";
      input.checked = Boolean(shown);
      wrap.append(input, title);
      break;
    case "select":
      input = document.createElement("select");
      if (!field.required) input.append(new Option("(default)", ""));
      for (const o of field.options) input.append(new Option(String(o), String(o)));
      input.value = shown === "" ? "" : String(shown);
      wrap.append(title, input);
      break;
    case "url_list":
    case "text_list":
      input = document.createElement("textarea");
      input.value = shown;
      input.placeholder = "one per line";
      wrap.append(title, input);
      break;
    case "secret": {
      input = document.createElement("input");
      input.type = "password";
      input.value = shown;
      input.autocomplete = "off";
      const row = document.createElement("div");
      row.className = "secret";
      const eye = document.createElement("button");
      eye.type = "button";
      eye.textContent = "Show";
      eye.addEventListener("click", e => {
        e.preventDefault();
        input.type = input.type === "password" ? "text" : "password";
        eye.textContent = input.type === "password" ? "Show" : "Hide";
      });
      row.append(input, eye);
      wrap.append(title, row);
      break;
    }
    default:
      input = document.createElement("input");
      input.type = field.kind === "number" ? "number" : field.kind === "url" ? "url" : "text";
      input.value = shown;
      wrap.append(title, input);
  }
  if (field.help) {
    const help = document.createElement("small");
    help.textContent = field.help;
    wrap.append(help);
  }
  const read = () => coerce(field, field.kind === "bool" ? input.checked : input.value);
  // typing updates the preview; leaving the field records one undo step
  input.addEventListener("input", () => {
    onChange(read(), false);
  });
  input.addEventListener("change", () => onChange(read(), true));
  return wrap;
}

function setValue(obj, key, value) {
  if (value === undefined) delete obj[key];
  else obj[key] = value;
}

function renderInspector() {
  const box = $("#inspector");
  box.replaceChildren();
  if (state.selected < 0 || !modules()[state.selected]) {
    renderPageSettings(box);
    return;
  }
  const entry = modules()[state.selected];
  const meta = metaOf(entry.name);
  const h = document.createElement("h3");
  h.textContent = meta ? meta.title : entry.name;
  box.append(h);
  if (meta) {
    const p = document.createElement("p");
    p.className = "muted small";
    p.textContent = meta.description;
    box.append(p);
  }
  const err = document.createElement("div");
  err.id = "module-error";
  box.append(err);
  renderModuleError();

  const values = entry.config;
  const absent = meta ? missing(meta, values) : [];
  if (meta) {
    for (const f of meta.fields || []) {
      if (!visible(f, values)) continue;
      const el = fieldInput(f, values[f.key], (v, record) => {
        setValue(values, f.key, v);
        if (record) commit();
        else schedulePreview();
      });
      if (absent.includes(f.key)) {
        el.classList.add("missing");
        const m = document.createElement("span");
        m.className = "err";
        m.textContent = "required";
        el.append(m);
      }
      box.append(el);
    }
  } else {
    const p = document.createElement("p");
    p.className = "muted";
    p.textContent = "No editor for this module: edit its settings as JSON.";
    box.append(p);
    box.append(rawEditor(entry));
  }

  const layout = document.createElement("fieldset");
  const legend = document.createElement("legend");
  legend.textContent = "Layout";
  layout.append(legend);
  const [iw, ih] = innerSize(entry);
  const auto = autoFont(state.catalog.font_rule, iw, ih);
  const num = (key, label) =>
    fieldInput({ key, label, kind: "number" }, values[key], (v, record) => {
      setValue(values, key, v);
      if (record) commit();
      else schedulePreview();
    });
  layout.append(num("padding_x", "Padding left/right (px)"), num("padding_y", "Padding top/bottom (px)"));
  const font = num("fontsize", "Font size (px)");
  const autoBtn = document.createElement("button");
  autoBtn.type = "button";
  autoBtn.textContent = `Auto (${auto} px)`;
  autoBtn.addEventListener("click", () => {
    values.fontsize = auto;
    commit();
  });
  font.append(autoBtn);
  layout.append(font);
  layout.append(
    fieldInput({ key: "language", label: "Language", kind: "select", options: LANGS, required: true }, values.language || "en", (v, record) => {
      setValue(values, "language", v);
      if (record) commit();
    }),
  );
  const info = document.createElement("p");
  info.className = "muted small";
  const [c, r, cs, rs] = values.cell;
  info.textContent = `Cell: column ${c + 1}, row ${r + 1}, ${cs} × ${rs} · inner ${iw} × ${ih} px`;
  layout.append(info);
  box.append(layout);

  const actions = document.createElement("div");
  actions.className = "actions";
  const del = document.createElement("button");
  del.type = "button";
  del.className = "danger";
  del.textContent = "Remove module";
  del.addEventListener("click", removeSelected);
  const done = document.createElement("button");
  done.type = "button";
  done.textContent = "Page settings";
  done.addEventListener("click", () => select(-1));
  actions.append(del, done);
  box.append(actions);
}

function renderModuleError() {
  const el = $("#module-error");
  if (!el) return;
  const text = state.errors[state.selected];
  el.className = text ? "module-error" : "";
  el.textContent = text || "";
}

function rawEditor(entry) {
  const wrap = document.createElement("label");
  wrap.className = "field";
  const area = document.createElement("textarea");
  area.rows = 12;
  const rest = Object.fromEntries(Object.entries(entry.config).filter(([k]) => !COMMON_KEYS.includes(k)));
  area.value = JSON.stringify(rest, null, 2);
  area.addEventListener("change", () => {
    try {
      const v = JSON.parse(area.value);
      for (const k of Object.keys(entry.config)) if (!COMMON_KEYS.includes(k)) delete entry.config[k];
      Object.assign(entry.config, v);
      commit();
    } catch (err) {
      setStatus("invalid JSON: " + err.message, "error");
    }
  });
  wrap.append(area);
  return wrap;
}

function renderPageSettings(box) {
  const h = document.createElement("h3");
  h.textContent = "Page";
  const p = document.createElement("p");
  p.className = "muted small";
  p.textContent = "Select a module on the panel to edit it.";
  box.append(h, p);
  const s = state.settings;
  const on = key => (v, record) => {
    setValue(s, key, v);
    if (record) commit();
    else schedulePreview();
  };
  const models = Object.keys(state.catalog.models).sort();
  box.append(fieldInput({ key: "model", label: "Display", kind: "select", options: models, required: true }, s.model, (v, record) => {
    if (!record) return;
    s.model = v;
    reflow();
  }));
  box.append(fieldInput({ key: "orientation", label: "Orientation", kind: "select", options: [0, 180], required: true }, s.orientation ?? 0, on("orientation")));
  box.append(fieldInput({ key: "update_times", label: "Update times", kind: "text_list", help: "HH:MM, one per line; empty uses the interval" },
    s.update_times, (v, record) => {
      setValue(s, "update_times", v === undefined ? undefined : v.split(", "));
      if (record) commit();
    }));
  box.append(fieldInput({ key: "update_interval", label: "Update interval (minutes)", kind: "number" }, s.update_interval, on("update_interval")));

  const look = document.createElement("fieldset");
  const legend = document.createElement("legend");
  legend.textContent = "Look";
  look.append(legend);
  look.append(fieldInput({ key: "dividers", label: "Hairlines between modules", kind: "bool" }, s.dividers, on("dividers")));
  look.append(fieldInput({ key: "border_around_modules", label: "Border around modules", kind: "bool" }, s.border_around_modules, on("border_around_modules")));
  look.append(fieldInput({ key: "info_section", label: "Info line (time, battery)", kind: "bool" }, s.info_section, (v, record) => {
    setValue(s, "info_section", v);
    if (record) reflow();
  }));
  if (s.info_section) {
    look.append(fieldInput({ key: "info_section_height", label: "Info line height (px)", kind: "number" }, s.info_section_height, (v, record) => {
      setValue(s, "info_section_height", v);
      if (record) reflow();
    }));
  }
  box.append(look);

  const adv = document.createElement("fieldset");
  const legend2 = document.createElement("legend");
  legend2.textContent = "Panel";
  adv.append(legend2);
  adv.append(fieldInput({ key: "image_hash", label: "Skip refresh when nothing changed", kind: "bool" }, s.image_hash, on("image_hash")));
  adv.append(fieldInput({ key: "disable_calibration", label: "Never run calibration", kind: "bool" }, s.disable_calibration, on("disable_calibration")));
  const g = grid();
  const gridField = (key, label) =>
    fieldInput({ key, label, kind: "number", help: "Changing it keeps the cells; move tiles that no longer fit" }, g[key], (v, record) => {
      if (!record || !v || v < 1) return;
      state.settings.grid = { ...g, [key]: Math.round(v) };
      reflow();
    });
  adv.append(gridField("columns", "Grid columns"), gridField("rows", "Grid rows"));
  box.append(adv);
}

// reflow keeps every module inside the (possibly changed) page and grid.
function reflow() {
  const g = grid();
  for (const m of modules()) {
    if (!m.config.cell) continue;
    let [c, r, cs, rs] = m.config.cell;
    cs = Math.min(cs, g.columns);
    rs = Math.min(rs, g.rows);
    m.config.cell = [Math.min(c, g.columns - cs), Math.min(r, g.rows - rs), cs, rs];
  }
  commit();
}

// ---------- keyboard ----------

function onKey(e) {
  const typing = ["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement?.tagName);
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "z" && !typing) {
    e.preventDefault();
    e.shiftKey ? redo() : undo();
    return;
  }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
    e.preventDefault();
    save();
    return;
  }
  if (typing || state.selected < 0) return;
  const entry = modules()[state.selected];
  const g = grid();
  const step = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] }[e.key];
  if (step) {
    e.preventDefault();
    const cell = e.shiftKey ? resizeCell(entry.config.cell, "se", step[0], step[1], spansFor(entry), g) : moveCell(entry.config.cell, step[0], step[1], g);
    if (fits(g, modules().map(m => m.config.cell), cell, state.selected)) applyCell(state.selected, cell);
  } else if (e.key === "Delete" || e.key === "Backspace") {
    e.preventDefault();
    removeSelected();
  } else if (e.key === "Escape") {
    select(-1);
  }
}

// ---------- load and save ----------

async function save() {
  try {
    syncSizes();
    await api("PUT", "/api/settings", state.settings);
    state.saved = snapshot();
    const absent = modules().filter(m => metaOf(m.name) && missing(metaOf(m.name), m.config).length).length;
    setStatus(absent ? `saved — ${absent} module(s) still miss required settings` : "saved", absent ? "error" : "");
  } catch (err) {
    setStatus("not saved: " + err.message, "error");
  }
}

function banner(text) {
  const b = $("#banner");
  b.textContent = text;
  b.hidden = !text;
}

// toGrid converts a stacked (upstream) layout into full-width grid tiles.
function toGrid() {
  const ms = modules();
  if (!ms.length || ms.every(m => Array.isArray(m.config?.cell))) return false;
  state.settings.grid = state.settings.grid || { ...state.catalog.grid };
  const p = page();
  const minRows = ms.map(m => spansFor({ name: m.name, config: { ...m.config, cell: [0, 0, 1, 1] } })[1]);
  const cells = fromLegacy(ms.map(m => (m.config.size || [0, 0])[1]), p.areaH, grid(), minRows);
  ms.forEach((m, i) => (m.config.cell = cells[i]));
  return true;
}

async function load() {
  if (!token) {
    document.body.textContent = "Open the address the builder printed in the terminal (it contains #token=…).";
    return;
  }
  try {
    state.catalog = await (await api("GET", "/api/catalog")).json();
    const res = await api("GET", "/api/settings");
    const isNew = res.headers.get("X-Inkycal-New") === "1";
    state.settings = await res.json();
    state.settings.modules = state.settings.modules || [];
    state.settings.modules.sort((a, b) => (a.position || 0) - (b.position || 0));
    for (const m of state.settings.modules) m.config = m.config || {};
    state.saved = isNew ? "" : snapshot();
    const converted = toGrid();
    if (isNew) banner("New settings: choose your display under Page, then add modules.");
    else if (converted) banner("This layout was stacked; it is now a grid. Nothing is written until you save.");
    if (!state.catalog.models[state.settings.model]) banner(`Unknown display "${state.settings.model}": choose one under Page.`);
  } catch (err) {
    document.body.textContent = "Could not load: " + err.message;
    return;
  }
  renderPalette();
  syncSizes();
  state.history = [snapshot()];
  renderAll();
  runPreview();
}

function setView(view) {
  state.view = view;
  document.body.classList.toggle("paper", view === "paper");
  document.body.classList.toggle("pixels", view === "pixels");
  $("#view-paper").setAttribute("aria-pressed", String(view === "paper"));
  $("#view-pixels").setAttribute("aria-pressed", String(view === "pixels"));
}

function wire() {
  setView("paper");
  $("#view-paper").addEventListener("click", () => setView("paper"));
  $("#view-pixels").addEventListener("click", () => setView("pixels"));
  $("#zoom").addEventListener("change", e => {
    state.zoom = e.target.value;
    renderAll();
  });
  $("#undo").addEventListener("click", undo);
  $("#redo").addEventListener("click", redo);
  $("#save").addEventListener("click", save);
  $("#refresh").addEventListener("click", () => {
    refreshNext = true;
    runPreview();
  });
  document.addEventListener("keydown", onKey);
  window.addEventListener("resize", () => state.settings && !drag && renderAll());
  window.addEventListener("beforeunload", e => {
    if (state.settings && isDirty()) e.preventDefault();
  });
  const pageEl = $("#page");
  pageEl.addEventListener("pointerdown", e => {
    if (e.target === pageEl || e.target.id === "tiles" || e.target.tagName === "IMG" || e.target.tagName === "CANVAS") select(-1);
  });
  const stage = $("#stage");
  stage.addEventListener("dragover", e => {
    if (e.dataTransfer.types.includes("text/x-inkycal-module")) e.preventDefault();
  });
  stage.addEventListener("drop", e => {
    const name = e.dataTransfer.getData("text/x-inkycal-module");
    const meta = name && metaOf(name);
    if (!meta) return;
    e.preventDefault();
    addModule(meta, cellAt(e.clientX, e.clientY));
  });
}

wire();
load();
