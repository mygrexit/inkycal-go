package builder

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type dot struct{ c config.Common }

func (d dot) Generate(context.Context, *modules.Env) (*render.Canvas, error) {
	w, h := d.c.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, 14)
	c.FillCircle(w/2, h/2, 5, render.InkColour)
	return c, nil
}

type fail struct{}

func (fail) Generate(context.Context, *modules.Env) (*render.Canvas, error) {
	return nil, errors.New("GET https://api.example.com/x?appid=SECRET: HTTP 401")
}

var testMeta = []modules.Meta{
	{Name: "Dot", Title: "Dot", MinW: 10, MinH: 10},
	{Name: "Fail", Title: "Fail", MinW: 10, MinH: 10},
}

func newTestServer(t *testing.T, settings string) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if settings != "" {
		if err := os.WriteFile(path, []byte(settings), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s, err := New(Config{
		SettingsPath: path,
		StateDir:     dir,
		Registry: map[string]modules.Factory{
			"Dot":  func(c config.Common, _ json.RawMessage) (modules.Module, error) { return dot{c}, nil },
			"Fail": func(config.Common, json.RawMessage) (modules.Module, error) { return fail{}, nil },
		},
		Catalog: testMeta,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, path
}

func do(t *testing.T, s *Server, method, path, body string, token bool) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token {
		req.Header.Set("X-Inkycal-Token", s.Token())
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

const gridSettings = `{"model": "epd_7_in_5_v2_colour", "info_section": false, "keep": "me",
  "grid": {"columns": 2, "rows": 2},
  "modules": [
    {"position": 1, "name": "Dot", "config": {"cell": [0, 0, 1, 2], "padding_x": 10, "padding_y": 10}},
    {"position": 2, "name": "Fail", "config": {"cell": [1, 0, 1, 2], "padding_x": 10, "padding_y": 10}}
  ]}`

func TestAPINeedsToken(t *testing.T) {
	s, _ := newTestServer(t, gridSettings)
	for _, p := range []string{"/api/catalog", "/api/settings"} {
		if rec := do(t, s, "GET", p, "", false); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s without token: %d", p, rec.Code)
		}
	}
	req := httptest.NewRequest("GET", "/api/catalog", nil)
	req.Header.Set("X-Inkycal-Token", "wrong")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong token: %d", rec.Code)
	}
	// The page itself loads without a token (it reads the token from the URL fragment).
	if rec := do(t, s, "GET", "/", "", false); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<html") {
		t.Errorf("index: %d", rec.Code)
	}
}

func TestCatalog(t *testing.T) {
	s, _ := newTestServer(t, gridSettings)
	rec := do(t, s, "GET", "/api/catalog", "", true)
	var got struct {
		Models  map[string][2]int `json:"models"`
		Modules []modules.Meta    `json:"modules"`
		Grid    struct{ Columns, Rows int }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Models["epd_12_in_48_colour_V2"] != [2]int{1304, 984} || len(got.Modules) != 2 || got.Grid.Columns != 12 {
		t.Errorf("catalog %+v", got)
	}
}

func TestSettingsRoundTripKeepsUnknownKeysAndBackup(t *testing.T) {
	s, path := newTestServer(t, gridSettings)
	rec := do(t, s, "GET", "/api/settings", "", true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"keep": "me"`) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	changed := strings.Replace(gridSettings, `"info_section": false`, `"info_section": true`, 1)
	if rec := do(t, s, "PUT", "/api/settings", changed, true); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"info_section": true`) || !strings.Contains(string(data), `"keep": "me"`) {
		t.Errorf("saved file:\n%s", data)
	}
	if bak, _ := os.ReadFile(path + ".bak"); !strings.Contains(string(bak), `"info_section": false`) {
		t.Errorf("backup:\n%s", bak)
	}
}

func TestSettingsRejectsInvalid(t *testing.T) {
	s, path := newTestServer(t, gridSettings)
	overlap := strings.Replace(gridSettings, `[1, 0, 1, 2]`, `[0, 1, 1, 1]`, 1)
	for name, body := range map[string]string{"json": `{"model": `, "overlap": overlap} {
		rec := do(t, s, "PUT", "/api/settings", body, true)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	if rec := do(t, s, "PUT", "/api/settings", "{"+strings.Repeat(" ", 2<<20)+"}", true); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("huge body: %d", rec.Code)
	}
	data, _ := os.ReadFile(path)
	if string(data) != gridSettings {
		t.Error("invalid settings were written")
	}
}

func TestSettingsMissingFileStartsNew(t *testing.T) {
	s, _ := newTestServer(t, "")
	rec := do(t, s, "GET", "/api/settings", "", true)
	if rec.Code != http.StatusOK || rec.Header().Get("X-Inkycal-New") != "1" {
		t.Fatalf("%d %v", rec.Code, rec.Header())
	}
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil || v["model"] == "" {
		t.Errorf("template %s", rec.Body)
	}
}

func TestPreviewRendersPageAndReportsErrors(t *testing.T) {
	s, _ := newTestServer(t, gridSettings)
	rec := do(t, s, "POST", "/api/preview", gridSettings, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got struct {
		Image         string `json:"image"`
		Width, Height int
		Errors        []struct {
			Position int    `json:"position"`
			Error    string `json:"error"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	b64, ok := strings.CutPrefix(got.Image, "data:image/png;base64,")
	if !ok {
		t.Fatalf("image %.40q", got.Image)
	}
	raw, _ := base64.StdEncoding.DecodeString(b64)
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	// 7.5" panel is 800×480: the page is portrait, 480 wide.
	if b := img.Bounds(); b.Dx() != 480 || b.Dy() != 800 || got.Width != 480 || got.Height != 800 {
		t.Errorf("image %v, reported %dx%d", b, got.Width, got.Height)
	}
	if r, g, _, _ := img.At(120, 400).RGBA(); r>>8 != 255 || g>>8 != 0 {
		t.Error("no red dot in the left module")
	}
	if len(got.Errors) != 1 || got.Errors[0].Position != 2 {
		t.Fatalf("errors %+v", got.Errors)
	}
	if strings.Contains(got.Errors[0].Error, "SECRET") {
		t.Errorf("secret leaked: %s", got.Errors[0].Error)
	}
	if rec := do(t, s, "POST", "/api/preview", `{"model": "nope", "modules": []}`, true); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown model: %d", rec.Code)
	}
	if _, err := io.ReadAll(rec.Body); err != nil {
		t.Fatal(err)
	}
}

// settings.json holds API keys: a save keeps the file's mode, and a new file and the
// backup are readable by the owner only.
func TestSaveKeepsSettingsPrivate(t *testing.T) {
	s, path := newTestServer(t, gridSettings)
	os.Chmod(path, 0o600)
	if rec := do(t, s, "PUT", "/api/settings", gridSettings, true); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{path, path + ".bak"} {
		if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("%s: mode %v err %v", p, st.Mode().Perm(), err)
		}
	}
	fresh, newPath := newTestServer(t, "")
	if rec := do(t, fresh, "PUT", "/api/settings", gridSettings, true); rec.Code != http.StatusOK {
		t.Fatalf("put new: %d", rec.Code)
	}
	if st, _ := os.Stat(newPath); st.Mode().Perm() != 0o600 {
		t.Errorf("new file mode %v", st.Mode().Perm())
	}
}
