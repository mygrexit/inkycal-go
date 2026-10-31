package jokes

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

func TestSplit(t *testing.T) {
	cases := []struct{ in, setup, punch string }{
		{"What do you call a fish wearing a bowtie? Sofishticated.", "What do you call a fish wearing a bowtie?", "Sofishticated."},
		{"Why did the scarecrow win an award?\r\nBecause he was outstanding in his field.", "Why did the scarecrow win an award?", "Because he was outstanding in his field."},
		{"I'm reading a book about anti-gravity. It's impossible to put down.", "I'm reading a book about anti-gravity. It's impossible to put down.", ""},
		{"Did you hear?! The   bakery closed.", "Did you hear?!", "The bakery closed."},
		{"\"Is it?\" asked the dog.", "\"Is it?\"", "asked the dog."},
		{"Why?", "Why?", ""},
		{"Q?\n", "Q?", ""},
	}
	for _, c := range cases {
		s, p := split(c.in)
		if s != c.setup || p != c.punch {
			t.Errorf("split(%q) = %q, %q; want %q, %q", c.in, s, p, c.setup, c.punch)
		}
	}
}

func TestNewIgnoresUnknownKeys(t *testing.T) {
	common := modulestest.Common(t, `{"size":[480,160],"padding_x":10,"padding_y":10,"fontsize":14}`)
	if _, err := Def.New(common, json.RawMessage(`{"whatever":1}`)); err != nil {
		t.Fatal(err)
	}
	tiny := modulestest.Common(t, `{"size":[60,20],"padding_x":0,"padding_y":0}`)
	if _, err := Def.New(tiny, nil); err == nil {
		t.Error("tiny module accepted")
	}
}

func TestDrawFitsAndShrinks(t *testing.T) {
	for _, size := range [][2]int{{904, 140}, {904, 600}, {300, 60}} {
		c := render.NewCanvas(size[0], size[1], render.FontRegular, 20)
		draw(c, "What do you call a fish wearing a bowtie?", "Sofishticated.", 20)
		if b, _ := modulestest.InkIn(c, 0, 0, c.W, c.H); !b {
			t.Errorf("%v: no text", size)
		}
		if _, col := modulestest.InkIn(c, 0, 0, c.W, c.H); !col {
			t.Errorf("%v: punchline dash missing", size)
		}
		// nothing in the outermost rows: the block is not clipped
		if b, _ := modulestest.InkIn(c, 0, 0, c.W, 1); b {
			t.Errorf("%v: ink in first row", size)
		}
	}
	// A very long joke in a small box is cut with an ellipsis, never overflows.
	c := render.NewCanvas(300, 45, render.FontRegular, 20)
	long := "This is a very long joke that goes on and on and on without any end in sight and never gets to the point"
	l := typeset(c, long, "", 16)
	l.cut(c, c.H)
	if !l.truncated || l.height > c.H {
		t.Errorf("long joke not cut: %+v", l)
	}
}

func TestGenerate(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		fmt.Fprint(w, `{"id":"R7UfaahVfFd","joke":"My dog used to chase people on a bike a lot. It got so bad I had to take his bike away.","status":200}`)
	}))
	defer srv.Close()
	common := modulestest.Common(t, `{"size":[924,180],"padding_x":10,"padding_y":20,"fontsize":20,"language":"en"}`)
	m, err := Def.New(common, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.(*Module).url = srv.URL
	env := &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: fetch.New("")}
	c, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q", gotAccept)
	}
	if c.W != 904 || c.H != 140 {
		t.Fatalf("canvas %dx%d", c.W, c.H)
	}
	if b, _ := modulestest.InkIn(c, 0, 0, c.W, c.H); !b {
		t.Error("nothing drawn")
	}
}

func TestGenerateEmptyJoke(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status":200}`)
	}))
	defer srv.Close()
	common := modulestest.Common(t, `{"size":[480,160],"padding_x":10,"padding_y":10,"fontsize":14}`)
	m, _ := Def.New(common, nil)
	m.(*Module).url = srv.URL
	if _, err := m.Generate(context.Background(), &modules.Env{Fetch: fetch.New("")}); err == nil {
		t.Error("empty joke accepted")
	}
}

func TestPreview(t *testing.T) {
	if os.Getenv("INKYCAL_PREVIEW_DIR") == "" {
		t.Skip("INKYCAL_PREVIEW_DIR not set")
	}
	samples := []struct{ name, joke string }{
		{"qa", "What do you call a fish wearing a bowtie? Sofishticated."},
		{"plain", "I'm reading a book about anti-gravity. It's impossible to put down."},
		{"long", "My wife told me to stop impersonating a flamingo. I had to put my foot down. She said if I keep doing it she will leave me, and honestly that would be a real loss for the household's balance."},
	}
	for _, size := range [][2]int{{904, 140}, {904, 600}} {
		for _, s := range samples {
			c := render.NewCanvas(size[0], size[1], render.FontRegular, 22)
			setup, punch := split(s.joke)
			draw(c, setup, punch, 22)
			savePreview(t, fmt.Sprintf("jokes-%s-%dx%d", s.name, size[0], size[1]), c)
		}
	}
}
