# One weather source

Status: ready-for-agent

Architecture candidate 4, chosen from three drafts (minimal source, provider adapters, own package); all three
converged on a package shared by both weather views.

## Decisions

1. New package `internal/forecast` (like `ical` and `picture`): `Settings` (provider, api_key, location, latitude,
   longitude) embedded in a view's config; `Settings.Source(lang, Want{Days, UV, OneCall})` checks the keys and picks
   the service without I/O; `Source.Fetch(ctx, fc, loc)` returns a `Report{Now, Points, UV, Stale}`.
2. Service choice as Weather has it: an empty provider means OpenWeatherMap with an api_key (upstream settings),
   else Open-Meteo. OpenWeatherMap goes by place name/id, else coordinates; Open-Meteo needs coordinates.
3. The two services are unexported adapters behind `Source`; OpenWeatherMap fetches current + forecast in parallel,
   plus the One Call UV index when wanted and allowed (api_version 3.0); Open-Meteo asks for `Want.Days` days and
   `uv_index` only when wanted.
4. Shared vocabulary moves too: glyphs, `DayIcon`, `SameDay`, `Beaufort`, `WindLevel`, `Fahrenheit`.
   `weather/export.go` and Fullweather's own fetching are deleted.
5. Tests reach the real API paths through `fetch/fetchtest` instead of package URL variables.
6. Step one is a pure refactor: previews and shape renders of both views pixel-identical.
7. Step two: Fullweather gets Open-Meteo — `provider` and a `location` place field in its Meta, `api_key` and
   `api_version` only for OpenWeatherMap, the builder infers the provider for Fullweather like for Weather.
   Upstream Fullweather (api_key + coordinates) stays on OpenWeatherMap by coordinates.
8. No sunrise (polar day/night) is a zero time in the model; neither view prints a 1970 clock.
