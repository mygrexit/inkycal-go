# Glossary

Domain words used in code, issues and reviews. German UI words in brackets.

**Page** [Seite]: the portrait canvas a cycle draws: panel height × width. The info line, when on, takes its bottom strip; the tiles share the rest.

**Tile** [Kachel]: one entry of `modules` in settings.json as it appears on the page: a module name, its config, a section, a look and possibly a problem. `config.Tile` is the tile decoded from settings; `app.tile` is the same tile resolved against the page in pixels. _Avoid_: widget, module (when the placed entry is meant; "module" is the code that draws it).

**Section**: the rectangle of the page a tile owns. Stacked: its `size`, one below the other. Grid: its cell's share of the page. The module draws in the section minus padding (the inner area).

**Stacked layout**: upstream's layout: every tile has a `size` and tiles follow each other down the page.

**Grid layout**: `grid` (columns × rows, default 12×16) plus a `cell` [col, row, colSpan, rowSpan] on every tile. A settings file has cells on all tiles or none.

**Placed / unplaced**: on a grid, a tile whose cell is missing, outside the grid or overlaps an earlier tile is unplaced: it has no section and is only reported.

**Theme / style / look**: the theme is the page's look (`theme`), a style a tile's own (`style` in its config). The tile's look is its style over the theme; a broken style falls back to the theme.

**Lenient / strict**: how settings are taken. The device is lenient (`config.Load`): a broken tile fails on its own and the rest of the page is drawn. The builder is strict (`config.Parse`): it refuses to save settings with a broken tile.

**Hint**: the plain-language text a failed tile shows in its section on the panel (`app/hint.go`, i18n). The builder's longer English text is its **advice** (`builder/explain.go`). Both come from the error's **kind** (`internal/problem`).

**Weather service** [Wetterdienst]: where a weather tile's data comes from, `provider` in its config: Open-Meteo (free, by coordinates) or OpenWeatherMap (needs `api_key`; by place name or coordinates). Upstream settings with an `api_key` and no `provider` mean OpenWeatherMap. `internal/forecast` reads both into one **report**: the weather now and 3-hour **slots** ahead.
