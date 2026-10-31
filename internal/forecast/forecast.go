// Package forecast reads the weather a view draws, current conditions and 3-hour
// forecast slots, from OpenWeatherMap or Open-Meteo into one model: metric units and
// OpenWeatherMap icon codes. The Weather and Fullweather modules share it.
package forecast

import (
	"context"
	"strings"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
)

// The weather services, as settings.json names them in "provider".
const (
	OpenMeteo      = "open-meteo"
	OpenWeatherMap = "openweathermap"
)

// Settings are a weather tile's keys for its data: which service and where. Views
// embed them in their config.
type Settings struct {
	Provider  string            `json:"provider"` // OpenMeteo or OpenWeatherMap; "" = OpenWeatherMap with an api_key (upstream settings), else Open-Meteo
	APIKey    string            `json:"api_key"`
	Location  config.FlexString `json:"location"` // OpenWeatherMap: a place name or city id
	Latitude  *config.FlexFloat `json:"latitude"`
	Longitude *config.FlexFloat `json:"longitude"`
}

// Want is what a view needs besides the place.
type Want struct {
	Days     int  // forecast days from Open-Meteo, 0 = 3; OpenWeatherMap always sends 5
	UV       bool // the current UV index: Open-Meteo has it, OpenWeatherMap only with OneCall
	OneCall  bool // the OpenWeatherMap key may use One Call 3.0 (upstream api_version 3.0)
	ByCoords bool // OpenWeatherMap by coordinates even when a place name is set (upstream Fullweather)
}

// Current is the weather now (metric units, OpenWeatherMap icon codes).
type Current struct {
	Icon        string
	Description string // localised condition text, e.g. "leichter Regen"
	Temp        float64
	FeelsLike   float64
	Humidity    int
	WindMPS     float64 // gust if reported, else mean speed (as in Python)
	WindMean    float64 // mean speed, m/s
	Sunrise     time.Time
	Sunset      time.Time // both zero when the service has none (polar day or night)
}

// Point is one 3-hour forecast slot (metric units).
type Point struct {
	Time   time.Time
	Temp   float64
	Icon   string
	Wind   float64 // mean speed, m/s
	Gust   float64 // gust if reported, else mean speed, m/s
	Precip float64 // rain + snow of the 3-hour slot, mm
}

// Report is the weather at one place.
type Report struct {
	Now    Current
	Points []Point  // ascending 3-hour slots; each Precip covers the three hours up to Time
	UV     *float64 // only when wanted, available and fresh
	Stale  bool     // some of it came from the cache
}

// Source is a checked place at a weather service, ready to fetch.
type Source struct {
	p    provider
	want Want
}

type provider interface {
	fetch(ctx context.Context, fc *fetch.Client, loc *time.Location, w Want) (Report, error)
}

// Source checks s and picks the service. OpenWeatherMap takes the place by name or
// city id, else (and with Want.ByCoords) by coordinates; Open-Meteo needs the
// coordinates. lang is for the condition text. Errors are setting problems
// (modules.Missing, modules.Bad), in the order a user fixes them; nothing is fetched.
func (s Settings) Source(lang string, w Want) (Source, error) {
	key, place := strings.TrimSpace(s.APIKey), strings.TrimSpace(string(s.Location))
	if w.ByCoords {
		place = ""
	}
	switch s.service() {
	case OpenWeatherMap:
		if key == "" {
			return Source{}, modules.Missing("api_key")
		}
		o := owm{apiKey: s.APIKey, place: place, lang: lang, oneCall: w.OneCall}
		if place == "" {
			var err error
			if o.lat, o.lon, err = s.coords(w.ByCoords); err != nil {
				return Source{}, err
			}
			o.hasCoords = true
		} else if lat, lon, err := s.coords(false); err == nil { // by name; coordinates only for One Call
			o.lat, o.lon, o.hasCoords = lat, lon, true
		}
		return Source{p: o, want: w}, nil
	case OpenMeteo:
		lat, lon, err := s.coords(true)
		if err != nil {
			return Source{}, err
		}
		return Source{p: openMeteo{lat: lat, lon: lon, lang: lang}, want: w}, nil
	}
	return Source{}, modules.Bad("provider %q: want %s or %s", s.Provider, OpenMeteo, OpenWeatherMap)
}

// coords checks latitude and longitude; without them the place is missing (by name,
// unless byCoords).
func (s Settings) coords(byCoords bool) (lat, lon float64, err error) {
	if s.Latitude == nil || s.Longitude == nil {
		if byCoords {
			return 0, 0, modules.Missing("latitude", "longitude") // the builder's place search writes them
		}
		return 0, 0, modules.Missing("location")
	}
	lat, lon = float64(*s.Latitude), float64(*s.Longitude)
	if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, modules.Bad("latitude/longitude %v/%v out of range", lat, lon)
	}
	return lat, lon, nil
}

// service is the service s names, or the one upstream settings mean.
func (s Settings) service() string {
	if s.Provider != "" {
		return s.Provider
	}
	if strings.TrimSpace(s.APIKey) != "" {
		return OpenWeatherMap
	}
	return OpenMeteo
}

// Fetch loads the weather. Errors never name the module.
func (s Source) Fetch(ctx context.Context, fc *fetch.Client, loc *time.Location) (Report, error) {
	return s.p.fetch(ctx, fc, loc, s.want)
}
