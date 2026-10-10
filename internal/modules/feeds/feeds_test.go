package feeds

import (
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"

	"inkycal/internal/config"
	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

const testRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Test</title>
<item><title>Eins</title><description>&lt;p&gt;Erster &amp;amp; bester&lt;/p&gt;</description></item>
<item><title>Zwei</title><description>Zweiter Text
mit Umbruch</description></item>
<item><title>Ohne Beschreibung</title></item>
</channel></rss>`

const testAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>A</title>
<entry><title>Atom-Titel</title><summary>Kurzfassung</summary><id>1</id><updated>2026-10-10T00:00:00Z</updated></entry>
</feed>`

func mustCommon(t *testing.T, raw string) config.Common {
	t.Helper()
	c, err := config.ParseCommon(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEntry(t *testing.T) {
	cases := []struct{ title, desc, want string }{
		{"Eins", "<p>Erster &amp; bester</p>", "•Eins: Erster & bester"},
		{"Zwei", "Zweiter Text\nmit Umbruch", "•Zwei: Zweiter Text"},
		{" Drei ", "&lt;b&gt; bleibt Text", "•Drei: <b> bleibt Text"},
	}
	for _, c := range cases {
		if got := entry(c.title, c.desc); got != c.want {
			t.Errorf("entry(%q, %q) = %q, want %q", c.title, c.desc, got, c.want)
		}
	}
}

func TestEntriesFromRSSAndAtom(t *testing.T) {
	p := gofeed.NewParser()
	rss, err := p.ParseString(testRSS)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"•Eins: Erster & bester", "•Zwei: Zweiter Text"}
	if got := entriesFromFeed(rss); !slices.Equal(got, want) {
		t.Errorf("rss entries = %q, want %q", got, want)
	}
	atom, err := p.ParseString(testAtom)
	if err != nil {
		t.Fatal(err)
	}
	if got := entriesFromFeed(atom); !slices.Equal(got, []string{"•Atom-Titel: Kurzfassung"}) {
		t.Errorf("atom entries = %q", got)
	}
}

func TestFetchEntriesSkipsBrokenFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.xml":
			io.WriteString(w, testRSS)
		case "/garbage.xml":
			io.WriteString(w, "this is not xml")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	fc := fetch.New("")

	entries, stale, err := fetchEntries(context.Background(), fc, []string{srv.URL + "/missing.xml", srv.URL + "/garbage.xml", srv.URL + "/ok.xml"})
	if err != nil {
		t.Fatal(err)
	}
	if stale || len(entries) != 2 {
		t.Errorf("entries = %q, stale = %v", entries, stale)
	}
	if _, _, err := fetchEntries(context.Background(), fc, []string{srv.URL + "/missing.xml"}); err == nil {
		t.Error("all feeds failing not reported")
	}
}

func TestSelectLines(t *testing.T) {
	c := render.NewCanvas(460, 100, render.FontDefault, 14)
	short := []string{"•A: a", "•B: b", "•C: c", "•D: d"}
	if got := selectLines(c, short, 460, 3); !slices.Equal(got, short[:3]) {
		t.Errorf("short = %q", got)
	}
	long := strings.Repeat("wort ", 200)
	if got := selectLines(c, []string{long, "•B: b"}, 460, 3); len(got) != 0 {
		t.Errorf("entry longer than the box must stop selection, got %q", got)
	}
}

func TestArrange(t *testing.T) {
	common := mustCommon(t, `{"size":[480,120],"padding_x":10,"padding_y":10,"fontsize":14}`)
	in := []string{"1", "2", "3", "4", "5", "6"}

	m, err := New(common, json.RawMessage(`{"feed_urls":"http://x/feed","shuffle_feeds":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.(*Module).arrange(slices.Clone(in)); !slices.Equal(got, in) {
		t.Errorf("shuffle off changed order: %q", got)
	}

	m2, err := New(common, json.RawMessage(`{"feed_urls":"http://x/feed"}`)) // default: shuffle on
	if err != nil {
		t.Fatal(err)
	}
	mod := m2.(*Module)
	mod.rnd = rand.New(rand.NewPCG(1, 2))
	got := mod.arrange(slices.Clone(in))
	sorted := slices.Clone(got)
	slices.Sort(sorted)
	if !slices.Equal(sorted, in) {
		t.Errorf("shuffle lost or duplicated entries: %q", got)
	}
}

func TestGenerate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, testRSS)
	}))
	defer srv.Close()
	common := mustCommon(t, `{"size":[480,120],"padding_x":10,"padding_y":10,"fontsize":14,"language":"en"}`)
	m, err := New(common, json.RawMessage(`{"feed_urls":"`+srv.URL+`/a.xml, `+srv.URL+`/b.xml","shuffle_feeds":false}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: fetch.New("")}
	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if c.W != 460 || c.H != 100 {
		t.Fatalf("canvas = %dx%d", c.W, c.H)
	}
	black := false
	for y := 0; y < c.H && !black; y++ {
		for x := 0; x < c.W; x++ {
			if b, _ := c.InkAt(x, y); b {
				black = true
				break
			}
		}
	}
	if !black {
		t.Error("no feed text drawn")
	}
}

func TestNewRequiresURLs(t *testing.T) {
	common := mustCommon(t, `{"size":[480,120],"padding_x":10,"padding_y":10,"fontsize":14}`)
	if _, err := New(common, json.RawMessage(`{"feed_urls":""}`)); err == nil {
		t.Error("empty feed_urls accepted")
	}
}
