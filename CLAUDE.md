# inkycal (Go)

Go rewrite of Inkycal for Raspberry Pi e-paper dashboards. One static binary; the main target is a Pi Zero 2 W + PiSugar 3 that wakes, draws, and powers off again — **wake time is the metric that matters**.

## Commands

| Command | What |
|---|---|
| `make test` | `go vet ./...` + `go test ./...` (run before every commit) |
| `make check` | `make test` + staticcheck + govulncheck + UI tests (before a release) |
| `go test -race ./...` | also run for changes to builder, fetch, render caches |
| `node --test internal/builder/ui/*.test.mjs` | builder UI logic (`*.test.mjs`, no npm deps) |
| `make bench` | in-process benchmarks of the wake path (`BenchmarkRunCycle`, `BenchmarkPack`); compare runs with benchstat |
| `make wakebench [BASE=v0.1.2]` | cold-start dry run of the working tree vs. the latest release (offline fixtures); ±5 % is noise |
| `go run ./cmd/inkycal --mode dry-run --settings s.json --out preview.png` | render without a panel |
| `go run ./cmd/inkycal --mode builder --settings s.json --listen 127.0.0.1:8080` | web builder; token is in `<state-dir>/builder-token` |
| `make release` | arm64/armv7/armv6 binaries + `SHA256SUMS` in `dist/` (names match `deploy/install.sh`); used by CI, see Releases |
| `make deploy PI=pi@host ARCH=arm64` | copy binary + systemd unit to a Pi |

Modes: `run | once | dry-run | clear | builder`.

## Layout

```
cmd/inkycal/          flags, mode dispatch
internal/app/         RunCycle, Compose (shared by device and builder), Registry, hints
internal/builder/     HTTP API (catalog, settings, preview, fonts, geocode, test-calendar), explain(), templates
internal/builder/ui/  plain ES modules; pure logic in grid/form/paper/style/panels/palette/wizard.js
internal/config/      settings.json (upstream-compatible), grid cells, Style/theme
internal/epd/         panel drivers, Panels() catalog, Inks(model); trace tests
internal/render/      Canvas, fonts, Theme (font pairs, custom fonts with fallback), inks, AssembleGrid
internal/modules/     one package per module + meta.go (Meta/Field) + modulestest
internal/problem/     error kinds (Classify, Mark): one source for panel hints and builder advice
internal/{fetch,ical,schedule,state,pisugar,sysinfo,timefmt,i18n,layout,picture}
deploy/               install.sh, systemd unit, image/ and tuning/ notes
tools/epdtrace/       regenerates driver byte traces from an upstream checkout
tools/wakebench/      cold-start comparison against a release
.github/workflows/    ci.yml (checks on push/PR), dev-build.yml (pre-release dev-build), release.yml (vX.Y.Z)
```

## Rules

- **Wake time must not get worse.** Check with `make bench` / `make wakebench` before a release. Nothing new on the device path unless a setting uses it; no network-online dependency in the unit; panel init and network wait run in parallel.
- **Upstream settings keep working.** `internal/app/testdata/upstream-generator.json` must load and every module must accept its config (`compat_test.go`). New keys are optional with old behaviour as default (e.g. Weather with `api_key` stays on OpenWeatherMap, TextToDisplay with `filepath` unchanged).
- **Display drivers** must match upstream byte for byte: `internal/epd/testdata/trace/*.txt` via `trace_test.go`; regenerate only with `tools/epdtrace/regen.sh`.
- **Red/yellow ink only for meaning** (today, alerts), never decoration. `colour_ink` is preview-only.
- **No secrets** (API keys, tokens, Wi-Fi passwords) in logs, errors or API responses — use `fetch.Redact`; secret fields use `modules.KindSecret`.
- **Builder scope:** upstream's modules done nicer, plus grid, themes/tile styles, assistant + templates, Open-Meteo, typed text, calendar help/test, plain-language hints. No custom "widget" module — it was removed on purpose.

## Adding or changing a module

- Write it as `var Def = modules.Define(Meta, build)` and add `Def` to `Defs` in `internal/app/registry.go`; give it a `modules.Meta` (Group, Icon, MinW/MinH, Fields with Help). `Define` decodes the config, then `build` checks settings (`modules.Missing`, `modules.Bad`), then the minimum size: never check size or decode by hand, never put the module's name in an error (the caller adds the tile's).
- Test with `modulestest.CheckMeta(t, Def)` (it also checks every Required field is enforced) and draw at every `modulestest.Shapes` size.
- Errors users can hit get a kind: mark them where they arise with `problem.Mark(problem.<Kind>, err)` (network, HTTP status, missing files and JSON types are recognised by type). A new kind goes into `internal/problem`, with advice in `builder/explain.go`, a hint key in `app/hint.go` + `i18n` (en and de) — except `Placement`: an unplaced tile has nowhere to show one — and a test that takes the error from its real producer (`problem_test.go`, or the producer's own package when it is internal, e.g. `app`). Never match error text.
- Field kinds the UI understands: text, secret, url, url_list, text_list, number, bool, select, multiline, place.

## Gotchas

- `deploy/install.sh` embeds the systemd unit; `install_test.go` fails if it drifts from `deploy/inkycal-go.service`.
- Installer channels: no flag = `releases/latest`, `--dev` = the `dev-build` pre-release (tag `dev-build`, so `git push origin dev` stays unambiguous).
- Tiles are decoded once, in `(*config.Settings).Tiles()` (cell, look, problem per tile); `app/page.go` places them on the page. Read a tile's settings there, not with another `json.Unmarshal`. Words: `GLOSSARY.md`.
- Grid cells: a page has cells on all tiles or none; a tile without one on a grid page is unplaced.
- Device and builder take settings differently: `config.Load` and `Compose` are lenient (a broken tile fails on its own, a bad theme falls back to the default); `config.Parse` (builder) rejects a broken theme, cell or style, so the builder never saves one.
- Builder writes settings atomically and keeps file mode; never loosen permissions of settings or `.bak`.
- Font cache keys custom fonts by path+size+mtime; only regular files ≤ 20 MiB.
- `go.mod` pins a patched `toolchain`; raise it when govulncheck reports standard-library issues — release binaries ship that stdlib.
- `.claude/settings.json` runs `goimports -w` after every edit of a `.go` file; don't hand-format.
- Killing a running instance: `kill $(pgrep -x inkycal)` — `pkill -f` matches your own shell.

## Releases

- Binaries come only from CI, never from a local build: every push to `dev` replaces the pre-release `dev-build` (`dev-build.yml`); an annotated tag `vX.Y.Z` on `main` publishes the stable release with the tag message as notes (`release.yml`).
- Order: CI green → owner tests `install.sh --dev` on a Pi (wake time, builder) → PR `dev` → `main`, owner merges → tag. Workflow files are owner-managed (the agent token lacks `workflow` scope).

## Workflow

- Work on `dev`; `main` is the released line. Small, tested commits; a failing test first for bug fixes.
- Prefer gopls (LSP) for definitions and references over grep when renaming or removing code.

## Agent skills

### Issue tracker

Local markdown under `.scratch/<feature>/` (GitHub Issues are off for this fork). See `docs/agents/issue-tracker.md`.

### Triage labels

The five default roles (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`) as a `Status:` line in each issue file. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` and `docs/adr/` at the repo root, created when a term or decision gets settled. See `docs/agents/domain.md`.
