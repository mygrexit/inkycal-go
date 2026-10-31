package image

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/picture"
	"inkycal/internal/problem"
	"inkycal/internal/render"
)

// Formats picture decodes. Python only took png/jpg/jpeg; the rest costs nothing extra.
var slideExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".webp": true}

// Slideshow is upstream's Slideshow: the images of one local folder in turn.
// Python advanced one image per run and kept the index in a cache file; we have
// no state, so the image is chosen from the time: one per interval, walking the
// folder in name order.
type Slideshow struct {
	common   config.Common
	dir      string
	interval int // minutes each image stays; divides the day into slots
	look     look
}

// NewSlideshow builds the module registered as "Slideshow".
//
// Besides the upstream keys it reads "interval" (minutes, default 1440 = one image
// per day). Set it to the time between wakes – e.g. 720 for update_times
// ["06:00", "18:00"] or 60 for update_interval 60 – to show the next image on every update.
func NewSlideshow(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var cfg struct {
		Path     string         `json:"path"`
		Interval config.FlexInt `json:"interval"`
		rawLook
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("slideshow: %w", err)
	}
	if cfg.Path = strings.TrimSpace(cfg.Path); cfg.Path == "" {
		return nil, problem.Mark(problem.Missing, fmt.Errorf("slideshow: config is missing path"))
	}
	if cfg.Interval < 0 || cfg.Interval > 1440 {
		return nil, fmt.Errorf("slideshow: interval %d: want 1–1440 minutes", cfg.Interval)
	}
	if cfg.Interval == 0 {
		cfg.Interval = 1440
	}
	if err := modules.CheckMinSize(c, SlideshowMeta); err != nil {
		return nil, err
	}
	l, err := cfg.rawLook.parse("slideshow", true)
	if err != nil {
		return nil, err
	}
	return &Slideshow{common: c, dir: cfg.Path, interval: int(cfg.Interval), look: l}, nil
}

// slideFiles lists the folder's images (not recursive, no dot files) in name order.
func slideFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || strings.HasPrefix(n, ".") || !slideExt[strings.ToLower(filepath.Ext(n))] {
			continue
		}
		out = append(out, filepath.Join(dir, n))
	}
	sort.Strings(out)
	return out, nil
}

// slot numbers the intervals since 1970 in local time: day*slotsPerDay + slot of the day.
// Consecutive slots give consecutive numbers across midnight (the day's last slot is
// just shorter when interval does not divide 24 h). Slots follow the wall clock, so
// on DST days a sub-daily interval skips or repeats one image; daily ones are unaffected.
// Slots start up to 30 minutes before the aligned times (06:00, 12:00, …), so a
// wake a few seconds early never shows the previous slot's image again.
func slot(now time.Time, interval int) int64 {
	now = now.Add(time.Duration(min(interval/2, 30)) * time.Minute)
	day := modules.DayNumber(now)
	perDay := int64((1440 + interval - 1) / interval)
	minute := now.Hour()*60 + now.Minute()
	return day*perDay + int64(minute/interval)
}

// pickSlide returns the file for now; the slot after shows the next file.
func pickSlide(files []string, now time.Time, interval int) string {
	n := int64(len(files))
	return files[((slot(now, interval)%n)+n)%n]
}

func (m *Slideshow) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	files, err := slideFiles(m.dir)
	if err != nil {
		return nil, fmt.Errorf("slideshow: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("slideshow: no PNG or JPG images in %s", m.dir)
	}
	img, _, err := picture.Load(ctx, env.Fetch, pickSlide(files, env.Now, m.interval))
	if err != nil {
		return nil, fmt.Errorf("slideshow: %w", err)
	}
	w, h := m.common.InnerSize()
	return m.look.place(img, w, h, int(m.common.FontSize)), nil
}
