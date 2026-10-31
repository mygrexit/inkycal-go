// Making a tile: what the palette and the templates both write for a new module. The
// numbers (padding, grid, font rule) come from Go in the catalog; nothing here repeats them.
import { cellRect, autoFont } from "./grid.js";

// newTile is a tile without a place yet: the module's field defaults, then preset,
// then the catalog's padding and the language (those two the preset cannot change).
export function newTile(catalog, name, preset, language) {
  const meta = catalog.modules.find(m => m.name === name) || { fields: [] };
  const defaults = Object.fromEntries((meta.fields || []).filter(f => f.default !== undefined).map(f => [f.key, f.default]));
  const [px, py] = catalog.tile_padding;
  return { position: 0, name, config: { ...defaults, ...preset, padding_x: px, padding_y: py, language } };
}

// placeTile puts config's tile on cell: its pixel size (upstream reads it) and the
// automatic font size for its inner area.
export function placeTile(catalog, grid, config, cell, pageW, areaH) {
  config.cell = [...cell];
  const r = cellRect(grid, cell, pageW, areaH);
  config.size = [r.w, r.h];
  config.fontsize = autoFontOf(catalog, grid, config, pageW, areaH);
}

// autoFontOf is the automatic font size for config's tile where it is now.
export function autoFontOf(catalog, grid, config, pageW, areaH) {
  const r = cellRect(grid, config.cell, pageW, areaH);
  return autoFont(catalog.font_rule, r.w - 2 * (Number(config.padding_x) || 0), r.h - 2 * (Number(config.padding_y) || 0));
}
