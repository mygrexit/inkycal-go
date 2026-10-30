# One frame for building a module

Status: ready-for-agent

Architecture candidate 3. Chosen from three drafts (minimal `Build`, declarative Meta, typed `Define`): the typed
`Define` with one module list, the fixed check order of the minimal draft, no runtime checks from Meta.

## Decisions

1. `modules.Define[C](meta, build) Def` makes a module's `Def{Meta, New}`. Its factory decodes the tile's config
   into a C (an empty config is `{}`), runs `build`, then checks the minimum size. The order is the same for every
   module: config that does not decode (BadSetting, "config: …"), a setting `build` refuses (Missing, BadSetting),
   a tile too small (TooSmall). No module calls `CheckMinSize` itself.
2. Errors from a module never name the module; `runModules` puts the tile's name in front of factory errors as it
   already does for Generate errors. Module-name prefixes inside the modules go ("Fullweather: fullweather: …").
3. Helpers `modules.Missing(key)` and `modules.Bad(format, …)` mark the two setting problems; settings that are
   wrong today without a kind (agenda columns, xkcd mode, weather provider, slideshow interval, fullweather
   coordinates, image look) get BadSetting.
4. `app` keeps one list `[]modules.Def`; `Registry` and `Catalog` are derived from it.
5. `modulestest.CheckMeta(t, def, …)` reads the config type from the Def and also checks that every Required field
   (with its ShowIf met) is enforced: without it the factory returns Missing. Fields the device infers on purpose
   opt out with a reason (weather `location`, textfile `source`).
6. Meta does not grow; the upstream inferences (weather provider from api_key, textfile source) stay in `build`.
7. Not in scope: a frame for Generate (canvas, Stale), splitting fetch from draw, shapes_test glue.
8. Upstream settings keep working (compat_test); pixels of all previews stay identical.
