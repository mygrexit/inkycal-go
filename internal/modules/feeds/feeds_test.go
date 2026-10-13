package feeds

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	cases := []struct {
		title, desc string
		want        item
	}{
		{"Eins", "<p>Erster &amp; bester</p>", item{"Eins", "Erster & bester", "Feed"}},
		{"Zwei", "Zweiter Text\nmit Umbruch", item{"Zwei", "Zweiter Text mit Umbruch", "Feed"}},
		{" Drei ", "&lt;b&gt; bleibt Text", item{"Drei", "<b> bleibt Text", "Feed"}},
	}
	for _, c := range cases {
		if got := entry(c.title, c.desc, " Feed "); got != c.want {
			t.Errorf("entry(%q, %q) = %+v, want %+v", c.title, c.desc, got, c.want)
		}
	}
}

func TestEntriesFromRSSAndAtom(t *testing.T) {
	p := gofeed.NewParser()
	rss, err := p.ParseString(testRSS)
	if err != nil {
		t.Fatal(err)
	}
	want := []item{{"Eins", "Erster & bester", "Test"}, {"Zwei", "Zweiter Text mit Umbruch", "Test"}}
	if got := entriesFromFeed(rss); !slices.Equal(got, want) {
		t.Errorf("rss entries = %+v, want %+v", got, want)
	}
	atom, err := p.ParseString(testAtom)
	if err != nil {
		t.Fatal(err)
	}
	if got := entriesFromFeed(atom); !slices.Equal(got, []item{{"Atom-Titel", "Kurzfassung", "A"}}) {
		t.Errorf("atom entries = %+v", got)
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
		t.Errorf("entries = %+v, stale = %v", entries, stale)
	}
	if _, _, err := fetchEntries(context.Background(), fc, []string{srv.URL + "/missing.xml"}); err == nil {
		t.Error("all feeds failing not reported")
	}
}

func TestLayout(t *testing.T) {
	c := render.NewCanvas(460, 100, render.FontDefault, 14)
	m := newMetrics(c, 14)
	short := []item{{"A", "a", "X"}, {"B", "b", "X"}, {"C", "c", "X"}, {"D", "d", "X"}}
	blocks, used := layout(c, short, m, c.H)
	if used != len(blocks) || used == 0 || used == len(short) {
		t.Errorf("blocks %d, used %d", len(blocks), used)
	}
	for _, b := range blocks {
		if b.Source != "" {
			t.Errorf("source shown for a single feed: %+v", b)
		}
		if bottom := b.Y + (len(b.Head)+len(b.Summary)-1)*m.step + m.desc; bottom > c.H {
			t.Errorf("block %+v below the canvas", b)
		}
	}
	long := strings.Repeat("wort ", 200)
	blocks, _ = layout(c, []item{{long, long, "X"}, {"B", "b", "Y"}}, m, c.H)
	if len(blocks) == 0 || len(blocks[0].Head) > 2 || len(blocks[0].Summary) > m.maxSummary {
		t.Fatalf("long item not clipped: %+v", blocks)
	}
	if !strings.HasSuffix(blocks[0].Head[1], "…") || blocks[0].Source == "" {
		t.Errorf("clip marker or source missing: %+v", blocks[0])
	}
	for _, l := range blocks[0].Head {
		if c.Advance(l, m.head) > c.W {
			t.Errorf("headline overflows: %q", l)
		}
	}
	for _, l := range blocks[0].Summary {
		if c.Advance(l, m.sum) > c.W {
			t.Errorf("line overflows: %q", l)
		}
	}
}

func TestArrange(t *testing.T) {
	common := mustCommon(t, `{"size":[480,120],"padding_x":10,"padding_y":10,"fontsize":14}`)
	in := []item{{Title: "1"}, {Title: "2"}, {Title: "3"}, {Title: "4"}, {Title: "5"}, {Title: "6"}}

	m, err := New(common, json.RawMessage(`{"feed_urls":"http://x/feed","shuffle_feeds":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := m.(*Module).arrange(slices.Clone(in)); !slices.Equal(got, in) {
		t.Errorf("shuffle off changed order: %+v", got)
	}

	m2, err := New(common, json.RawMessage(`{"feed_urls":"http://x/feed"}`)) // default: shuffle on
	if err != nil {
		t.Fatal(err)
	}
	mod := m2.(*Module)
	mod.rnd = rand.New(rand.NewPCG(1, 2))
	got := mod.arrange(slices.Clone(in))
	sorted := slices.Clone(got)
	slices.SortFunc(sorted, func(a, b item) int { return strings.Compare(a.Title, b.Title) })
	if !slices.Equal(sorted, in) {
		t.Errorf("shuffle lost or duplicated entries: %+v", got)
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

// savePreview writes c as PNG (black, red, white) into $INKYCAL_PREVIEW_DIR.
func savePreview(t *testing.T, name string, c *render.Canvas) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			b, col := c.InkAt(x, y)
			switch {
			case b:
				img.Set(x, y, color.Black)
			case col:
				img.Set(x, y, color.RGBA{R: 230, A: 255})
			default:
				img.Set(x, y, color.White)
			}
		}
	}
	f, err := os.Create(filepath.Join(os.Getenv("INKYCAL_PREVIEW_DIR"), name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestPreview(t *testing.T) {
	if os.Getenv("INKYCAL_PREVIEW_DIR") == "" {
		t.Skip("INKYCAL_PREVIEW_DIR not set")
	}
	items := []item{
		{"Bahnstreik ab Donnerstag: Was Pendler jetzt wissen müssen", "Die GDL hat einen 48-stündigen Streik im Personen- und Güterverkehr angekündigt. Die Bahn will einen Notfahrplan anbieten, rechnet aber mit massiven Ausfällen im Fern- und Regionalverkehr.", "tagesschau.de"},
		{"Herbstferien: Stau-Prognose für das Wochenende", "Der ADAC erwartet volle Autobahnen vor allem in Richtung Süden.", "ADAC"},
		{"Neue Kita in der Südstadt eröffnet", "Nach zwei Jahren Bauzeit bietet die Einrichtung Platz für 90 Kinder in sechs Gruppen.", "Stadtanzeiger"},
		{"Wetter: Erster Nachtfrost in den Höhenlagen", "In der Nacht zum Freitag kann es in den Mittelgebirgen bis minus zwei Grad kalt werden.", "tagesschau.de"},
		{"Fußball: Heimsieg im Derby", "Mit 2:1 setzte sich die Heimmannschaft vor ausverkauftem Haus durch.", "Stadtanzeiger"},
		{"Bibliothek verlängert Öffnungszeiten", "Ab November ist die Stadtbibliothek auch samstags bis 18 Uhr geöffnet.", "Stadtanzeiger"},
	}
	for _, size := range [][2]int{{904, 140}, {904, 600}} {
		c := render.NewCanvas(size[0], size[1], render.FontDefault, 22)
		draw(c, slices.Clone(items), 22, "de")
		savePreview(t, fmt.Sprintf("feeds-%dx%d", size[0], size[1]), c)
	}
	c := render.NewCanvas(440, 300, render.FontDefault, 20)
	draw(c, items[:3], 20, "en")
	savePreview(t, "feeds-440x300", c)
}

func TestEntryTagSpacing(t *testing.T) {
	for in, want := range map[string]string{
		"<p>Erster Satz.</p><p>Zweiter Satz.</p>": "Erster Satz. Zweiter Satz.",
		"Zeile1<br/>Zeile2":                       "Zeile1 Zeile2",
		"Die <b>B</b>undesregierung":              "Die Bundesregierung",
		"CO<sub>2</sub>-Preis":                    "CO2-Preis",
	} {
		if got := entry("t", in, "").Summary; got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}
