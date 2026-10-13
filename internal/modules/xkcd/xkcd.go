// Package xkcd shows a comic from xkcd.com by Randall Munroe: the latest one or a
// random pick that changes once a day, with its title and, optionally, the alt text
// (Python: modules/inkycal_xkcd.py, settings name "Xkcd").
package xkcd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"image"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/picture"
	"inkycal/internal/render"
)

// baseURL is the JSON API; tests point it at an httptest server.
var baseURL = "https://xkcd.com"

const (
	// maxAttempts bounds the comics tried in random mode (missing image, filter).
	maxAttempts = 4
	// filterRatio: with "filter", a random comic is skipped when it is more than this
	// many times larger than its space in either direction (same as the Python module).
	filterRatio = 1.75
	// retinaFrom: comic spaces at least this tall load the _2x image when there is one.
	retinaFrom = 300
)

// yesNo reads the Python options "yes"/"no" as well as JSON booleans.
type yesNo bool

func (y *yesNo) UnmarshalJSON(b []byte) error {
	switch strings.ToLower(strings.Trim(string(bytes.TrimSpace(b)), `"`)) {
	case "yes", "true", "1":
		*y = true
	case "no", "false", "0", "null", "":
		*y = false
	default:
		return fmt.Errorf("want \"yes\" or \"no\", got %s", b)
	}
	return nil
}

type settings struct {
	Mode    string `json:"mode"`
	Palette string `json:"palette"`
	Alt     yesNo  `json:"alt"`
	Filter  yesNo  `json:"filter"`
}

type Module struct {
	common  config.Common
	random  bool
	palette picture.Palette
	alt     bool
	filter  bool
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	if w, h := c.InnerSize(); w < 160 || h < 80 {
		return nil, fmt.Errorf("xkcd: inner size %dx%d too small (min 160x80)", w, h)
	}
	var s settings
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, fmt.Errorf("xkcd: %w", err)
		}
	}
	m := &Module{common: c, alt: bool(s.Alt), filter: bool(s.Filter)}
	switch strings.ToLower(strings.TrimSpace(s.Mode)) {
	case "", "latest":
	case "random":
		m.random = true
	default:
		return nil, fmt.Errorf("xkcd: mode %q: want latest or random", s.Mode)
	}
	p, err := picture.ParsePalette(strings.ToLower(strings.TrimSpace(s.Palette)))
	if err != nil {
		return nil, fmt.Errorf("xkcd: %w", err)
	}
	m.palette = p
	return m, nil
}

// comic is the part of xkcd's info.0.json the module uses.
type comic struct {
	Num       int    `json:"num"`
	Title     string `json:"title"`
	SafeTitle string `json:"safe_title"`
	Alt       string `json:"alt"`
	Img       string `json:"img"`
}

func (c comic) title() string {
	t := c.SafeTitle
	if t == "" {
		t = c.Title
	}
	return clean(t)
}

// clean decodes HTML entities and repairs the doubly encoded UTF-8 ("Ã©" for "é")
// found in some of the older entries.
func clean(s string) string {
	s = html.UnescapeString(strings.TrimSpace(s))
	latin := make([]byte, 0, len(s))
	multi := false
	for _, r := range s {
		if r > 0xFF {
			return s
		}
		multi = multi || r >= 0x80
		latin = append(latin, byte(r))
	}
	if multi && utf8.Valid(latin) {
		return string(latin)
	}
	return s
}

func getComic(ctx context.Context, f *fetch.Client, num int) (comic, bool, error) {
	url := baseURL + "/info.0.json"
	if num > 0 {
		url = fmt.Sprintf("%s/%d/info.0.json", baseURL, num)
	}
	var c comic
	stale, err := f.GetJSON(ctx, url, nil, &c)
	if err != nil {
		return comic{}, false, fmt.Errorf("xkcd: %w", err)
	}
	if c.Num <= 0 {
		return comic{}, stale, fmt.Errorf("xkcd: %s: no comic number", url)
	}
	return c, stale, nil
}

// retinaURL is the double-resolution variant xkcd publishes for most newer comics.
func retinaURL(img string) string {
	if !strings.HasPrefix(img, "https://imgs.xkcd.com/") && !strings.HasPrefix(img, baseURL) {
		return ""
	}
	if s, ok := strings.CutSuffix(img, ".png"); ok && !strings.HasSuffix(s, "_2x") {
		return s + "_2x.png"
	}
	return ""
}

// loadImage returns the comic image and its scale relative to the normal (1x) image.
// The _2x image is only worth its bigger download when the comic is shown large.
func loadImage(ctx context.Context, f *fetch.Client, c comic, large bool) (image.Image, int, bool, error) {
	if c.Img == "" || strings.HasSuffix(c.Img, "/comics/") {
		return nil, 0, false, fmt.Errorf("xkcd %d has no image (interactive comic?)", c.Num)
	}
	if u := retinaURL(c.Img); large && u != "" {
		img, stale, err := picture.Load(ctx, f, u)
		if err == nil {
			return img, 2, stale, nil
		}
		var se *fetch.StatusError
		if !errors.As(err, &se) || se.Code != http.StatusNotFound {
			slog.Info("xkcd: retina image unavailable", "num", c.Num, "err", err)
		}
	}
	img, stale, err := picture.Load(ctx, f, c.Img)
	if err != nil {
		return nil, 0, false, fmt.Errorf("xkcd %d: %w", c.Num, err)
	}
	return img, 1, stale, nil
}

// splitmix64 scrambles the day number so that consecutive days are far apart.
func splitmix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// pick returns the random comic of now's calendar day for the given attempt. The
// range is the latest number rounded down to a hundred, so a comic published during
// the day does not change the pick (the newest ones are what "latest" shows anyway).
// Comic 404 does not exist.
func pick(now time.Time, latest, attempt int) int {
	y, m, d := now.Date()
	day := time.Date(y, m, d, 12, 0, 0, 0, time.UTC).Unix() / 86400
	bound := latest / 100 * 100
	if bound < 1 {
		bound = latest
	}
	n := 1 + int(splitmix64(uint64(day)*8+uint64(attempt))%uint64(max(bound, 1)))
	if n == 404 {
		n = 405
	}
	return n
}

// tooBig is the Python filter: the comic is more than filterRatio times its space.
func tooBig(imgW, imgH, boxW, boxH int) bool {
	return float64(imgW) > filterRatio*float64(boxW) || float64(imgH) > filterRatio*float64(boxH)
}

type loaded struct {
	comic comic
	img   image.Image
	scale int // 2 for the _2x image
}

func (m *Module) load(ctx context.Context, env *modules.Env, boxW, boxH int) (loaded, error) {
	latest, stale, err := getComic(ctx, env.Fetch, 0)
	if err != nil {
		return loaded{}, err
	}
	env.Stale = env.Stale || stale
	large := boxH >= retinaFrom
	if !m.random {
		img, scale, stale, err := loadImage(ctx, env.Fetch, latest, large)
		if err != nil {
			return loaded{}, err
		}
		env.Stale = env.Stale || stale
		return loaded{latest, img, scale}, nil
	}

	var best loaded
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		n := pick(env.Now, latest.Num, attempt)
		cand, stale, err := tryComic(ctx, env.Fetch, n, large)
		if err != nil {
			lastErr = err
			slog.Info("xkcd: skipping comic", "num", n, "err", err)
			continue
		}
		b := cand.img.Bounds()
		if !m.filter || !tooBig(b.Dx()/cand.scale, b.Dy()/cand.scale, boxW, boxH) {
			env.Stale = env.Stale || stale
			return cand, nil
		}
		slog.Info("xkcd: comic too big for the module, trying another", "num", n, "size", b.Size())
		if best.img == nil || area(cand) < area(best) {
			best = cand
			env.Stale = env.Stale || stale
		}
	}
	if best.img != nil {
		return best, nil // nothing passed the filter: show the smallest
	}
	return loaded{}, lastErr
}

func tryComic(ctx context.Context, f *fetch.Client, n int, large bool) (loaded, bool, error) {
	c, stale, err := getComic(ctx, f, n)
	if err != nil {
		return loaded{}, false, err
	}
	img, scale, stale2, err := loadImage(ctx, f, c, large)
	if err != nil {
		return loaded{}, false, err
	}
	return loaded{c, img, scale}, stale || stale2, nil
}

func area(l loaded) int {
	b := l.img.Bounds()
	return b.Dx() * b.Dy() / (l.scale * l.scale)
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	w, h := m.common.InnerSize()
	body := int(m.common.FontSize)
	c := render.NewCanvas(w, h, render.FontDefault, body)
	// The comic gets at most the full module; the filter and the retina choice only
	// need this rough box, the exact one depends on the comic's shape.
	got, err := m.load(ctx, env, w, h-body*2)
	if err != nil {
		return nil, err
	}
	b := got.img.Bounds()
	alt := ""
	if m.alt {
		alt = clean(got.comic.Alt)
	}
	l := plan(c, body, sheet{
		W: b.Dx(), H: b.Dy(), Scale: got.scale,
		Title: got.comic.title(), Number: fmt.Sprintf("xkcd %d", got.comic.Num), Alt: alt,
	})
	draw(c, l, got.img, m.palette)
	return c, nil
}
