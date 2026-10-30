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
	"sync"
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

var (
	// Block tags separate words, inline tags (<b>, <sub>) sit inside them.
	blockTagRe = regexp.MustCompile(`(?i)</?(p|br|div|li|ul|ol|h[1-6]|tr|td|th|table|blockquote|hr)\b[^<]*?>`)
	tagRe      = regexp.MustCompile(`<[^<]+?>`)
)

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if err := json.Unmarshal(raw, &rc); err != nil {
		return nil, fmt.Errorf("feeds: config: %w", err)
	}
	if len(rc.FeedURLs) == 0 {
		return nil, errors.New("feeds: feed_urls is missing")
	}
	if err := modules.CheckMinSize(c, Meta); err != nil {
		return nil, err
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

// item is one feed entry as drawn: headline, summary and the feed it came from.
type item struct {
	Title, Summary, Source string
}

// entry cleans one feed item: tags stripped before entities are decoded
// (escaped markup stays text), whitespace and line breaks folded.
func entry(title, description, source string) item {
	text := html.UnescapeString(tagRe.ReplaceAllString(blockTagRe.ReplaceAllString(description, " "), ""))
	return item{
		Title:   strings.Join(strings.Fields(title), " "),
		Summary: strings.Join(strings.Fields(text), " "),
		Source:  strings.Join(strings.Fields(source), " "),
	}
}

// entriesFromFeed keeps only items with a summary, like Python's `"summary" in posts`.
func entriesFromFeed(f *gofeed.Feed) []item {
	var out []item
	for _, it := range f.Items {
		if strings.TrimSpace(it.Description) == "" {
			continue
		}
		out = append(out, entry(it.Title, it.Description, f.Title))
	}
	return out
}

// fetchEntries loads all feeds; broken feeds are logged and skipped,
// an error is returned only if none could be loaded.
func fetchEntries(ctx context.Context, fc *fetch.Client, urls []string) ([]item, bool, error) {
	parser := gofeed.NewParser()
	var (
		out   []item
		stale bool
		errs  []error
		ok    int
	)
	// Fetch in parallel (a wake waits for the slowest feed, not the sum), parse in order.
	results := make([]fetch.Result, len(urls))
	fetchErrs := make([]error, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Go(func() {
			results[i], fetchErrs[i] = fc.Get(ctx, u, nil)
		})
	}
	wg.Wait()
	for i, u := range urls {
		res, err := results[i], fetchErrs[i]
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

// arrange shuffles the entries in place when shuffle_feeds is on.
func (m *Module) arrange(entries []item) []item {
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
	c := env.Canvas(w, h, render.FontDefault, int(m.common.FontSize))
	if len(entries) == 0 {
		slog.Info("feeds: no entries with a summary")
	}
	draw(c, entries, int(m.common.FontSize), m.common.Language)
	return c, nil
}
