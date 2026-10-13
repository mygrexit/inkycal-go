// Style logic for the page theme and each tile's own look (config.Style in Go).

export const PRESETS = [
  { id: "standard", label: "Standard", colors: { paper: "white", ink: "black", accent: "colour" } },
  { id: "inverted", label: "Inverted", colors: { paper: "black", ink: "white", accent: "colour" } },
  { id: "colour-tile", label: "Colour tile", colors: { paper: "colour", ink: "white", accent: "black" } },
  { id: "colour-text", label: "Colour text", colors: { paper: "white", ink: "colour", accent: "black" } },
  { id: "no-colour", label: "No colour", colors: { paper: "white", ink: "black", accent: "black" } },
];

export const DEFAULT_COLORS = { paper: "white", ink: "black", accent: "colour" };
export const ROLES = ["text", "strong", "bold", "display"];

export function inkOptions(panelInks) {
  return panelInks === "bw" ? ["white", "black"] : ["white", "black", "colour"];
}

// presetsFor lists the presets a panel can show. On b/w panels the accent becomes
// black (or white on black paper) and presets that need colour paper or ink go.
export function presetsFor(panelInks) {
  if (panelInks !== "bw") return PRESETS;
  const out = [];
  for (const p of PRESETS) {
    if (p.colors.paper === "colour" || p.colors.ink === "colour") continue;
    const colors = { ...p.colors, accent: p.colors.paper === "black" ? "white" : "black" };
    if (!out.some(o => JSON.stringify(o.colors) === JSON.stringify(colors))) out.push({ ...p, colors });
  }
  return out;
}

// effective is what a tile looks like: its own style where set, else the page theme.
export function effective(page, tile) {
  const out = { font: "", scale: 0, custom_fonts: {}, colors: { ...DEFAULT_COLORS } };
  for (const s of [page, tile]) {
    if (!s) continue;
    if (s.font) out.font = s.font;
    if (s.scale) out.scale = s.scale;
    Object.assign(out.custom_fonts, s.custom_fonts || {});
    for (const k of Object.keys(DEFAULT_COLORS)) if (s.colors && s.colors[k]) out.colors[k] = s.colors[k];
  }
  if (!out.font) delete out.font;
  if (!out.scale) delete out.scale;
  return out;
}

export function matchPreset(colors) {
  const hit = PRESETS.find(p => Object.keys(DEFAULT_COLORS).every(k => (colors[k] || DEFAULT_COLORS[k]) === p.colors[k]));
  return hit ? hit.id : "custom";
}

// cleanStyle removes empty parts; undefined when nothing is left.
export function cleanStyle(st) {
  if (!st) return undefined;
  const out = {};
  if (st.font) out.font = st.font;
  if (st.scale) out.scale = st.scale;
  const colors = Object.fromEntries(Object.entries(st.colors || {}).filter(([, v]) => v));
  if (Object.keys(colors).length) out.colors = colors;
  const fonts = Object.fromEntries(Object.entries(st.custom_fonts || {}).filter(([, v]) => v));
  if (Object.keys(fonts).length) out.custom_fonts = fonts;
  return Object.keys(out).length ? out : undefined;
}

// fontChoice is the value of the font select: "" (inherit), a pair name, or a custom file.
export function fontChoice(st) {
  if (!st || !st.font) return "";
  if (st.font === "custom") return (st.custom_fonts && st.custom_fonts.text) || "";
  return st.font;
}

// applyFont sets a pair ("sans", "serif", "mono"), a custom file ("fonts/…") or "" (inherit).
export function applyFont(st, choice) {
  const out = { ...(st || {}) };
  delete out.custom_fonts;
  delete out.font;
  if (choice.startsWith("fonts/")) {
    out.font = "custom";
    out.custom_fonts = Object.fromEntries(ROLES.map(r => [r, choice]));
  } else if (choice) {
    out.font = choice;
  }
  return out;
}
