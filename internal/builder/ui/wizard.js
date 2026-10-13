// The first-run assistant: battery estimate, update schedules and templates.
import { cellRect, autoFont } from "./grid.js";

// refreshSeconds: how long a full panel refresh takes (measured: 12.48″ colour 51 s).
export function refreshSeconds(panel) {
  if (!panel) return 20;
  if (panel.inks === "gray16") return 3;
  if (panel.inks === "bw") return 6;
  return panel.inches >= 12 ? 52 : 20;
}

// Rough battery model of a Pi Zero 2 W with a PiSugar: awake ~1.2 W for boot (~15 s),
// fetching (~2 s), the refresh and shutdown (~8 s); off ~0.5 mW for the PiSugar's RTC;
// 85 % of the cell (3.7 V) usable.
const AWAKE_W = 1.2;
const OFF_W = 0.0005;
const OVERHEAD_S = 25;

export function estimateDays({ wakesPerDay, panel, capacity }) {
  const batteryWh = (capacity / 1000) * 3.7 * 0.85;
  const wakeWh = (AWAKE_W * (OVERHEAD_S + refreshSeconds(panel))) / 3600;
  const perDay = wakesPerDay * wakeWh + OFF_W * 24;
  return Math.round(batteryWh / perDay);
}

export const BATTERIES = [
  { id: 0, label: "No battery (USB power)" },
  { id: 1200, label: "PiSugar 3 (1200 mAh)" },
  { id: 5000, label: "PiSugar 3 Plus (5000 mAh)" },
];

const hours = (from, to, step) => Array.from({ length: Math.floor((to - from) / step) + 1 }, (_, i) => `${String(from + i * step).padStart(2, "0")}:00`);

export const SCHEDULES = [
  { id: "keep", label: "Keep my current schedule", keep: true, wakesPerDay: 0 },
  { id: "daily", label: "Once a day at 6:00 (longest battery life)", times: ["06:00"], wakesPerDay: 1 },
  { id: "twice", label: "Twice a day, 6:00 and 18:00", times: ["06:00", "18:00"], wakesPerDay: 2 },
  { id: "day4", label: "Every 4 hours during the day", times: hours(6, 22, 4), wakesPerDay: 5 },
  { id: "hourly", label: "Every hour from 6:00 to 22:00", times: hours(6, 22, 1), wakesPerDay: 17 },
  { id: "quarter", label: "Every 15 minutes (needs USB power)", interval: 15, wakesPerDay: 96 },
];

// wakesFor counts the updates per day of a settings file (for the battery estimate).
export function wakesFor(settings) {
  if (Array.isArray(settings.update_times) && settings.update_times.length) return settings.update_times.length;
  return Math.max(1, Math.round(1440 / (Number(settings.update_interval) || 60)));
}

// pickSchedule preselects the preset matching the settings, "keep" when none does
// (a new file starts with once a day).
export function pickSchedule(settings, isNew) {
  if (isNew) return "daily";
  const hit = SCHEDULES.find(sc => sc.times && JSON.stringify(sc.times) === JSON.stringify(settings.update_times));
  return hit ? hit.id : "keep";
}

export function applySchedule(settings, schedule) {
  if (schedule.keep) return settings;
  const out = { ...settings };
  if (schedule.times) {
    out.update_times = [...schedule.times];
  } else {
    delete out.update_times;
    out.update_interval = schedule.interval;
  }
  return out;
}

const pad = catalog => catalog.template_padding || [16, 12];

// templateFits: every module of the template gets at least its minimum inner size.
export function templateFits(tpl, catalog, pageW, areaH) {
  const g = { columns: 12, rows: 16 };
  const [px, py] = pad(catalog);
  return tpl.modules.every(m => {
    const meta = catalog.modules.find(x => x.name === m.name);
    if (!meta) return false;
    const r = cellRect(g, m.cell, pageW, areaH);
    return r.w - 2 * px >= (meta.min_w || 1) && r.h - 2 * py >= (meta.min_h || 1);
  });
}

// applyTemplate replaces the modules with the template's (other settings stay).
export function applyTemplate(settings, tpl, catalog, lang, pageW, areaH) {
  const g = { columns: 12, rows: 16 };
  const [px, py] = pad(catalog);
  const modules = tpl.modules.map((m, i) => {
    const meta = catalog.modules.find(x => x.name === m.name) || { fields: [] };
    const defaults = Object.fromEntries((meta.fields || []).filter(f => f.default !== undefined).map(f => [f.key, f.default]));
    const r = cellRect(g, m.cell, pageW, areaH);
    const config = { ...defaults, ...(m.config || {}), cell: [...m.cell], size: [r.w, r.h], padding_x: px, padding_y: py, language: lang };
    if (catalog.font_rule) config.fontsize = autoFont(catalog.font_rule, r.w - 2 * px, r.h - 2 * py);
    return { position: i + 1, name: m.name, config };
  });
  return { ...settings, grid: { ...g }, modules };
}
