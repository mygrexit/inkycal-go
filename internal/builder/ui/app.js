// inkycal builder: places modules on the grid and shows the page as the panel renders it.
import { cellRect, fits, minSpans, bestFree, makeRoom, nudge, fromLegacy, moveCell, resizeCell } from "./grid.js";
import { newTile, placeTile, autoFontOf, innerOf } from "./tile.js";
import { coerce, display, missing, partition, optionLabel, normalize, selectRead } from "./form.js";
import { toPaper } from "./paper.js";
import { groupPanels, ORIENTATIONS } from "./panels.js";
import { paletteGroups, tileTitle } from "./palette.js";
import { estimateDays, SCHEDULES, BATTERIES, applySchedule, applyTemplate, templateFits, pickSchedule, wakesFor } from "./wizard.js";
import { presetsFor, inkOptions, effective, matchPreset, cleanStyle, fontChoice, applyFont } from "./style.js";

const $ = sel => document.querySelector(sel);
const token = new URLSearchParams(location.hash.slice(1)).get("token") || "";
const LANGS = ["en", "de"];
const COMMON_KEYS = ["size", "cell", "padding_x", "padding_y", "fontsize", "language"];

let inspectorLang = "en"; // language of the selected tile, for format examples

const state = {
  catalog: null,
  settings: null,
  selected: -1,
  history: [],
  future: [],
  saved: "",
  errors: {}, // module index → error text
  view: "paper",
  fonts: [], // uploaded custom fonts
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

// ---------- DOM helpers ----------

function button(text, onClick, cls = "") {
  const b = document.createElement("button");
  b.type = "button";
  if (cls) b.className = cls;
  b.textContent = text;
  b.addEventListener("click", onClick);
  return b;
}

// optionButton is a list choice: a bold title with a line of detail below.
function optionButton(title, detail, onClick, cls = "panel-option") {
  const b = button("", onClick, cls);
  const n = document.createElement("strong");
  n.textContent = title;
  const d = document.createElement("span");
  d.textContent = detail;
  b.append(n, d);
  return b;
}

function panelButton(pn, onClick) {
  return optionButton(pn.name, `${pn.w} × ${pn.h} px` + (pn.hint ? " · " + pn.hint : ""), onClick,
    "panel-option" + (pn.model === state.settings.model ? " current" : ""));
}

function fieldset(title) {
  const fs = document.createElement("fieldset");
  const legend = document.createElement("legend");
  legend.textContent = title;
  fs.append(legend);
  return fs;
}

function small(text) {
  const el = document.createElement("small");
  el.textContent = text;
  return el;
}

// ---------- geometry ----------

const grid = () => state.settings.grid || state.catalog.grid;
const modules = () => state.settings.modules;
const metaOf = name => state.catalog.modules.find(m => m.name === name);
const cells = () => modules().map(m => m.config.cell);

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
  const p = page();
  return innerOf(grid(), entry.config, p.pageW, p.areaH);
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
    for (const e of data.errors) state.errors[e.position - 1] = e;
    showImage(data.image);
    showDirty();
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
    updateTileView();
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

function showDirty() {
  setStatus(isDirty() ? "unsaved changes" : "saved", isDirty() ? "dirty" : "");
}

function layoutPage() {
  const p = page();
  const stage = $("#stage");
  const fit = narrow()
    ? Math.min((stage.clientWidth - 32) / p.pageW, (window.innerHeight * 0.62) / p.pageH)
    : Math.min((stage.clientWidth - 80) / p.pageW, (stage.clientHeight - 80) / p.pageH);
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

const narrow = () => window.matchMedia("(max-width: 900px)").matches;

const NUDGES = [
  ["up", "↑", "Move up"], ["down", "↓", "Move down"], ["left", "←", "Move left"], ["right", "→", "Move right"],
  ["taller", "↕+", "Taller"], ["shorter", "↕−", "Shorter"], ["wider", "↔+", "Wider"], ["narrower", "↔−", "Narrower"],
];

// renderTileList is the phone way to arrange tiles: a list with move and size buttons.
function renderTileList() {
  const list = $("#tile-list");
  list.replaceChildren();
  const all = cells();
  const mins = modules().map(m => spansFor(m));
  modules().forEach((m, i) => {
    if (!m.config.cell) return;
    const li = document.createElement("li");
    li.className = i === state.selected ? "selected" : "";
    const name = button(tileTitle(metaOf(m.name), m) + (state.errors[i] ? " ⚠" : ""), () => select(i), "tile-name");
    const tools = document.createElement("span");
    tools.className = "nudges";
    for (const [action, sym, label] of NUDGES) {
      const next = nudge(grid(), all, mins, i, action);
      const b = button(sym, () => applyCell(i, next));
      b.title = label;
      b.setAttribute("aria-label", `${label}: ${name.textContent}`);
      b.disabled = !next;
      tools.append(b);
    }
    li.append(name, tools);
    list.append(li);
  });
}

function renderTiles() {
  renderTileList();
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
    t.setAttribute("aria-label", `${tileTitle(meta, m)}, column ${c + 1}, row ${r + 1}, ${cs} × ${rs}`);
    t.dataset.index = i;
    if (state.errors[i]) t.title = state.errors[i].hint || state.errors[i].error;
    place(t, cellRect(grid(), m.config.cell, p.pageW, p.areaH));
    const label = document.createElement("span");
    label.className = "label";
    label.textContent = tileTitle(meta, m) + (state.errors[i] ? " — error" : "");
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
  state.paletteEntries = [];
  for (const g of paletteGroups(state.catalog)) {
    const h = document.createElement("li");
    h.className = "group";
    h.textContent = g.title;
    list.append(h);
    for (const e of g.entries) {
      const idx = state.paletteEntries.push(e) - 1;
      const li = document.createElement("li");
      li.draggable = true;
      li.tabIndex = 0;
      li.setAttribute("role", "button");
      const title = document.createElement("strong");
      title.textContent = (e.icon ? e.icon + "  " : "") + e.title;
      const desc = document.createElement("span");
      desc.textContent = e.description || "";
      li.append(title, desc);
      li.addEventListener("click", () => addModule(e.meta, null, e.config));
      li.addEventListener("keydown", ev => {
        if (ev.key === "Enter" || ev.key === " ") {
          ev.preventDefault();
          addModule(e.meta, null, e.config);
        }
      });
      li.addEventListener("dragstart", ev => ev.dataTransfer.setData("text/x-inkycal-module", String(idx)));
      list.append(li);
    }
  }
}

function renderAll() {
  $("#display-chip").textContent = panelName();
  $("#undo").disabled = state.history.length < 2;
  $("#redo").disabled = !state.future.length;
  showDirty();
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

function addModule(meta, at, preset = {}) {
  const entry = newTile(state.catalog, meta.name, preset, defaultLanguage());
  const [cs, rs] = spansFor(entry);
  const taken = cells();
  let cell = at ? [at[0], at[1], cs, rs] : null;
  if (cell) cell = moveCell(cell, 0, 0, grid());
  // a click (or a drop on a taken spot) gets the largest free box up to a full-width band
  if (!cell || !fits(grid(), taken, cell, -1)) cell = bestFree(grid(), taken, [cs, rs], [grid().columns, Math.max(rs, 4)]);
  if (!cell) {
    // a full page: shrink the biggest tile that can spare the room
    const room = makeRoom(grid(), taken, modules().map(m => spansFor(m)), [cs, rs]);
    if (!room) {
      setStatus(`no room for ${meta.title}: make another tile smaller first`, "error");
      return;
    }
    const other = modules()[room.shrunk];
    applyCellSilently(room.shrunk, room.cells[room.shrunk]);
    cell = room.cell;
    banner(`The page was full: ${(metaOf(other.name) || { title: other.name }).title} got smaller to make room. Undo with Ctrl+Z.`);
  }
  const p = page();
  placeTile(state.catalog, grid(), entry.config, cell, p.pageW, p.areaH);
  modules().push(entry);
  renumber();
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
  if (e.pointerType === "touch" && narrow()) {
    // fingers scroll the page; tiles move with the buttons in the list below
    select(Number(e.currentTarget.dataset.index));
    return;
  }
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
  const ok = fits(g, cells(), drag.to, drag.i);
  place(drag.tile, cellRect(g, drag.to, p.pageW, p.areaH));
  drag.tile.classList.toggle("invalid", !ok);
}

function endDrag() {
  if (!drag) return;
  const { i, from, to } = drag;
  const ok = fits(grid(), cells(), to, i);
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
  applyCellSilently(i, cell);
  commit();
}

function applyCellSilently(i, cell) {
  const entry = modules()[i];
  const p = page();
  const wasAuto = Number(entry.config.fontsize) === autoFontOf(state.catalog, grid(), entry.config, p.pageW, p.areaH);
  entry.config.cell = cell;
  if (wasAuto) entry.config.fontsize = autoFontOf(state.catalog, grid(), entry.config, p.pageW, p.areaH);
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
    case "select": {
      input = document.createElement("select");
      if (!field.required) input.append(new Option("(default)", ""));
      for (const o of field.options) input.append(new Option(optionLabel(field, o, state.catalog.examples, inspectorLang), String(o)));
      const known = shown === "" || field.options.some(o => String(o) === String(shown));
      wrap.append(title, input);
      if (field.custom) {
        // a typed value for formats the list does not offer
        input.append(new Option("Other…", "__custom"));
        const own = document.createElement("input");
        own.type = "text";
        own.value = known ? "" : String(shown);
        own.hidden = known;
        own.addEventListener("change", () => onChange(own.value.trim() || undefined, true));
        input.addEventListener("change", () => {
          own.hidden = input.value !== "__custom";
          if (!own.hidden) own.focus();
        });
        wrap.append(own);
      }
      input.value = known ? (shown === "" ? "" : String(shown)) : "__custom";
      break;
    }
    case "multiline":
      input = document.createElement("textarea");
      input.value = shown;
      input.rows = 7;
      input.className = "prose";
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
      const eye = button("Show", e => {
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
  if (field.help) wrap.append(small(field.help));
  const read = () => (field.kind === "select" ? selectRead(field, input.value) : { value: coerce(field, field.kind === "bool" ? input.checked : input.value) });
  // typing updates the preview; leaving the field records one undo step
  input.addEventListener("input", () => {
    const r = read();
    if (!r.skip) onChange(r.value, false);
  });
  input.addEventListener("change", () => {
    const r = read();
    if (!r.skip) onChange(r.value, true);
  });
  return wrap;
}

function setValue(obj, key, value) {
  if (value === undefined) delete obj[key];
  else obj[key] = value;
}

// edits is a fieldInput callback for obj[key]: typing previews, leaving the field commits.
function edits(obj, key) {
  return (v, record) => {
    setValue(obj, key, v);
    if (record) commit();
    else schedulePreview();
  };
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
  h.textContent = tileTitle(meta, entry);
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
  const view = document.createElement("canvas");
  view.id = "tile-view";
  view.setAttribute("aria-label", "This tile as the panel shows it");
  box.append(view);
  updateTileView();

  const values = entry.config;
  inspectorLang = values.language || "en";
  const absent = meta ? missing(meta, values) : [];
  const more = document.createElement("details");
  more.className = "more";
  const moreTitle = document.createElement("summary");
  moreTitle.textContent = "More options";
  more.append(moreTitle);
  if (meta) {
    const { basic, advanced } = partition(meta, values);
    for (const f of [...basic, ...advanced]) {
      if (f.kind === "place") {
        const el = placeInput(f, values);
        if (absent.includes(f.key)) el.classList.add("missing");
        (f.advanced ? more : box).append(el);
        continue;
      }
      const el = fieldInput(f, values[f.key], edits(values, f.key));
      if (absent.includes(f.key)) {
        el.classList.add("missing");
        const m = document.createElement("span");
        m.className = "err";
        m.textContent = "please fill in";
        el.append(m);
      }
      if (f.key === "ical_urls") el.append(calendarHelp(values));
      (f.advanced ? more : box).append(el);
    }
  } else {
    const p = document.createElement("p");
    p.className = "muted";
    p.textContent = "No editor for this module: edit its settings as JSON.";
    box.append(p);
    box.append(rawEditor(entry));
  }

  const [iw, ih] = innerSize(entry);
  const pg = page();
  const auto = autoFontOf(state.catalog, grid(), entry.config, pg.pageW, pg.areaH);
  const num = (key, label) => fieldInput({ key, label, kind: "number" }, values[key], edits(values, key));
  more.append(num("padding_x", "Inner margin left/right (px)"), num("padding_y", "Inner margin top/bottom (px)"));
  const font = num("fontsize", "Base text size (px)");
  font.append(button(`Auto (${auto} px)`, () => {
    values.fontsize = auto;
    commit();
  }));
  more.append(font);
  more.append(
    fieldInput({ key: "language", label: "Language", kind: "select", options: LANGS, labels: { en: "English", de: "Deutsch" }, required: true }, values.language || "en", (v, record) => {
      setValue(values, "language", v);
      if (record) commit();
    }),
  );
  const info = document.createElement("p");
  info.className = "muted small";
  const [c, r, cs, rs] = values.cell;
  info.textContent = `Cell: column ${c + 1}, row ${r + 1}, ${cs} × ${rs} · inner ${iw} × ${ih} px`;
  more.append(info);
  box.append(more);
  box.append(styleEditor(values, "style", "Style of this tile", state.settings.theme));

  const actions = document.createElement("div");
  actions.className = "actions";
  actions.append(button("Remove module", removeSelected, "danger"), button("Page settings", () => select(-1)));
  box.append(actions);
}

function renderModuleError() {
  const el = $("#module-error");
  if (!el) return;
  const e = state.errors[state.selected];
  el.className = e ? "module-error" : "";
  el.replaceChildren();
  if (!e) return;
  const what = document.createElement("strong");
  what.textContent = e.hint || "This tile could not be drawn.";
  const raw = document.createElement("small");
  raw.textContent = e.error;
  el.append(what, document.createElement("br"), raw);
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
  const on = key => edits(s, key);
  // these change the page area, so tiles may need to move
  const onReflow = key => (v, record) => {
    setValue(s, key, v);
    if (record) reflow();
  };
  const disp = document.createElement("div");
  disp.className = "field";
  const dl = document.createElement("span");
  dl.textContent = "Display";
  disp.append(dl, button(panelName() + " — change…", openDisplayPicker));
  box.append(disp);
  box.append(fieldInput({ key: "orientation", label: "Orientation", kind: "select", options: ORIENTATIONS.map(o => o.value),
    labels: Object.fromEntries(ORIENTATIONS.map(o => [String(o.value), o.label])), required: true }, s.orientation ?? 0, on("orientation")));
  box.append(fieldInput({ key: "update_times", label: "Update times", kind: "text_list", help: "HH:MM, one per line; empty uses the interval" },
    s.update_times, (v, record) => {
      setValue(s, "update_times", v === undefined ? undefined : v.split(", "));
      if (record) commit();
    }));
  box.append(fieldInput({ key: "update_interval", label: "Update interval (minutes)", kind: "number" }, s.update_interval, on("update_interval")));

  const look = fieldset("Look");
  look.append(fieldInput({ key: "dividers", label: "Hairlines between modules", kind: "bool" }, s.dividers, on("dividers")));
  look.append(fieldInput({ key: "border_around_modules", label: "Border around modules", kind: "bool" }, s.border_around_modules, on("border_around_modules")));
  look.append(fieldInput({ key: "info_section", label: "Info line (time, battery)", kind: "bool" }, s.info_section, onReflow("info_section")));
  if (s.info_section) {
    look.append(fieldInput({ key: "info_section_height", label: "Info line height (px)", kind: "number" }, s.info_section_height, onReflow("info_section_height")));
  }
  box.append(look);
  box.append(styleEditor(s, "theme", "Theme (all tiles)", null));

  const adv = fieldset("Panel");
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

// ---------- styles ----------

const panelInks = () => (state.catalog.panels && state.catalog.panels[state.settings.model] || {}).inks || "bwr";

// inkCss is how an ink looks on e-paper, for swatches.
function inkCss(ink) {
  if (ink === "white") return "#e8e5dc";
  if (ink === "black" || (ink === "colour" && panelInks() === "bw")) return "#1d1d1b";
  if (panelInks() === "gray16") return "#5c5b57";
  return state.settings.colour_ink === "yellow" ? "#d6ac1e" : "#a3262a";
}

const INK_LABEL = { white: "White", black: "Black", colour: "Colour" };

// styleEditor edits owner[key] (a tile's "style" or the page "theme"); parent is the
// look the tile inherits (the page theme), null for the theme itself.
function styleEditor(owner, key, title, parent) {
  const set = fn => {
    const next = cleanStyle(fn({ ...(owner[key] || {}), colors: { ...((owner[key] || {}).colors || {}) } }));
    if (next) owner[key] = next;
    else delete owner[key];
    commit();
  };
  const own = owner[key] || {};
  const eff = effective(parent, own);
  const fs = fieldset(title);

  // colour presets as little tiles in the panel's inks
  const presets = document.createElement("div");
  presets.className = "presets";
  const current = matchPreset(eff.colors);
  for (const p of presetsFor(panelInks())) {
    const b = button("Aa", () => set(st => ({ ...st, colors: { ...p.colors } })), "preset");
    b.title = p.label;
    b.setAttribute("aria-pressed", String(current === p.id));
    b.style.background = inkCss(p.colors.paper);
    b.style.color = inkCss(p.colors.ink);
    const dot = document.createElement("span");
    dot.style.background = inkCss(p.colors.accent);
    b.append(dot);
    presets.append(b);
  }
  fs.append(presets);

  const inks = inkOptions(panelInks());
  const isTile = key !== "theme";
  const inherit = isTile ? "(theme)" : "(default)";
  const row = document.createElement("div");
  row.className = "row";
  for (const part of ["paper", "ink", "accent"]) {
    const label = { paper: "Paper", ink: "Text", accent: "Accent" }[part];
    row.append(fieldInput({ key: part, label, kind: "select", options: inks }, (own.colors || {})[part] || "", (v, record) => {
      if (record) set(st => ({ ...st, colors: { ...st.colors, [part]: v } }));
    }));
  }
  for (const sel of row.querySelectorAll("select")) {
    sel.options[0].textContent = inherit;
    for (const o of sel.options) if (INK_LABEL[o.value]) o.textContent = INK_LABEL[o.value];
  }
  fs.append(row);

  // font: pairs and uploaded files
  const fontWrap = document.createElement("label");
  fontWrap.className = "field";
  const ft = document.createElement("span");
  ft.textContent = "Font";
  const sel = document.createElement("select");
  sel.append(new Option(isTile ? "(theme)" : "Sans (default)", ""));
  for (const [v, l] of [["sans", "Sans"], ["serif", "Serif"], ["mono", "Mono"]]) sel.append(new Option(l, v));
  for (const f of state.fonts) {
    const gaps = ["german", "greek"].filter(k => !f.coverage[k]);
    sel.append(new Option(f.name + (gaps.length ? ` (no ${gaps.join(", ")} — falls back)` : ""), f.path));
  }
  sel.value = fontChoice(own);
  sel.addEventListener("change", () => set(st => applyFont(st, sel.value)));
  fontWrap.append(ft, sel);
  fs.append(fontWrap);

  // size factor
  const sizeWrap = document.createElement("label");
  sizeWrap.className = "field";
  const sz = document.createElement("span");
  const pct = Math.round((own.scale || eff.scale || 1) * 100);
  sz.textContent = `Text size ${pct} %` + (own.scale ? "" : ` ${inherit}`);
  const range = document.createElement("input");
  range.type = "range";
  range.min = "50";
  range.max = "200";
  range.step = "5";
  range.value = String(pct);
  range.addEventListener("input", () => (sz.textContent = `Text size ${range.value} %`));
  range.addEventListener("change", () => set(st => ({ ...st, scale: Number(range.value) / 100 === 1 && !isTile ? 0 : Number(range.value) / 100 })));
  sizeWrap.append(sz, range);
  fs.append(sizeWrap);

  if (Object.keys(own).length) fs.append(button(isTile ? "Use the theme" : "Reset theme", () => set(() => ({}))));

  if (key === "theme") {
    if (panelInks() === "bwr") {
      fs.append(fieldInput({ key: "colour_ink", label: "Your panel's colour", kind: "select", options: ["red", "yellow"], required: true },
        state.settings.colour_ink || "red", (v, record) => {
          if (!record) return;
          state.settings.colour_ink = v;
          commit();
        }));
    }
    const up = document.createElement("label");
    up.className = "field";
    const ut = document.createElement("span");
    ut.textContent = "Upload a font (.ttf, .otf)";
    const file = document.createElement("input");
    file.type = "file";
    file.accept = ".ttf,.otf";
    file.addEventListener("change", () => file.files[0] && uploadFont(file.files[0]));
    up.append(ut, file);
    fs.append(up);
  }
  return fs;
}

async function loadFonts() {
  try {
    state.fonts = await (await api("GET", "/api/fonts")).json();
  } catch {
    state.fonts = [];
  }
}

async function uploadFont(file) {
  const name = file.name.replace(/[^A-Za-z0-9._-]/g, "-").replace(/^[.]+/, "");
  try {
    const res = await fetch("/api/fonts/" + encodeURIComponent(name), { method: "PUT", headers: { "X-Inkycal-Token": token }, body: file });
    if (!res.ok) throw new Error((await res.text()).trim());
    await loadFonts();
    setStatus(`font ${name} uploaded`);
    renderInspector();
  } catch (err) {
    setStatus("font not uploaded: " + err.message, "error");
  }
}

// updateTileView shows the selected tile alone, cut from the latest preview.
function updateTileView() {
  if (!state.settings) return;
  const view = $("#tile-view");
  const entry = modules()[state.selected];
  if (!view || !entry || !entry.config.cell) return;
  const src = state.view === "paper" ? $("#paper") : $("#pixels");
  const p = page();
  const r = cellRect(grid(), entry.config.cell, p.pageW, p.areaH);
  const width = Math.min(r.w, $("#inspector").clientWidth - 30);
  const scale = width / r.w;
  view.width = Math.max(1, Math.round(r.w * scale));
  view.height = Math.max(1, Math.round(r.h * scale));
  const ctx = view.getContext("2d");
  ctx.imageSmoothingEnabled = state.view === "paper";
  try {
    ctx.drawImage(src, r.x, r.y, r.w, r.h, 0, 0, view.width, view.height);
  } catch {
    // the preview has not loaded yet
  }
}

// ---------- calendar help ----------

// calendarHelp explains where the secret iCal address is and tests each address.
function calendarHelp(values) {
  const box = document.createElement("div");
  box.className = "cal-help";
  const how = document.createElement("details");
  const sum = document.createElement("summary");
  sum.textContent = "How do I get this address?";
  how.append(sum);
  for (const g of state.catalog.guides || []) {
    const p = document.createElement("p");
    const n = document.createElement("strong");
    n.textContent = g.name + ": ";
    p.append(n, g.steps);
    how.append(p);
  }
  const out = document.createElement("div");
  out.className = "cal-results";
  const test = button("Test the addresses", async e => {
    e.preventDefault();
    const urls = display({ kind: "url_list" }, values.ical_urls).split("\n").filter(Boolean);
    out.replaceChildren();
    if (!urls.length) out.textContent = "Add an address first.";
    for (const [i, url] of urls.entries()) {
      const line = document.createElement("p");
      line.textContent = `Calendar ${i + 1}: testing…`;
      out.append(line);
      try {
        const r = await (await api("POST", "/api/test-calendar", { url })).json();
        if (r.ok) {
          const next = (r.next || []).map(n => `${n.date} ${n.title}`).join(" · ");
          line.textContent = `Calendar ${i + 1}: ✓ ${r.events} events in the next two months` + (next ? ` — ${next}` : "") + (r.hint ? ` (${r.hint})` : "");
          line.className = "ok";
        } else {
          line.textContent = `Calendar ${i + 1}: ✗ ${r.hint || r.error}`;
          line.className = "bad";
        }
      } catch (err) {
        line.textContent = `Calendar ${i + 1}: ✗ ${err.message}`;
        line.className = "bad";
      }
    }
  });
  box.append(how, test, out);
  return box;
}

// ---------- assistant ----------

const STEPS = ["display", "template", "connect", "schedule"];
const wiz = { step: 0, schedule: "daily", battery: 1200 };

function openWizard(isNew = false) {
  wiz.step = 0;
  wiz.schedule = pickSchedule(state.settings, isNew);
  wiz.battery = state.settings.use_pi_sugar === false ? 0 : 1200;
  renderWizard();
  $("#wizard").showModal();
}

function renderWizard() {
  const body = $("#wizard-body");
  body.replaceChildren();
  const step = STEPS[wiz.step];
  $("#wizard-step").textContent = `Step ${wiz.step + 1} of ${STEPS.length}`;
  $("#wizard-back").disabled = wiz.step === 0;
  $("#wizard-next").textContent = wiz.step === STEPS.length - 1 ? "Done" : "Next";
  const h = document.createElement("h3");
  body.append(h);
  const p = page();
  if (step === "display") {
    h.textContent = "Which display do you have?";
    const hint = document.createElement("p");
    hint.className = "muted small";
    hint.textContent = "Size and colours are on the box; V1/V2 on a sticker on the back. Current: " + panelName();
    const search = document.createElement("input");
    search.type = "search";
    search.placeholder = "Search, e.g. 7.5 red";
    search.className = "wide";
    const list = document.createElement("div");
    list.className = "wizard-list";
    const fill = () => {
      list.replaceChildren();
      for (const g of groupPanels(state.catalog.panel_list || [], search.value)) {
        for (const pn of g.panels) {
          list.append(panelButton(pn, () => {
            state.settings.model = pn.model;
            reflow();
            renderWizard();
          }));
        }
      }
    };
    search.addEventListener("input", fill);
    fill();
    body.append(hint, search, list);
  } else if (step === "template") {
    h.textContent = "What should it show?";
    const list = document.createElement("div");
    list.className = "wizard-list";
    if (modules().length) {
      list.append(button("Keep my current tiles", () => {
        wiz.step++;
        renderWizard();
      }, "panel-option current"));
    }
    for (const tpl of state.catalog.templates || []) {
      const fits = templateFits(tpl, state.catalog, p.pageW, p.areaH);
      const b = optionButton(tpl.title, tpl.description + (fits ? "" : " — too big for this display"), () => {
        if (modules().length && !confirm("Replace the current tiles with this template?")) return;
        Object.assign(state.settings, applyTemplate(state.settings, tpl, state.catalog, defaultLanguage(), p.pageW, p.areaH));
        state.selected = -1;
        banner("");
        commit();
        wiz.step++;
        renderWizard();
      });
      b.disabled = !fits;
      list.append(b);
    }
    body.append(list);
  } else if (step === "connect") {
    h.textContent = "Connect your services";
    // required fields first; an empty calendar address is worth a hint too
    const needs = (meta, m) => {
      const keys = missing(meta, m.config);
      const fields = (meta.fields || []).filter(f => keys.includes(f.key) && !f.advanced);
      if (!keys.length && (meta.fields || []).some(f => f.key === "ical_urls") && !m.config.ical_urls) {
        return { optional: true, labels: ["your calendar address (optional)"] };
      }
      return { optional: false, labels: fields.map(f => f.label) };
    };
    const todo = modules().map((m, i) => ({ m, i, meta: metaOf(m.name) })).filter(x => x.meta && needs(x.meta, x.m).labels.length);
    const list = document.createElement("div");
    list.className = "wizard-list";
    if (!todo.length) {
      const ok = document.createElement("p");
      ok.textContent = "✓ Every tile has what it needs.";
      list.append(ok);
    }
    for (const { m, i, meta } of todo) {
      const { optional, labels } = needs(meta, m);
      list.append(optionButton((optional ? "＋ " : "⚠ ") + tileTitle(meta, m), (optional ? "Add " : "Needs: ") + labels.join(", ") + " — set up", () => {
        $("#wizard").close();
        select(i);
        banner(`Fill in the marked fields of "${tileTitle(meta, m)}" — then open the assistant again or just save.`);
      }));
    }
    body.append(list);
  } else {
    h.textContent = "When should it update?";
    const list = document.createElement("div");
    list.className = "wizard-list";
    const panel = state.catalog.panels && state.catalog.panels[state.settings.model];
    const est = document.createElement("p");
    est.className = "estimate";
    const showEstimate = () => {
      const sc = SCHEDULES.find(x => x.id === wiz.schedule);
      if (!wiz.battery) {
        est.textContent = "On USB power the panel can update as often as you like.";
        return;
      }
      const days = estimateDays({ wakesPerDay: sc.keep ? wakesFor(state.settings) : sc.wakesPerDay, panel, capacity: wiz.battery });
      est.textContent = days >= 14 ? `Battery lasts about ${Math.round(days / 7)} weeks (rough estimate).` : `Battery lasts about ${days} days (rough estimate).`;
    };
    for (const sc of SCHEDULES) {
      const lab = document.createElement("label");
      lab.className = "field check";
      const r = document.createElement("input");
      r.type = "radio";
      r.name = "schedule";
      r.checked = sc.id === wiz.schedule;
      r.addEventListener("change", () => {
        wiz.schedule = sc.id;
        showEstimate();
      });
      const t = document.createElement("span");
      t.textContent = sc.label;
      lab.append(r, t);
      list.append(lab);
    }
    const bat = document.createElement("select");
    for (const b of BATTERIES) bat.append(new Option(b.label, String(b.id)));
    bat.value = String(wiz.battery);
    bat.addEventListener("change", () => {
      wiz.battery = Number(bat.value);
      showEstimate();
    });
    showEstimate();
    body.append(list, bat, est);
  }
}

function wizardNext() {
  if (STEPS[wiz.step] === "schedule") {
    Object.assign(state.settings, applySchedule(state.settings, SCHEDULES.find(x => x.id === wiz.schedule)));
    commit();
    $("#wizard").close();
    setStatus("ready — press Save to keep it", "dirty");
    return;
  }
  wiz.step++;
  renderWizard();
}

// ---------- place search ----------

// placeInput searches a town and writes its name and coordinates into the tile.
function placeInput(field, values) {
  const wrap = document.createElement("div");
  wrap.className = "field place";
  const title = document.createElement("span");
  title.textContent = field.label + (field.required ? " *" : "");
  const row = document.createElement("div");
  row.className = "secret";
  const input = document.createElement("input");
  input.type = "search";
  input.value = values[field.key] || "";
  input.placeholder = "e.g. Hamburg";
  const list = document.createElement("div");
  list.className = "place-results";
  const search = async () => {
    const q = input.value.trim();
    if (!q) return;
    list.textContent = "searching…";
    try {
      const res = await api("GET", "/api/geocode?q=" + encodeURIComponent(q) + "&lang=" + encodeURIComponent(inspectorLang));
      const places = await res.json();
      list.replaceChildren();
      if (!places.length) list.textContent = "Nothing found — try another spelling.";
      for (const p of places) {
        list.append(button(p.label, () => {
          values[field.key] = p.location;
          values.latitude = Math.round(p.latitude * 1000) / 1000;
          values.longitude = Math.round(p.longitude * 1000) / 1000;
          commit();
        }));
      }
    } catch (err) {
      list.textContent = "Search failed: " + err.message;
    }
  };
  input.addEventListener("keydown", e => {
    if (e.key === "Enter") {
      e.preventDefault();
      search();
    }
  });
  row.append(input, button("Search", search));
  wrap.append(title, row, list);
  if (values.latitude !== undefined && values.longitude !== undefined) {
    wrap.append(small(`📍 ${values[field.key] || ""} (${values.latitude}, ${values.longitude})`));
  } else if (field.help) {
    wrap.append(small(field.help));
  }
  return wrap;
}

// ---------- display picker ----------

function panelName() {
  const p = state.catalog.panels && state.catalog.panels[state.settings.model];
  return p ? p.name : "Choose your display";
}

function openDisplayPicker() {
  const dlg = $("#display-dialog");
  $("#display-search").value = "";
  renderDisplayList("");
  dlg.showModal();
  $("#display-search").focus();
}

function renderDisplayList(query) {
  const box = $("#display-list");
  box.replaceChildren();
  for (const g of groupPanels(state.catalog.panel_list || [], query)) {
    const h = document.createElement("h4");
    h.textContent = g.group;
    box.append(h);
    for (const p of g.panels) {
      box.append(panelButton(p, () => {
        $("#display-dialog").close();
        if (p.model === state.settings.model) return;
        state.settings.model = p.model;
        banner("");
        reflow();
      }));
    }
  }
  if (!box.children.length) box.textContent = "No display matches.";
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
    if (fits(g, cells(), cell, state.selected)) applyCell(state.selected, cell);
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
  let isNew = false;
  if (!token) {
    document.body.textContent = "Open the address the builder printed in the terminal (it contains #token=…).";
    return;
  }
  try {
    state.catalog = await (await api("GET", "/api/catalog")).json();
    await loadFonts();
    const res = await api("GET", "/api/settings");
    isNew = res.headers.get("X-Inkycal-New") === "1";
    state.settings = await res.json();
    state.settings.modules = state.settings.modules || [];
    state.settings.modules.sort((a, b) => (a.position || 0) - (b.position || 0));
    for (const m of state.settings.modules) m.config = normalize(m.name, m.config || {});
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
  if (isNew) openWizard(true);
}

function setView(view) {
  state.view = view;
  updateTileView();
  document.body.classList.toggle("paper", view === "paper");
  document.body.classList.toggle("pixels", view === "pixels");
  $("#view-paper").setAttribute("aria-pressed", String(view === "paper"));
  $("#view-pixels").setAttribute("aria-pressed", String(view === "pixels"));
}

function wire() {
  // a new address with another token: the page must start over with it
  window.addEventListener("hashchange", () => location.reload());
  setView("paper");
  $("#view-paper").addEventListener("click", () => setView("paper"));
  $("#display-chip").addEventListener("click", openDisplayPicker);
  $("#wizard-open").addEventListener("click", () => openWizard(false));
  $("#wizard-next").addEventListener("click", wizardNext);
  $("#wizard-back").addEventListener("click", () => {
    wiz.step = Math.max(0, wiz.step - 1);
    renderWizard();
  });
  $("#wizard-skip").addEventListener("click", () => $("#wizard").close());
  $("#display-close").addEventListener("click", () => $("#display-dialog").close());
  $("#display-search").addEventListener("input", e => renderDisplayList(e.target.value));
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
    const entry = state.paletteEntries[Number(e.dataTransfer.getData("text/x-inkycal-module"))];
    if (!entry) return;
    e.preventDefault();
    addModule(entry.meta, cellAt(e.clientX, e.clientY), entry.config);
  });
}

wire();
load();
