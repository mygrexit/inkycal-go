// Package builder serves the visual dashboard editor: the page, the module catalog,
// settings load/save and live previews rendered by the same code as the panel.
package builder

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"inkycal/internal/app"
	"inkycal/internal/config"
	"inkycal/internal/epd"
	"inkycal/internal/fetch"
	"inkycal/internal/layout"
	"inkycal/internal/modules"
	"inkycal/internal/render"
	"inkycal/internal/timefmt"
)

//go:embed ui/*.html ui/*.css ui/*.js ui/*.svg
var uiFiles embed.FS

const (
	maxBody        = 1 << 20
	previewTimeout = 20 * time.Second
	freshFor       = 10 * time.Minute // live data in previews is at most this old
)

// newSettings is what an empty builder starts from.
const newSettings = `{
    "model": "epd_7_in_5_v2_colour",
    "update_interval": 60,
    "orientation": 0,
    "info_section": false,
    "image_hash": true,
    "grid": {"columns": 12, "rows": 16},
    "modules": []
}
`

type Config struct {
	SettingsPath string
	StateDir     string // HTTP cache for previews
	Registry     map[string]modules.Factory
	Catalog      []modules.Meta
	Token        string // "" = random
}

type Server struct {
	cfg      Config
	token    string
	saveMu   sync.Mutex
	renderMu sync.Mutex
	cached   *fetch.Client // FreshFor: previews while editing
	live     *fetch.Client // "refresh data"
}

func New(cfg Config) (*Server, error) {
	s := &Server{cfg: cfg, token: cfg.Token}
	if s.token == "" {
		tok, err := loadToken(filepath.Join(cfg.StateDir, "builder-token"))
		if err != nil {
			return nil, err
		}
		s.token = tok
	}
	cache := filepath.Join(cfg.StateDir, "http")
	s.cached, s.live = fetch.New(cache), fetch.New(cache)
	s.cached.FreshFor = freshFor
	return s, nil
}

// catalogModules sends every module's fields as a list (JSON [] rather than null).
func catalogModules(ms []modules.Meta) []modules.Meta {
	out := make([]modules.Meta, len(ms))
	for i, m := range ms {
		if m.Fields == nil {
			m.Fields = []modules.Field{}
		}
		out[i] = m
	}
	return out
}

// formatExamples renders every date/time format option on a fixed day, so the builder can
// offer "Tue, 13 Oct" instead of "ddd, D MMM".
func (s *Server) formatExamples() map[string]map[string]string {
	at := time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)
	out := map[string]map[string]string{"en": {}, "de": {}}
	for _, m := range s.cfg.Catalog {
		for _, f := range m.Fields {
			if f.Key != "date_format" && f.Key != "time_format" {
				continue
			}
			for _, o := range f.Options {
				if layout, ok := o.(string); ok {
					for lang := range out {
						out[lang][layout] = timefmt.Format(at, layout, lang)
					}
				}
			}
		}
	}
	return out
}

// loadToken reuses the token of an earlier run, so the address printed at start stays
// the same; the first run creates it (owner-only, it guards settings with API keys).
func loadToken(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		if tok := string(bytes.TrimSpace(data)); len(tok) == 32 {
			return tok, nil
		}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return tok, os.WriteFile(path, []byte(tok+"\n"), 0o600)
}

// Token must accompany every API request (header X-Inkycal-Token).
func (s *Server) Token() string { return s.token }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	ui, _ := fs.Sub(uiFiles, "ui")
	static := http.FileServerFS(ui)
	mux.Handle("GET /", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self'")
		static.ServeHTTP(w, r)
	}))
	mux.HandleFunc("GET /api/catalog", s.catalog)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("POST /api/preview", s.preview)
	mux.HandleFunc("GET /api/fonts", s.listFonts)
	mux.HandleFunc("GET /api/geocode", s.geocode)
	mux.HandleFunc("POST /api/test-calendar", s.testCalendar)
	mux.HandleFunc("PUT /api/fonts/{name}", s.putFont)
	return s.auth(mux)
}

// auth guards /api/: a header (not a cookie), so other sites cannot send it for you.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Inkycal-Token")), []byte(s.token)) != 1 {
				http.Error(w, "missing or wrong token: open the address the builder printed", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func (s *Server) catalog(w http.ResponseWriter, _ *http.Request) {
	list := epd.Panels()
	panels := make(map[string]epd.Panel, len(list))
	for _, p := range list {
		panels[p.Model] = p
	}
	writeJSON(w, map[string]any{
		"examples":         s.formatExamples(),
		"guides":           guides,
		"templates":        templates,
		"template_padding": [2]int{templatePadX, templatePadY},
		"models":           epd.Models,
		"panels":           panels,
		"panel_list":       list,
		"modules":          catalogModules(s.cfg.Catalog),
		"grid":             layout.Default,
		"font_rule":        map[string]int{"h_div": 9, "w_div": 40, "min": 12, "max": 32}, // modules.AutoFontSize
	})
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	data, err := os.ReadFile(s.cfg.SettingsPath)
	if errors.Is(err, os.ErrNotExist) {
		w.Header().Set("X-Inkycal-New", "1")
		data, err = []byte(newSettings), nil
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// readSettings reads and validates a request body; the JSON is kept verbatim so keys
// this program does not know survive a save.
func (s *Server) readSettings(w http.ResponseWriter, r *http.Request) ([]byte, *config.Settings, bool) {
	body, ok := readBody(w, r, maxBody, "settings larger than 1 MiB")
	if !ok {
		return nil, nil, false
	}
	st, err := config.Parse(body, "settings")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, nil, false
	}
	st.SetDir(filepath.Dir(s.cfg.SettingsPath)) // custom fonts are relative to settings.json
	if _, ok := epd.Models[st.Model]; !ok {
		http.Error(w, fmt.Sprintf("display model %q not supported", st.Model), http.StatusBadRequest)
		return nil, nil, false
	}
	return body, st, true
}

// readBody reads a request body of at most limit bytes; tooBig is the error beyond that.
func readBody(w http.ResponseWriter, r *http.Request, limit int64, tooBig string) ([]byte, bool) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			http.Error(w, tooBig, http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return nil, false
	}
	return body, true
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	body, _, ok := s.readSettings(w, r)
	if !ok {
		return
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "    "); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	pretty.WriteByte('\n')
	s.saveMu.Lock()
	defer s.saveMu.Unlock()
	if err := writeAtomic(s.cfg.SettingsPath, pretty.Bytes()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"saved": s.cfg.SettingsPath})
}

// writeAtomic keeps the previous file as .bak and replaces it via rename. settings.json
// holds API keys: the file keeps its mode, a new one (and every backup) is owner-only.
func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	if old, err := os.ReadFile(path); err == nil {
		os.Remove(path + ".bak") // WriteFile keeps the mode of an existing file
		if err := os.WriteFile(path+".bak", old, 0o600); err != nil {
			return err
		}
	}
	return replaceFile(path, ".settings-*.json", data, mode)
}

// replaceFile writes data to a temporary file (named by pattern) next to path and
// renames it over path, so nobody ever reads a half-written file.
func replaceFile(path, pattern string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), pattern)
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// sizeDisplay only tells Compose the panel size.
type sizeDisplay struct{ w, h int }

func (d sizeDisplay) Size() (int, int)      { return d.w, d.h }
func (sizeDisplay) Show(render.Frame) error { return nil }
func (sizeDisplay) Clear(int) error         { return nil }

// The colour layer as the panels show it (sampled from photos, not screen red).
var (
	colourRed    = color.RGBA{196, 22, 28, 255}
	colourYellow = color.RGBA{230, 190, 0, 255}
)

// queryString hides API keys and tokens in URLs quoted by module errors.
var queryString = regexp.MustCompile(`\?[^\s"']+`)

// userError is an error as the builder shows it: query strings hidden, plus advice.
func userError(err error) (msg, hint string) {
	msg = queryString.ReplaceAllString(err.Error(), "?…")
	return msg, explain(msg)
}

type moduleError struct {
	Position int    `json:"position"` // 1-based index in settings order
	Name     string `json:"name"`
	Error    string `json:"error"`
	Hint     string `json:"hint,omitempty"` // what to do, in plain words
}

func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	_, st, ok := s.readSettings(w, r)
	if !ok {
		return
	}
	size := epd.Models[st.Model]
	client := s.cached
	if r.URL.Query().Get("refresh") == "1" {
		client = s.live
	}
	s.renderMu.Lock()
	defer s.renderMu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), previewTimeout)
	defer cancel()
	start := time.Now()
	o := &app.Options{
		Settings: st, Display: sizeDisplay{size[0], size[1]}, Fetch: client,
		Loc: time.Local, Now: time.Now, Registry: s.cfg.Registry,
	}
	composed, err := app.Compose(ctx, o, time.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	colour := colourRed
	if st.ColourInk == "yellow" {
		colour = colourYellow
	}
	var img bytes.Buffer
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&img, composed.Canvas.PreviewInks(epd.Inks(st.Model), colour)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	errs := []moduleError{}
	for i, m := range composed.Modules {
		if m.Err != nil {
			msg, hint := userError(m.Err)
			errs = append(errs, moduleError{i + 1, st.Modules[i].Name, msg, hint})
		}
	}
	writeJSON(w, map[string]any{
		"image":   "data:image/png;base64," + base64.StdEncoding.EncodeToString(img.Bytes()),
		"width":   composed.Canvas.W,
		"height":  composed.Canvas.H,
		"errors":  errs,
		"took_ms": time.Since(start).Milliseconds(),
	})
}
