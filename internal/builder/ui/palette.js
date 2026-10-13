// The module palette: groups in a fixed order, empty groups left out.

export const GROUPS = [
  ["weather", "Weather & time"],
  ["calendar", "Calendar & tasks"],
  ["info", "News, notes & fun"],
  ["pictures", "Pictures"],
];

export function paletteGroups(catalog) {
  const out = GROUPS.map(([id, title]) => ({ id, title, entries: [] }));
  const byId = Object.fromEntries(out.map(g => [g.id, g]));
  for (const meta of catalog.modules) {
    (byId[meta.group] || byId.info).entries.push({ title: meta.title, description: meta.description, icon: meta.icon, meta, config: {} });
  }
  return out.filter(g => g.entries.length);
}

// tileTitle names a tile on the panel and in the inspector.
export function tileTitle(meta, entry) {
  return meta ? meta.title : entry.name;
}
