package xkcd

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"inkycal/internal/render"
)

// TestPreview renders sample comics to PNG. It needs INKYCAL_PREVIEW_DIR (output) and
// INKYCAL_XKCD_SAMPLES: a directory with <num>.json files as served by xkcd.com and
// the images they link in comics/ (plus any _2x variants).
func TestPreview(t *testing.T) {
	out, samples := os.Getenv("INKYCAL_PREVIEW_DIR"), os.Getenv("INKYCAL_XKCD_SAMPLES")
	if out == "" || samples == "" {
		t.Skip("INKYCAL_PREVIEW_DIR and INKYCAL_XKCD_SAMPLES not set")
	}
	latest := 3310
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dir, file := path.Split(r.URL.Path); dir == "/comics/" {
			http.ServeFile(w, r, filepath.Join(samples, "comics", filepath.Base(file)))
			return
		}
		n := latest
		if r.URL.Path != "/info.0.json" {
			if _, err := fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &n); err != nil {
				http.NotFound(w, r)
				return
			}
		}
		data, err := os.ReadFile(filepath.Join(samples, strconv.Itoa(n)+".json"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		var c map[string]any
		json.Unmarshal(data, &c)
		c["img"] = srv.URL + "/comics/" + path.Base(c["img"].(string))
		json.NewEncoder(w).Encode(c)
	}))
	defer srv.Close()
	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	now := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	for _, p := range []struct {
		name, size string
		num        int
		alt        string
	}{
		{"tall-latest-alt", `[904,600]`, 3310, "yes"},
		{"tall-wide-alt", `[904,600]`, 927, "yes"},
		{"tall-square-alt-bwr", `[904,600]`, 1205, "yes"},
		{"band-latest-alt", `[904,140]`, 3310, "yes"},
		{"band-wide", `[904,140]`, 927, "no"},
		{"band-wide-alt", `[904,140]`, 927, "yes"},
		{"half-python-alt", `[440,600]`, 353, "yes"},
		{"tall-first-jpeg", `[904,600]`, 1, "no"},
	} {
		latest = p.num
		c, _ := generate(t, `{"size":`+p.size+`,"fontsize":18,"language":"de"}`, `{"mode":"latest","palette":"bwr","alt":"`+p.alt+`"}`, now)
		if err := writePNG(filepath.Join(out, "xkcd-"+p.name+".png"), c); err != nil {
			t.Fatal(err)
		}
	}
}

func writePNG(name string, c *render.Canvas) error {
	img := image.NewRGBA(image.Rect(0, 0, c.W+20, c.H+20))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			switch b, col := c.InkAt(x, y); {
			case b:
				img.Set(x+10, y+10, color.Black)
			case col:
				img.Set(x+10, y+10, color.RGBA{220, 0, 0, 255})
			}
		}
	}
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
