// The display picker: panels grouped by size, searchable by words or model code.

export const ORIENTATIONS = [
  { value: 0, label: "Normal" },
  { value: 180, label: "Upside down (cable on the other side)" },
];

// groupPanels filters by every word of the query and keeps the catalog's size order.
export function groupPanels(list, query) {
  const words = query.toLowerCase().split(/\s+/).filter(Boolean);
  const out = [];
  for (const p of list) {
    const text = (p.name + " " + p.model).toLowerCase();
    if (!words.every(w => text.includes(w))) continue;
    let g = out.find(x => x.group === p.group);
    if (!g) out.push((g = { group: p.group, panels: [] }));
    g.panels.push(p);
  }
  return out;
}
