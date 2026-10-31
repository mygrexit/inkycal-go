# Resolve a tile once

Status: ready-for-agent

Architecture candidate 2. Design chosen from four "design it twice" drafts: a config half (data) plus an app half (pixels), no new package.

## Problem

Each tile's config is decoded up to five times (tolerate ×2, GridCells, ParseCommon, the module), "unplaced" (`config.ErrCell`) is handled in four places, `tolerate` re-implements `layout.Validate`, and the stacked/grid split runs through `Compose` and `runModules`.

## Decisions

1. `config` gets `(*Settings).Tiles() (grid *layout.Grid, tiles []Tile, err error)`. It is lenient, decodes each tile's config once and stores nothing. Each `Tile` has `Common` (defaults, font size scaled by the look), `Cell` (nil when stacked or unplaced), `Look` (effective style; the theme alone when the tile's own style is broken) and `Err`. `err` is only for a page-wide problem (a broken grid).
2. `Load` = decode + theme fallback (Warnings). `Parse` = decode + theme check + `Tiles`; it rejects the first tile with a placement or style problem as `module N (name): …`. Decode and padding problems stay tile errors, as today.
3. Deleted: `ErrCell`, `tolerate`, `GridCells`, `validateStyles`, `ModuleEntry.Err`; `Effective` becomes unexported; `scaled` moves into config.
4. `app/page.go` resolves the tiles against the page: section, sized `Common`, theme, inks, and the canvas a module or its hint draws on. `Compose` and `runModules` keep no `rects`/`ErrCell` branches. The unknown-module check stays in `runModules` (its hint uses the inner area).
5. `Compose` always resolves leniently, including for settings built in memory.
6. Panel pixels and device error texts stay identical. Only the builder's save rejections for cells change their wording (`module 2 (B): grid layout: no cell`).
7. `render.Assemble` / `AssembleGrid` stay as they are (merging them changes dividers).
8. Tests go through `Tiles`, `Parse`, `Load` and `Compose`; the `GridCells` tests are replaced, not kept.

## Accepted after review

- A broken grid (e.g. 0 columns) together with a tile without a cell now fails the page with the grid's own problem (`grid layout: grid 0x2: …`) instead of `module 2 (B) has no cell`: the grid is the cause.
- When a tile's config does not decode, cell and style are read once more on their own, so a field of the wrong type does not cost the tile its cell (as before). The happy path decodes once.
