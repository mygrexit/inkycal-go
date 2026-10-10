// Package feeds ports inkycal_feeds.py: headlines and summaries from RSS/Atom feeds.
package feeds

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

type Module struct {
	common  config.Common
	urls    []string
	shuffle bool
	rnd     *rand.Rand
}

type rawConfig struct {
	FeedURLs     config.StringList `json:"feed_urls"`
	ShuffleFeeds *config.FlexBool  `json:"shuffle_feeds"`
}

var tagRe = regexp.MustCompile(`<[^<]+?>`)

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("feeds: config: %w", err)
	}
	if len(rc.FeedURLs) == 0 {
		return nil, errors.New("feeds: feed_urls is missing")
	}
	if w, h := c.InnerSize(); w < 50 || h < 20 {
		return nil, fmt.Errorf("feeds: inner size %dx%d too small (min 50x20)", w, h)
	}
	m := &Module{
		common:  c,
		urls:    []string(rc.FeedURLs),
		shuffle: true,
		rnd:     rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x9e3779b97f4a7c15)),
	}
	if rc.ShuffleFeeds != nil {
		m.shuffle = bool(*rc.ShuffleFeeds)
	}
	return m, nil
}

// entry formats one item as "•Title: summary": tags stripped before entities
// are decoded (escaped markup stays text), cut at the first line break.
func entry(title, description string) string {
	text := strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(description, "")))
	e := "•" + strings.TrimSpace(title) + ": " + text
	if i := strings.IndexByte(e, '\n'); i >= 0 {
		e = e[:i]
	}
	return strings.TrimRight(e, " \t\r")
}

// entriesFromFeed keeps only items with a summary, like Python's `"summary" in posts`.
func entriesFromFeed(f *gofeed.Feed) []string {
	var out []string
	for _, it := range f.Items {
		if strings.TrimSpace(it.Description) == "" {
			continue
		}
		out = append(out, entry(it.Title, it.Description))
	}
	return out
}

// fetchEntries loads all feeds; broken feeds are logged and skipped,
// an error is returned only if none could be loaded.
func fetchEntries(ctx context.Context, fc *fetch.Client, urls []string) ([]string, bool, error) {
	parser := gofeed.NewParser()
	var (
		out   []string
		stale bool
		errs  []error
		ok    int
	)
	for _, u := range urls {
		res, err := fc.Get(ctx, u, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		feed, err := parser.Parse(bytes.NewReader(res.Body))
		if err != nil {
			errs = append(errs, fmt.Errorf("parse %s: %w", u, err))
			continue
		}
		ok++
		stale = stale || res.Stale
		out = append(out, entriesFromFeed(feed)...)
	}
	if ok == 0 {
		return nil, false, fmt.Errorf("feeds: no feed could be loaded: %w", errors.Join(errs...))
	}
	for _, err := range errs {
		slog.Warn("feeds: skipping feed", "err", err)
	}
	return out, stale, nil
}

// selectLines wraps entries until the next one would not fit completely.
func selectLines(c *render.Canvas, entries []string, width, maxLines int) []string {
	if len(entries) > maxLines {
		entries = entries[:maxLines]
	}
	var lines []string
	for _, e := range entries {
		wrapped := c.Wrap(e, width)
		if len(lines)+len(wrapped) > maxLines {
			break
		}
		lines = append(lines, wrapped...)
	}
	return lines
}

// arrange shuffles the entries in place when shuffle_feeds is on.
func (m *Module) arrange(entries []string) []string {
	if m.shuffle {
		m.rnd.Shuffle(len(entries), func(i, j int) { entries[i], entries[j] = entries[j], entries[i] })
	}
	return entries
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	entries, stale, err := fetchEntries(ctx, env.Fetch, m.urls)
	if err != nil {
		return nil, err
	}
	env.Stale = env.Stale || stale
	entries = m.arrange(entries)

	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontDefault, int(m.common.FontSize))
	const lineSpacing = 1
	lh := c.LineHeight() + lineSpacing
	maxLines := h / (lh + lineSpacing)
	top := (h % lh) / 2
	lines := selectLines(c, entries, w, maxLines)
	if len(lines) == 0 {
		slog.Info("feeds: nothing fits into the module", "entries", len(entries))
	}
	for i, line := range lines {
		c.Write(0, top+i*lh, w, lh, line, render.TextOpts{Align: render.AlignLeft})
	}
	return c, nil
}
