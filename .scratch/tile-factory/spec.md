# One way to make a tile in the builder

Status: ready-for-agent

Architecture candidate 5, chosen from three drafts (one pure newTile, Go owns the numbers, shared fixture); they agreed.

## Decisions

1. Go owns the numbers and sends them in the catalog: `font_rule` is `modules.AutoFont` (a `FontRule` value that
   `AutoFontSize` uses), `tile_padding` (renamed from `template_padding`), `grid` (`layout.Default`). No literal copy.
2. New pure `ui/tile.js`: `newTile(catalog, name, preset, language)` (field defaults, preset, then padding and
   language), `placeTile(catalog, grid, config, cell, pageW, areaH)` (cell, size, auto font size), `autoFontOf`.
   Palette (`addModule`) and templates (`applyTemplate`, `templateFits`) both use them; placement on a free spot
   stays in `addModule`.
3. Padding 16/12 for every new tile (the tested template value); the palette's untested 20/16 goes. Existing settings
   are untouched.
4. `wizard.js` takes the grid from the catalog instead of a 12×16 literal.
5. Guard: `ui/testdata/autofont.json` (rule + cases) is read by a Go test and a node test; the catalog test checks
   `tile_padding` and `font_rule`.
6. While here: `style.js effective` drops the page's custom fonts when a tile picks a font pair, as
   `config.effective` does (with a test).
7. Not in scope: cell placement helpers (`cellRect`, `minSpans` stay JS copies of layout), `page()` numbers.
