package builder

import (
	"net/http"
	"net/url"
	"strings"
)

// geocodeURL is Open-Meteo's place search (no account); a variable for tests.
var geocodeURL = "https://geocoding-api.open-meteo.com/v1/search"

type place struct {
	Name     string  `json:"name"`
	Label    string  `json:"label"`    // "Berlin, Berlin, Deutschland"
	Location string  `json:"location"` // what Weather stores: "Berlin,DE" (also understood by OpenWeatherMap)
	Lat      float64 `json:"latitude"`
	Lon      float64 `json:"longitude"`
}

// geocode finds places by name for the place fields (weather, sun & moon, pollen).
func (s *Server) geocode(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" || len(q) > 100 {
		http.Error(w, "type a place name", http.StatusBadRequest)
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}
	v := url.Values{"name": {q}, "count": {"8"}, "language": {lang}, "format": {"json"}}
	var res struct {
		Results []struct {
			Name        string  `json:"name"`
			Lat         float64 `json:"latitude"`
			Lon         float64 `json:"longitude"`
			Country     string  `json:"country"`
			CountryCode string  `json:"country_code"`
			Admin1      string  `json:"admin1"`
		} `json:"results"`
	}
	if _, err := s.cached.GetJSON(r.Context(), geocodeURL+"?"+v.Encode(), nil, &res); err != nil {
		http.Error(w, "place search failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	out := []place{}
	for _, p := range res.Results {
		parts := []string{p.Name}
		for _, x := range []string{p.Admin1, p.Country} {
			if x != "" {
				parts = append(parts, x)
			}
		}
		out = append(out, place{Name: p.Name, Label: strings.Join(parts, ", "), Location: p.Name + "," + p.CountryCode, Lat: p.Lat, Lon: p.Lon})
	}
	writeJSON(w, out)
}
