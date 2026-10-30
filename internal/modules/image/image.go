// Package image holds the three picture modules: Inkyimage (one image from a file or
// URL), Slideshow (a local folder, one image per slot) and Inkyserver (an image
// rendered by an Inkycal server for the module's size). All three share the
// internal/picture pipeline and the same placement: the picture is scaled to fit
// the module and centred, without frame, title or ornament.
package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	goimage "image"
	"inkycal/internal/problem"
	"strconv"
	"strings"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/picture"
	"inkycal/internal/render"
)

// look is how an image is turned into ink and placed; shared by all three modules.
type look struct {
	palette     picture.Palette
	dither      bool
	autoflip    bool
	orientation string // "horizontal", "vertical" or "" (= the module's own shape)
	maxW, maxH  int    // percent of the module, 1–100
}

// rawLook holds the upstream keys behind look. Python reads palette, autoflip,
// orientation, max_width_percent, max_height_percent and dither.
type rawLook struct {
	Palette     string          `json:"palette"`
	Autoflip    config.FlexBool `json:"autoflip"`
	Orientation *string         `json:"orientation"`
	MaxW        json.RawMessage `json:"max_width_percent"`
	MaxH        json.RawMessage `json:"max_height_percent"`
	Dither      json.RawMessage `json:"dither"`
}

func (r rawLook) parse(name string, ditherDefault bool) (look, error) {
	l := look{autoflip: bool(r.Autoflip)}
	var err error
	if l.palette, err = picture.ParsePalette(strings.ToLower(strings.TrimSpace(r.Palette))); err != nil {
		return l, fmt.Errorf("%s: %w", name, err)
	}
	if r.Orientation != nil {
		switch o := strings.ToLower(strings.TrimSpace(*r.Orientation)); o {
		case "", "horizontal", "vertical":
			l.orientation = o
		default:
			return l, fmt.Errorf("%s: orientation %q: want horizontal or vertical", name, *r.Orientation)
		}
	}
	if l.maxW, err = parsePercent(r.MaxW, "max_width_percent"); err != nil {
		return l, fmt.Errorf("%s: %w", name, err)
	}
	if l.maxH, err = parsePercent(r.MaxH, "max_height_percent"); err != nil {
		return l, fmt.Errorf("%s: %w", name, err)
	}
	l.dither = ditherDefault
	if len(bytes.TrimSpace(r.Dither)) > 0 {
		var d config.FlexBool
		if err := json.Unmarshal(r.Dither, &d); err != nil {
			return l, fmt.Errorf("%s: dither: %w", name, err)
		}
		// null and "" keep the default, like Python's `"dither" in config and config["dither"] == False`.
		if s := strings.Trim(string(bytes.TrimSpace(r.Dither)), `"`); s != "null" && s != "" {
			l.dither = bool(d)
		}
	}
	return l, nil
}

// parsePercent mirrors Python's int(float(value)) with a 1–100 range; missing means 100.
func parsePercent(raw json.RawMessage, field string) (int, error) {
	s := strings.Trim(string(bytes.TrimSpace(raw)), `"`)
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return 100, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number between 1 and 100", field)
	}
	if n := int(f); n >= 1 && n <= 100 {
		return n, nil
	}
	return 0, fmt.Errorf("%s must be between 1 and 100", field)
}

// place maps img onto a w×h canvas: flatten, optionally turn, fit inside the
// allowed share of the module and centre it. Only the palette decides whether
// red ink appears, so a bw module never gets colour.
func (l look) place(img goimage.Image, w, h int, fontSize int) *render.Canvas {
	c := render.NewCanvas(w, h, render.FontDefault, fontSize) // pictures carry no text: no theme needed
	flip := false
	if l.autoflip {
		o := l.orientation
		if o == "" {
			o = "vertical"
			if w >= h {
				o = "horizontal"
			}
		}
		flip = picture.NeedsFlip(img.Bounds().Dx(), img.Bounds().Dy(), o)
	}
	// Scale first: rotating and flattening a camera-sized photo would cost
	// seconds and ~100 MB on a Pi Zero; at module size it is nothing.
	bw, bh := box(w, h, l.maxW, l.maxH)
	var fit goimage.Image
	if flip {
		fit = picture.Rotate90(picture.Fit(img, bh, bw))
	} else {
		fit = picture.Fit(img, bw, bh)
	}
	fit = picture.Flatten(fit)
	fw, fh := fit.Bounds().Dx(), fit.Bounds().Dy()
	picture.Draw(c, fit, (w-fw)/2, (h-fh)/2, l.palette, l.dither)
	return c
}

// box is the largest area the picture may use (Python: int(size * percent / 100), at least 1).
func box(w, h, pw, ph int) (int, int) {
	return max(1, w*pw/100), max(1, h*ph/100)
}

// Image is upstream's Inkyimage: one picture from a local path or an http(s) URL.
type Image struct {
	common config.Common
	path   string
	look   look
}

// NewImage builds the module registered as "Inkyimage".
func NewImage(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var cfg struct {
		Path string `json:"path"`
		rawLook
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("image: %w", err)
	}
	if cfg.Path = strings.TrimSpace(cfg.Path); cfg.Path == "" {
		return nil, problem.Mark(problem.Missing, fmt.Errorf("image: config is missing path"))
	}
	if err := modules.CheckMinSize(c, ImageMeta); err != nil {
		return nil, err
	}
	l, err := cfg.rawLook.parse("image", true)
	if err != nil {
		return nil, err
	}
	return &Image{common: c, path: cfg.Path, look: l}, nil
}

func (m *Image) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	img, stale, err := picture.Load(ctx, env.Fetch, m.path)
	if err != nil {
		return nil, fmt.Errorf("image: %w", err)
	}
	env.Stale = env.Stale || stale
	w, h := m.common.InnerSize()
	return m.look.place(img, w, h, int(m.common.FontSize)), nil
}
