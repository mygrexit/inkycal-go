package textfile

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

// savePreview writes c as PNG (black, red, white) into $INKYCAL_PREVIEW_DIR.
func savePreview(t *testing.T, name string, c *render.Canvas) {
	t.Helper()
	dir := os.Getenv("INKYCAL_PREVIEW_DIR")
	if dir == "" {
		return
	}
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
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

const sample = "# Wochenplan\n" +
	"Diese Woche ist Laternenbasteln in der Kita, bitte eine leere Konservendose und Transparentpapier mitgeben.\n" +
	"\n" +
	"- Montag: Turnbeutel einpacken\n" +
	"- Mittwoch: Elternabend um 19:30 Uhr in der Gruppe Sonnenblumen, Anmeldung bis Dienstag\n" +
	"  - Kuchen mitbringen\n" +
	"- Freitag: Ausflug zum Bauernhof\n" +
	"\n" +
	"1. Müll rausbringen\n" +
	"2. Pflanzen gießen\n" +
	"\n" +
	"Danke!\n"

func TestNewConfig(t *testing.T) {
	common := modulestest.Common(t, `{"size":[480,200],"padding_x":10,"padding_y":10,"fontsize":14}`)
	for _, tc := range []struct {
		raw    string
		remote bool
		ok     bool
	}{
		{`{"filepath":"/home/pi/notes.txt"}`, false, true},
		{`{"filepath":"https://example.org/notes.txt"}`, true, true},
		{`{"filepath":"HTTP://example.org/notes.txt"}`, true, true},
		{`{"filepath":""}`, false, false},
		{`{}`, false, false},
	} {
		m, err := New(common, json.RawMessage(tc.raw))
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v", tc.raw, err)
			continue
		}
		if err == nil && m.(*Module).remote != tc.remote {
			t.Errorf("%s: remote = %v", tc.raw, m.(*Module).remote)
		}
	}
}

func TestParse(t *testing.T) {
	ps := parse("\ufeff# Titel\r\nText  mit   Lücken\r\n\r\n\r\n- eins\n\t* zwei\n  3) drei\n10. zehn\n#hashtag\n   eingerückt\n\n")
	want := []para{
		{Heading: true, Text: "Titel"},
		{Text: "Text mit Lücken"},
		{Blank: true},
		{Level: 1, Marker: "•", Text: "eins"},
		{Level: 3, Marker: "•", Text: "zwei"},
		{Level: 2, Marker: "3)", Text: "drei"},
		{Level: 1, Marker: "10.", Text: "zehn"},
		{Text: "#hashtag"},
		{Level: 1, Text: "eingerückt"},
	}
	if len(ps) != len(want) {
		t.Fatalf("got %d paras: %+v", len(ps), ps)
	}
	for i := range want {
		if ps[i] != want[i] {
			t.Errorf("para %d = %+v, want %+v", i, ps[i], want[i])
		}
	}
}

func TestLayoutHangingIndent(t *testing.T) {
	c := render.NewCanvas(300, 600, render.FontRegular, 20)
	m := newMetrics(c, 20)
	ps := parse("- ein ziemlich langer Listenpunkt, der sicher umbricht und weitergeht")
	lines, next, cut := layout(c, ps, m, c.H)
	if next != 1 || cut || len(lines) < 2 {
		t.Fatalf("lines %+v next %d cut %v", lines, next, cut)
	}
	if lines[0].Marker != "•" || lines[1].Marker != "" || lines[0].X != lines[1].X || lines[0].MarkerX >= lines[0].X {
		t.Errorf("no hanging indent: %+v", lines)
	}
	for _, l := range lines {
		if l.X+c.Advance(l.Text, l.Style) > c.W {
			t.Errorf("line overflows: %q", l.Text)
		}
	}
}

func TestDrawCountsRest(t *testing.T) {
	c := render.NewCanvas(904, 140, render.FontRegular, 20)
	m := newMetrics(c, 20)
	ps := parse(sample)
	lines, next, _ := layout(c, ps, m, c.H-m.step)
	if next >= len(ps) {
		t.Fatal("sample fits a 140 px band; test needs overflow")
	}
	for _, l := range lines {
		if l.Baseline+m.desc > c.H-m.step {
			t.Errorf("line %q below limit", l.Text)
		}
	}
	draw(c, ps, 20, "de")
	if b, _ := modulestest.InkIn(c, 0, 0, c.W, c.H); !b {
		t.Error("nothing drawn")
	}
}

func TestGenerateLocalAndRemote(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/notes.txt" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, sample)
	}))
	defer srv.Close()
	common := modulestest.Common(t, `{"size":[924,640],"padding_x":10,"padding_y":20,"fontsize":20,"language":"de"}`)
	for _, src := range []string{path, srv.URL + "/notes.txt"} {
		m, err := New(common, json.RawMessage(fmt.Sprintf(`{"filepath":%q}`, src)))
		if err != nil {
			t.Fatal(err)
		}
		env := &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: fetch.New("")}
		c, err := m.Generate(context.Background(), env)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if c.W != 904 || c.H != 600 {
			t.Fatalf("canvas %dx%d", c.W, c.H)
		}
		if b, _ := modulestest.InkIn(c, 0, 0, c.W, c.H); !b {
			t.Errorf("%s: nothing drawn", src)
		}
	}
	for _, src := range []string{filepath.Join(dir, "missing.txt"), srv.URL + "/missing.txt"} {
		m, _ := New(common, json.RawMessage(fmt.Sprintf(`{"filepath":%q}`, src)))
		if _, err := m.Generate(context.Background(), &modules.Env{Fetch: fetch.New("")}); err == nil {
			t.Errorf("%s: missing file not reported", src)
		}
	}
}

func TestPreview(t *testing.T) {
	if os.Getenv("INKYCAL_PREVIEW_DIR") == "" {
		t.Skip("INKYCAL_PREVIEW_DIR not set")
	}
	long := sample + strings.Repeat("Noch eine Zeile mit etwas Text.\n", 20)
	for _, size := range [][2]int{{904, 140}, {904, 600}} {
		c := render.NewCanvas(size[0], size[1], render.FontRegular, 22)
		draw(c, parse(long), 22, "de")
		savePreview(t, fmt.Sprintf("textfile-%dx%d", size[0], size[1]), c)
	}
}
