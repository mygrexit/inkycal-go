package image

import (
	"bytes"
	"context"
	"encoding/json"
	goimage "image"
	"image/color"
	"image/png"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/picture"
	"inkycal/internal/render"
)

func mustCommon(t *testing.T, w, h int) config.Common {
	t.Helper()
	raw := `{"size":[` + strconv.Itoa(w+80) + `,` + strconv.Itoa(h+40) + `],"padding_x":40,"padding_y":20,"fontsize":22,"language":"de"}`
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// scene paints a landscape: sky gradient, red sun, layered hills, a tree — enough
// tones and one saturated colour to show dithering and the palette rules.
func scene(w, h int) *goimage.RGBA {
	img := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	sx, sy, sr := float64(w)*0.68, float64(h)*0.34, float64(h)*0.14
	for y := range h {
		for x := range w {
			fy := float64(y) / float64(h)
			v := uint8(235 - 90*fy)
			col := color.RGBA{v, v, uint8(min(255, int(v)+15)), 255}
			if math.Hypot(float64(x)-sx, float64(y)-sy) < sr {
				col = color.RGBA{230, 30, 25, 255}
			}
			fx := float64(x) / float64(w)
			if y > int(float64(h)*(0.62+0.08*math.Sin(fx*5.1))) {
				col = color.RGBA{120, 120, 110, 255}
			}
			if y > int(float64(h)*(0.75+0.05*math.Sin(fx*9.3+1))) {
				col = color.RGBA{50, 55, 45, 255}
			}
			// a tree: trunk and crown
			tx, ty := float64(w)*0.22, float64(h)*0.5
			if math.Abs(float64(x)-tx) < float64(w)*0.008 && float64(y) > ty && fy < 0.8 {
				col = color.RGBA{30, 25, 20, 255}
			}
			if math.Hypot(float64(x)-tx, (float64(y)-ty)*1.3) < float64(h)*0.12 {
				col = color.RGBA{70, 80, 60, 255}
			}
			img.SetRGBA(x, y, col)
		}
	}
	return img
}

func pngBytes(t *testing.T, img goimage.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writePNG(t *testing.T, path string, img goimage.Image) {
	t.Helper()
	if err := os.WriteFile(path, pngBytes(t, img), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newEnv(t *testing.T) *modules.Env {
	f := fetch.New(t.TempDir())
	f.Attempts, f.Backoff = 1, 0
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	return &modules.Env{Now: now, Loc: time.UTC, Fetch: f}
}

// inkBox returns the bounding box of all ink and whether any red was set.
func inkBox(c *render.Canvas) (r goimage.Rectangle, red bool) {
	first := true
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b, col := c.InkAt(x, y)
			red = red || col
			if !b && !col {
				continue
			}
			p := goimage.Rect(x, y, x+1, y+1)
			if first {
				r, first = p, false
			} else {
				r = r.Union(p)
			}
		}
	}
	return r, red
}

func TestParseUpstreamKeys(t *testing.T) {
	c := mustCommon(t, 400, 200)
	m, err := ImageDef.New(c, json.RawMessage(`{"path":"/x.png","palette":"bwr","autoflip":"True","orientation":"vertical",
		"max_width_percent":"50.7","max_height_percent":80,"dither":false}`))
	if err != nil {
		t.Fatal(err)
	}
	got := m.(*Image).look
	want := look{palette: picture.BWR, dither: false, autoflip: true, orientation: "vertical", maxW: 50, maxH: 80}
	if got != want {
		t.Fatalf("look = %+v, want %+v", got, want)
	}
	// Defaults: dither on for Inkyimage, off for Inkyserver; 100 %; 16gray → bw.
	m, _ = ImageDef.New(c, json.RawMessage(`{"path":"a.jpg","palette":"16gray","autoflip":false,"orientation":"horizontal","dither":null}`))
	if l := m.(*Image).look; !l.dither || l.maxW != 100 || l.maxH != 100 || l.palette != picture.BW {
		t.Fatalf("defaults: %+v", l)
	}
	s, err := ServerDef.New(c, json.RawMessage(`{"path":"http://h/{width}x{height}","palette":"bw","path_body":"a, b,c","dither":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if srv := s.(*Server); srv.look.dither || strings.Join(srv.body, "|") != "a|b|c" {
		t.Fatalf("server: %+v", srv)
	}
	s, _ = ServerDef.New(c, json.RawMessage(`{"path":"http://h/","palette":"bwr","path_body":["https://x/a.ics","https://x/b.ics"],"dither":true}`))
	if srv := s.(*Server); !srv.look.dither || len(srv.body) != 2 {
		t.Fatalf("server list body: %+v", srv)
	}
	if ss, err := SlideshowDef.New(c, json.RawMessage(`{"path":"/pics","palette":"bwy","autoflip":true,"orientation":"horizontal"}`)); err != nil || ss.(*Slideshow).interval != 1440 {
		t.Fatalf("slideshow: %v %+v", err, ss)
	}
	for _, bad := range []string{
		`{"palette":"bw"}`,
		`{"path":"x","palette":"rgb"}`,
		`{"path":"x","palette":"bw","orientation":"diagonal"}`,
		`{"path":"x","palette":"bw","max_width_percent":0}`,
		`{"path":"x","palette":"bw","max_height_percent":"101"}`,
		`{"path":"x","palette":"bw","max_height_percent":"lots"}`,
		`{"path":"x","palette":"bw","dither":"maybe"}`,
	} {
		if _, err := ImageDef.New(c, json.RawMessage(bad)); err == nil {
			t.Errorf("ImageDef.New(%s): want error", bad)
		}
	}
	if _, err := SlideshowDef.New(c, json.RawMessage(`{"path":"x","interval":2000}`)); err == nil {
		t.Error("slideshow interval 2000: want error")
	}
}

func TestServerURL(t *testing.T) {
	got := serverURL("https://s/panel/calendar/{model}?width={width}&height={height}&x={{y}}", 904, 600)
	if got != "https://s/panel/calendar/{model}?width=904&height=600&x={y}" {
		t.Fatal(got)
	}
}

func TestSlotWalksConsecutively(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Berlin")
	at := func(d, h, m int) time.Time { return time.Date(2026, 10, d, h, m, 0, 0, loc) }
	files := []string{"a", "b", "c", "d", "e"}
	// Twice a day with interval 720: every update shows the next file, across midnight and DST (25 Oct).
	var seq []string
	for d := 23; d <= 27; d++ {
		seq = append(seq, pickSlide(files, at(d, 6, 0), 720), pickSlide(files, at(d, 18, 0), 720))
	}
	for i := 1; i < len(seq); i++ {
		if prev := strings.Index("abcde", seq[i-1]); seq[i] != files[(prev+1)%5] {
			t.Fatalf("sequence %v does not walk", seq)
		}
	}
	// A wake a few seconds early stays in the slot; a run in the same slot repeats.
	if slot(at(13, 5, 59), 60) != slot(at(13, 6, 0), 60) || slot(at(13, 6, 20), 60) != slot(at(13, 6, 0), 60) {
		t.Fatal("early or late wake changed the slot")
	}
	// Default one per day: the same all day, the next file tomorrow.
	if slot(at(13, 6, 0), 1440) != slot(at(13, 22, 0), 1440) || slot(at(14, 6, 0), 1440) != slot(at(13, 6, 0), 1440)+1 {
		t.Fatal("daily slot")
	}
}

func TestSlideFilesFilterAndSort(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"b.JPG", "a.png", "c.webp", "notes.txt", ".hidden.png", "d.jpeg"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	os.Mkdir(filepath.Join(dir, "sub.png"), 0o755)
	got, err := slideFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, g := range got {
		names = append(names, filepath.Base(g))
	}
	if strings.Join(names, ",") != "a.png,b.JPG,c.webp,d.jpeg" {
		t.Fatal(names)
	}
}

func TestPlaceCentresAndRespectsPalette(t *testing.T) {
	img := scene(300, 200)
	for _, tc := range []struct {
		pal     picture.Palette
		wantRed bool
	}{{picture.BW, false}, {picture.BWR, true}, {picture.BWY, true}} {
		l := look{palette: tc.pal, dither: true, maxW: 100, maxH: 100}
		c := l.place(img, 904, 140, 22)
		box, red := inkBox(c)
		if red != tc.wantRed {
			t.Errorf("%s: red=%v", tc.pal, red)
		}
		// 300×200 fit in 904×140 → 210×140, centred horizontally.
		if box.Dy() > 140 || box.Dx() > 210 || box.Min.X < (904-210)/2 || box.Max.X > (904+210)/2 {
			t.Errorf("%s: ink box %v not centred", tc.pal, box)
		}
	}
	// max percents shrink the box and keep it centred.
	l := look{palette: picture.BW, dither: true, maxW: 50, maxH: 50}
	box, _ := inkBox(l.place(img, 400, 400, 22))
	if box.Dx() > 200 || box.Min.X < 100 || box.Min.Y < 100 || box.Max.Y > 300 {
		t.Errorf("50%%: ink box %v", box)
	}
	// autoflip without orientation follows the module's shape: a portrait module turns a landscape image.
	l = look{palette: picture.BW, dither: false, autoflip: true, maxW: 100, maxH: 100}
	box, _ = inkBox(l.place(img, 200, 600, 22))
	if box.Dy() <= box.Dx() {
		t.Errorf("autoflip: ink box %v not portrait", box)
	}
}

func TestImageGenerateFromURLAndFile(t *testing.T) {
	data := pngBytes(t, scene(120, 80))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer ts.Close()
	file := filepath.Join(t.TempDir(), "p.png")
	os.WriteFile(file, data, 0o644)
	for _, path := range []string{ts.URL + "/p.png", file} {
		m, err := ImageDef.New(mustCommon(t, 904, 140), json.RawMessage(`{"path":"`+path+`","palette":"bwr"}`))
		if err != nil {
			t.Fatal(err)
		}
		c, err := m.Generate(context.Background(), newEnv(t))
		if err != nil {
			t.Fatal(err)
		}
		if c.W != 904 || c.H != 140 {
			t.Fatalf("canvas %dx%d", c.W, c.H)
		}
		if box, red := inkBox(c); !red || box.Empty() {
			t.Fatalf("%s: nothing drawn (%v, red=%v)", path, box, red)
		}
	}
	m, _ := ImageDef.New(mustCommon(t, 904, 140), json.RawMessage(`{"path":"`+ts.URL+`/missing","palette":"bw"}`))
	ts.Config.Handler = http.NotFoundHandler()
	if _, err := m.Generate(context.Background(), newEnv(t)); err == nil {
		t.Fatal("404: want error")
	}
}

func TestSlideshowGenerate(t *testing.T) {
	dir := t.TempDir()
	writePNG(t, filepath.Join(dir, "1.png"), scene(90, 60))
	// The second slide is solid black, so the two are easy to tell apart.
	black := goimage.NewRGBA(goimage.Rect(0, 0, 60, 60))
	for i := 3; i < len(black.Pix); i += 4 {
		black.Pix[i] = 255
	}
	writePNG(t, filepath.Join(dir, "2.png"), black)
	m, err := SlideshowDef.New(mustCommon(t, 300, 300), json.RawMessage(`{"path":"`+dir+`","palette":"bw","interval":60}`))
	if err != nil {
		t.Fatal(err)
	}
	env := newEnv(t)
	seen := map[bool]bool{}
	for h := 6; h < 8; h++ {
		env.Now = time.Date(2026, 10, 13, h, 0, 0, 0, time.UTC)
		c, err := m.Generate(context.Background(), env)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := c.InkAt(1, 150) // the black square fills the module edge to edge
		seen[b] = true
	}
	if len(seen) != 2 {
		t.Fatal("consecutive hours showed the same slide")
	}
	empty, _ := SlideshowDef.New(mustCommon(t, 300, 300), json.RawMessage(`{"path":"`+t.TempDir()+`","palette":"bw"}`))
	if _, err := empty.Generate(context.Background(), env); err == nil {
		t.Fatal("empty folder: want error")
	}
}

func TestServerGenerateGetAndPost(t *testing.T) {
	var gotQuery, gotBody, gotMethod string
	data := pngBytes(t, scene(904, 140))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotQuery = r.Method, r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Write(data)
	}))
	defer ts.Close()
	c := mustCommon(t, 904, 140)
	m, _ := ServerDef.New(c, json.RawMessage(`{"path":"`+ts.URL+`/panel?width={width}&height={height}","palette":"bw","path_body":"","dither":false}`))
	if _, err := m.Generate(context.Background(), newEnv(t)); err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodGet || gotQuery != "width=904&height=140" {
		t.Fatalf("GET: %s %q", gotMethod, gotQuery)
	}
	m, _ = ServerDef.New(c, json.RawMessage(`{"path":"`+ts.URL+`/panel","palette":"bwr","path_body":"https://a/x.ics,https://b/y.ics","dither":true}`))
	out, err := m.Generate(context.Background(), newEnv(t))
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost || gotBody != `["https://a/x.ics","https://b/y.ics"]` {
		t.Fatalf("POST: %s %s", gotMethod, gotBody)
	}
	if _, red := inkBox(out); !red {
		t.Fatal("bwr server image has no red")
	}
}

// TestPreview writes PNGs for a visual check when INKYCAL_PREVIEW_DIR is set.
func TestPreview(t *testing.T) {
	dir := os.Getenv("INKYCAL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("INKYCAL_PREVIEW_DIR not set")
	}
	src := scene(1200, 800)
	for _, tc := range []struct {
		name string
		w, h int
		l    look
	}{
		{"image-904x140-bwr", 904, 140, look{palette: picture.BWR, dither: true, maxW: 100, maxH: 100}},
		{"image-904x600-bwr", 904, 600, look{palette: picture.BWR, dither: true, maxW: 100, maxH: 100}},
		{"image-904x600-bw", 904, 600, look{palette: picture.BW, dither: true, maxW: 100, maxH: 100}},
		{"image-904x600-bwr-60pct-nodither", 904, 600, look{palette: picture.BWR, dither: false, maxW: 60, maxH: 60}},
		{"slideshow-904x600-autoflip-vertical", 904, 600, look{palette: picture.BWR, dither: true, autoflip: true, orientation: "vertical", maxW: 100, maxH: 100}},
	} {
		c := tc.l.place(src, tc.w, tc.h, 22)
		out := goimage.NewRGBA(goimage.Rect(0, 0, c.W, c.H))
		for y := 0; y < c.H; y++ {
			for x := 0; x < c.W; x++ {
				col := color.RGBA{255, 255, 255, 255}
				if b, r := c.InkAt(x, y); r {
					col = color.RGBA{255, 0, 0, 255}
				} else if b {
					col = color.RGBA{0, 0, 0, 255}
				}
				out.SetRGBA(x, y, col)
			}
		}
		writePNG(t, filepath.Join(dir, tc.name+".png"), out)
	}
}

// Offline, a POSTed image comes from the cache like a fetched one: the panel keeps it.
func TestServerPostFallsBackToCache(t *testing.T) {
	data := pngBytes(t, scene(904, 140))
	down := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if down {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write(data)
	}))
	defer ts.Close()
	m, _ := ServerDef.New(mustCommon(t, 904, 140), json.RawMessage(`{"path":"`+ts.URL+`/panel","palette":"bw","path_body":"https://a/x.ics"}`))
	env := newEnv(t)
	env.Fetch.Backoff = time.Millisecond
	if _, err := m.Generate(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	down = true
	if _, err := m.Generate(context.Background(), env); err != nil || !env.Stale {
		t.Errorf("server down: err %v stale %v, want the cached image", err, env.Stale)
	}
}
