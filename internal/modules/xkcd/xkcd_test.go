package xkcd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/picture"
	"inkycal/internal/render"
)

func mustCommon(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

const common480 = `{"size":[480,320],"padding_x":10,"padding_y":10,"fontsize":14,"language":"de"}`

func TestNewUpstreamKeys(t *testing.T) {
	c := mustCommon(t, common480)
	m, err := New(c, json.RawMessage(`{"mode":"random","palette":"bwr","alt":"yes","filter":"yes"}`))
	if err != nil {
		t.Fatal(err)
	}
	x := m.(*Module)
	if !x.random || x.palette != picture.BWR || !x.alt || !x.filter {
		t.Errorf("parsed %+v", x)
	}
	m, err = New(c, json.RawMessage(`{"mode":"latest","palette":"bw","alt":"no","filter":"no"}`))
	if err != nil {
		t.Fatal(err)
	}
	if x := m.(*Module); x.random || x.alt || x.filter || x.palette != picture.BW {
		t.Errorf("parsed %+v", x)
	}
	// Defaults and JSON booleans.
	m, err = New(c, json.RawMessage(`{"alt":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if x := m.(*Module); x.random || !x.alt || x.palette != picture.BW {
		t.Errorf("parsed %+v", x)
	}
}

func TestNewRejects(t *testing.T) {
	c := mustCommon(t, common480)
	for _, raw := range []string{`{"mode":"oldest"}`, `{"palette":"rgb"}`, `{"alt":"maybe"}`} {
		if _, err := New(c, json.RawMessage(raw)); err == nil {
			t.Errorf("%s accepted", raw)
		}
	}
	small := mustCommon(t, `{"size":[100,50]}`)
	if _, err := New(small, nil); err == nil {
		t.Error("tiny module accepted")
	}
}

func TestPickIsStablePerDay(t *testing.T) {
	loc := time.FixedZone("CEST", 2*3600)
	morning := time.Date(2026, 10, 13, 6, 0, 0, 0, loc)
	evening := time.Date(2026, 10, 13, 22, 0, 0, 0, loc)
	if pick(morning, 3310, 0) != pick(evening, 3310, 0) {
		t.Error("pick changes during the day")
	}
	if pick(morning, 3310, 0) != pick(morning, 3399, 0) {
		t.Error("a new comic in the same hundred changes the pick")
	}
	seen := map[int]bool{}
	for d := 0; d < 30; d++ {
		n := pick(morning.AddDate(0, 0, d), 3310, 0)
		if n < 1 || n > 3300 || n == 404 {
			t.Fatalf("pick = %d", n)
		}
		seen[n] = true
	}
	if len(seen) < 25 {
		t.Errorf("only %d different comics in 30 days", len(seen))
	}
	if pick(morning, 3310, 0) == pick(morning, 3310, 1) {
		t.Error("attempts pick the same comic")
	}
	if n := pick(morning, 50, 0); n < 1 || n > 50 {
		t.Errorf("small range pick = %d", n)
	}
}

func TestClean(t *testing.T) {
	cases := map[string]string{
		"CafÃ©":           "Café",
		"Tom &amp; Jerry": "Tom & Jerry",
		"  plain  ":       "plain",
		"Ελληνικά":        "Ελληνικά",
		"naïve (latin)":   "naïve (latin)",
	}
	for in, want := range cases {
		if got := clean(in); got != want {
			t.Errorf("clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRetinaURL(t *testing.T) {
	if got := retinaURL("https://imgs.xkcd.com/comics/standards.png"); got != "https://imgs.xkcd.com/comics/standards_2x.png" {
		t.Errorf("got %q", got)
	}
	for _, u := range []string{"https://imgs.xkcd.com/comics/barrel_cropped_(1).jpg", "https://example.com/a.png", "https://imgs.xkcd.com/comics/a_2x.png"} {
		if got := retinaURL(u); got != "" {
			t.Errorf("retinaURL(%q) = %q", u, got)
		}
	}
}

func TestTooBig(t *testing.T) {
	if tooBig(700, 200, 400, 200) {
		t.Error("1.75x wide counted as too big")
	}
	if !tooBig(701, 200, 400, 200) || !tooBig(100, 400, 400, 200) {
		t.Error("oversized comic passed")
	}
}

func TestFitComic(t *testing.T) {
	s := sheet{W: 800, H: 400, Scale: 2}
	if w, h := fitComic(s, 1000, 1000); w != 800 || h != 400 {
		t.Errorf("2x image enlarged: %dx%d", w, h)
	}
	if w, h := fitComic(s, 400, 1000); w != 400 || h != 200 {
		t.Errorf("width-bound fit: %dx%d", w, h)
	}
	s.Scale = 1
	if w, h := fitComic(s, 5000, 5000); w != 1600 || h != 800 {
		t.Errorf("1x image enlarged more than twice: %dx%d", w, h)
	}
}

func checkLayout(t *testing.T, c *render.Canvas, l layout) {
	t.Helper()
	cr := l.Comic
	if cr.Min.X < 0 || cr.Min.Y < 0 || cr.Max.X > c.W || cr.Max.Y > c.H || cr.Dx() < 1 {
		t.Fatalf("comic %v outside %dx%d", cr, c.W, c.H)
	}
	st := newStyles(l.Body)
	capT := c.CapHeight(st.title)
	if l.TitleBase-capT < 0 {
		t.Errorf("title clipped at the top: baseline %d", l.TitleBase)
	}
	if !l.Side && l.TitleBase > cr.Min.Y {
		t.Errorf("title baseline %d below comic top %d", l.TitleBase, cr.Min.Y)
	}
	if c.Advance(l.Title, st.title) > l.TextW || l.TextX < 0 || l.TextX+l.TextW > c.W {
		t.Errorf("title %q does not fit its column", l.Title)
	}
	_, desc := c.Metrics(st.alt)
	capA := c.CapHeight(st.alt)
	for i, line := range l.AltLines {
		base := l.AltBase + i*l.AltStep
		if !l.Side && base-capA < cr.Max.Y {
			t.Errorf("alt line %d overlaps the comic", i)
		}
		if base+desc > c.H {
			t.Errorf("alt line %d (baseline %d) clipped at %d", i, base, c.H)
		}
		if c.Advance(line, st.alt) > l.TextW {
			t.Errorf("alt line %q wider than %d", line, l.TextW)
		}
	}
	if l.Side && l.TextX < cr.Max.X {
		t.Errorf("text column at %d overlaps comic ending at %d", l.TextX, cr.Max.X)
	}
}

const longAlt = "Although NASA agreed that Voyager 2 'seemed lonely' they decided against launching a companion orbiter mission after further measurements indicated that the spacecraft actually preferred the peace and quiet."

func TestPlan(t *testing.T) {
	cases := []struct {
		name     string
		w, h     int
		s        sheet
		wantSide bool
	}{
		{"tall module, portrait comic", 904, 600, sheet{W: 780, H: 846, Scale: 2, Alt: longAlt}, true},
		{"tall module, wide strip", 904, 600, sheet{W: 1000, H: 567, Scale: 2, Alt: longAlt}, false},
		{"band, portrait comic", 904, 140, sheet{W: 390, H: 423, Scale: 1, Alt: longAlt}, true},
		{"band, wide strip, no alt", 904, 140, sheet{W: 740, H: 230, Scale: 1}, true},
		{"portrait module, portrait comic", 440, 600, sheet{W: 518, H: 588, Scale: 1, Alt: longAlt}, false},
		{"small square", 300, 300, sheet{W: 500, H: 500, Scale: 1, Alt: longAlt + " " + longAlt}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := render.NewCanvas(tc.w, tc.h, render.FontDefault, 18)
			tc.s.Title, tc.s.Number = "Rejected Space Mission Profiles", "xkcd 3310"
			l := plan(c, 18, tc.s)
			if l.Side != tc.wantSide {
				t.Errorf("side = %v, want %v (comic %v)", l.Side, tc.wantSide, l.Comic)
			}
			checkLayout(t, c, l)
			if l.Kicker && l.NumberAt.Y >= l.TitleBase-c.CapHeight(newStyles(18).title) {
				t.Error("kicker overlaps the title")
			}
			if tc.s.Alt != "" && len(l.AltLines) == 0 {
				t.Error("alt text dropped")
			}
		})
	}
}

func TestPlanTruncatesAltText(t *testing.T) {
	c := render.NewCanvas(300, 300, render.FontDefault, 18)
	s := sheet{W: 500, H: 500, Scale: 1, Title: "T", Number: "xkcd 1", Alt: strings.Repeat(longAlt+" ", 5)}
	l := plan(c, 18, s)
	last := l.AltLines[len(l.AltLines)-1]
	if !strings.HasSuffix(last, "…") {
		t.Errorf("last line %q does not show the cut", last)
	}
	if l.Comic.Dy() < c.H/3 {
		t.Errorf("alt text squeezed the comic to %d px", l.Comic.Dy())
	}
}

// lineArt draws a frame and a diagonal stroke with an anti-aliased grey rim, plus a
// red square, so palettes and the darkening can be checked.
func lineArt(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.Black)
		img.Set(x, h-1, color.Black)
		img.Set(x, h/2, color.Gray{Y: 180}) // faint stroke: must survive
	}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(w/4+x, h/4+y, color.RGBA{255, 0, 0, 255})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

type server struct {
	mu   sync.Mutex
	hits map[string]int
}

func (s *server) count(p string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[p]
}

// serveXkcd starts a fake xkcd: latest is 300; 404 and 12 have no image; every
// comic n > 0 links /comics/c<n>.png, only 300 has a _2x variant.
func serveXkcd(t *testing.T, imgW, imgH int) *server {
	t.Helper()
	s := &server{hits: map[string]int{}}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.hits[r.URL.Path]++
		s.mu.Unlock()
		var n int
		switch {
		case r.URL.Path == "/info.0.json":
			n = 300
		case strings.HasSuffix(r.URL.Path, "/info.0.json"):
			fmt.Sscanf(r.URL.Path, "/%d/info.0.json", &n)
		case r.URL.Path == "/comics/c300_2x.png":
			w.Write(lineArt(imgW*2, imgH*2))
			return
		case strings.HasSuffix(r.URL.Path, "_2x.png"):
			http.NotFound(w, r)
			return
		case strings.HasPrefix(r.URL.Path, "/comics/"):
			w.Write(lineArt(imgW, imgH))
			return
		}
		if n <= 0 || n > 300 || n == 404 {
			http.NotFound(w, r)
			return
		}
		img := fmt.Sprintf("%s/comics/c%d.png", srv.URL, n)
		if n == 12 {
			img = ""
		}
		json.NewEncoder(w).Encode(map[string]any{
			"num": n, "title": fmt.Sprintf("Comic %d", n), "safe_title": fmt.Sprintf("Comic %d", n),
			"alt": longAlt, "img": img,
		})
	}))
	old := baseURL
	baseURL = srv.URL
	t.Cleanup(func() {
		baseURL = old
		srv.Close()
	})
	return s
}

func generate(t *testing.T, size, raw string, now time.Time) (*render.Canvas, *modules.Env) {
	t.Helper()
	m, err := New(mustCommon(t, size), json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	f := fetch.New("")
	f.Attempts = 1
	env := &modules.Env{Now: now, Loc: now.Location(), Fetch: f}
	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	return c, env
}

func inks(c *render.Canvas) (black, colour int) {
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b, col := c.InkAt(x, y)
			if b {
				black++
			}
			if col {
				colour++
			}
		}
	}
	return
}

func TestGenerateLatest(t *testing.T) {
	srv := serveXkcd(t, 200, 150)
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	c, env := generate(t, `{"size":[924,620],"padding_x":10,"padding_y":10,"fontsize":18}`, `{"mode":"latest","palette":"bw","alt":"yes"}`, now)
	if c.W != 904 || c.H != 600 {
		t.Fatalf("canvas %dx%d", c.W, c.H)
	}
	if env.Stale {
		t.Error("stale without cache")
	}
	if srv.count("/comics/c300_2x.png") != 1 || srv.count("/comics/c300.png") != 0 {
		t.Errorf("large module should load only the _2x image: %v", srv.hits)
	}
	black, colour := inks(c)
	if black == 0 || colour == 0 {
		t.Errorf("black=%d colour=%d: want comic and text black, number red", black, colour)
	}
	// The faint grey stroke across the comic's middle must survive the mapping.
	l := plan(render.NewCanvas(904, 600, render.FontDefault, 18), 18, sheet{W: 400, H: 300, Scale: 2, Title: "Comic 300", Number: "xkcd 300", Alt: longAlt})
	mid := l.Comic.Min.Y + l.Comic.Dy()/2
	found := false
	for y := mid - 3; y <= mid+3; y++ {
		if b, _ := c.InkAt(l.Comic.Min.X+l.Comic.Dx()/3, y); b {
			found = true
		}
	}
	if !found {
		t.Error("faint stroke lost")
	}
}

func TestGenerateBandUsesNormalImage(t *testing.T) {
	srv := serveXkcd(t, 200, 150)
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	generate(t, `{"size":[904,140],"fontsize":16}`, `{"mode":"latest","palette":"bwr"}`, now)
	if srv.count("/comics/c300_2x.png") != 0 || srv.count("/comics/c300.png") != 1 {
		t.Errorf("small module should load only the 1x image: %v", srv.hits)
	}
}

func TestGenerateRandomSkipsComicsWithoutImage(t *testing.T) {
	serveXkcd(t, 200, 150)
	// Find a day whose first pick is comic 12 (no image) in the range 1..300.
	day := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 5000 && pick(day, 300, 0) != 12; i++ {
		day = day.AddDate(0, 0, 1)
	}
	if pick(day, 300, 0) != 12 {
		t.Skip("no day picks comic 12")
	}
	c, _ := generate(t, common480, `{"mode":"random","alt":"no"}`, day)
	if black, _ := inks(c); black == 0 {
		t.Error("nothing drawn")
	}
}

func TestGenerateRandomFilter(t *testing.T) {
	srv := serveXkcd(t, 2000, 2000) // every comic far too big for 460x300
	now := time.Date(2026, 10, 13, 6, 0, 0, 0, time.UTC)
	generate(t, common480, `{"mode":"random","filter":"yes"}`, now)
	tried := 0
	for p, n := range srv.hits {
		if strings.HasPrefix(p, "/comics/") && !strings.HasSuffix(p, "_2x.png") {
			tried += n
		}
	}
	if tried != maxAttempts {
		t.Errorf("tried %d comics, want %d", tried, maxAttempts)
	}
}

func TestGenerateFailsWithoutNetwork(t *testing.T) {
	old := baseURL
	baseURL = "http://127.0.0.1:1"
	t.Cleanup(func() { baseURL = old })
	m, _ := New(mustCommon(t, common480), nil)
	f := fetch.New("")
	f.Attempts = 1
	if _, err := m.Generate(context.Background(), &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: f}); err == nil {
		t.Error("no error without network")
	}
}
