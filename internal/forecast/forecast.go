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

// Settings are a weather tile's keys for its data: which service and where. Views
// embed them in their config.
type Settings struct {
	Provider  string            `json:"provider"` // "open-meteo" or "openweathermap"; "" = OpenWeatherMap with an api_key (upstream settings), else Open-Meteo
	APIKey    string            `json:"api_key"`
	Location  config.FlexString `json:"location"` // OpenWeatherMap: a place name or city id
	Latitude  *config.FlexFloat `json:"latitude"`
	Longitude *config.FlexFloat `json:"longitude"`
}

// Want is what a view needs besides the place.
type Want struct {
	Days    int  // forecast days from Open-Meteo, 0 = 3; OpenWeatherMap always sends 5
	UV      bool // the current UV index: Open-Meteo has it, OpenWeatherMap only with OneCall
	OneCall bool // the OpenWeatherMap key may use One Call 3.0 (upstream api_version 3.0)
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
// city id, else by coordinates; Open-Meteo needs the coordinates. lang is for the
// condition text. Errors are setting problems (modules.Missing, modules.Bad); nothing
// is fetched.
func (s Settings) Source(lang string, w Want) (Source, error) {
	hasCoords := s.Latitude != nil && s.Longitude != nil
	var lat, lon float64
	if hasCoords {
		lat, lon = float64(*s.Latitude), float64(*s.Longitude)
		if lat < -90 || lat > 90 || lon < -180 || lon > 180 {
			return Source{}, modules.Bad("latitude/longitude %v/%v out of range", lat, lon)
		}
	}
	key := strings.TrimSpace(s.APIKey)
	switch s.provider() {
	case "openweathermap":
		place := strings.TrimSpace(string(s.Location))
		switch {
		case key == "":
			return Source{}, modules.Missing("api_key")
		case place == "" && !hasCoords:
			return Source{}, modules.Missing("location")
		}
		return Source{p: owm{apiKey: s.APIKey, place: place, lang: lang, lat: lat, lon: lon, oneCall: w.OneCall}, want: w}, nil
	case "open-meteo":
		if !hasCoords {
			return Source{}, modules.Missing("latitude", "longitude") // the builder's place search writes them
		}
		return Source{p: openMeteo{lat: lat, lon: lon, lang: lang}, want: w}, nil
	}
	return Source{}, modules.Bad("provider %q: want open-meteo or openweathermap", s.Provider)
}

// provider is the service s names, or the one upstream settings mean.
func (s Settings) provider() string {
	if s.Provider != "" {
		return s.Provider
	}
	if strings.TrimSpace(s.APIKey) != "" {
		return "openweathermap"
	}
	return "open-meteo"
}

// Fetch loads the weather. Errors never name the module.
func (s Source) Fetch(ctx context.Context, fc *fetch.Client, loc *time.Location) (Report, error) {
	return s.p.fetch(ctx, fc, loc, s.want)
}
