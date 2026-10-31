// Package jokes ports inkycal_jokes.py: a random dad joke from icanhazdadjoke.com.
package jokes

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"inkycal/internal/config"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

// APIURL is the joke endpoint; it answers with JSON when asked to.
const APIURL = "https://icanhazdadjoke.com/"

type Module struct {
	common config.Common
	url    string
}

// Def takes no module keys: upstream Jokes has none besides the shared ones.
var Def = modules.Define(Meta, func(c config.Common, _ struct{}) (modules.Module, error) {
	return &Module{common: c, url: APIURL}, nil
})

type apiJoke struct {
	ID     string `json:"id"`
	Joke   string `json:"joke"`
	Status int    `json:"status"`
}

func (m *Module) fetch(ctx context.Context, env *modules.Env) (string, error) {
	h := http.Header{}
	h.Set("Accept", "application/json")
	// The API asks clients to identify themselves.
	h.Set("User-Agent", "inkycal-go (https://github.com/aceinnolab/Inkycal)")
	var j apiJoke
	stale, err := env.Fetch.GetJSON(ctx, m.url, h, &j)
	if err != nil {
		return "", err
	}
	env.Stale = env.Stale || stale
	joke := strings.TrimSpace(j.Joke)
	if joke == "" {
		return "", errors.New("API returned no joke")
	}
	return joke, nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	joke, err := m.fetch(ctx, env)
	if err != nil {
		return nil, err
	}
	w, h := m.common.InnerSize()
	c := env.Canvas(w, h, render.FontRegular, int(m.common.FontSize))
	setup, punch := split(joke)
	draw(c, setup, punch, int(m.common.FontSize))
	return c, nil
}

// split separates a question/answer joke into setup and punchline: at the
// first line break, else after the first "?" that is followed by more text.
// Jokes without such a split come back whole as setup.
func split(joke string) (setup, punch string) {
	joke = strings.ReplaceAll(joke, "\r", "")
	if i := strings.IndexByte(joke, '\n'); i >= 0 {
		s, p := clean(joke[:i]), clean(joke[i+1:])
		if s != "" && p != "" {
			return s, p
		}
	}
	for i := 0; i < len(joke); i++ {
		if joke[i] != '?' {
			continue
		}
		j := i + 1
		for j < len(joke) && (joke[j] == '?' || joke[j] == '!' || joke[j] == '"') {
			j++ // keep "?!" and closing quotes with the question
		}
		if s, p := clean(joke[:j]), clean(joke[j:]); s != "" && p != "" && j < len(joke) && joke[j] == ' ' {
			return s, p
		}
	}
	return clean(joke), ""
}

// clean folds all whitespace runs into single spaces.
func clean(s string) string { return strings.Join(strings.Fields(s), " ") }

// layout is the joke typeset at one size.
type layout struct {
	st, pst    render.Style
	setup      []string
	punch      []string
	step, gap  int // baseline to baseline; extra space before the punchline
	indent     int // hanging indent for the red dash
	height     int // cap top of the first line to the descender of the last
	capH, desc int
	truncated  bool
}

func typeset(c *render.Canvas, setup, punch string, size int) layout {
	l := layout{
		st:  render.Style{Font: render.FontRegular, Size: size},
		pst: render.Style{Font: render.FontDefault, Size: size},
	}
	l.step = size * 4 / 3
	l.capH = c.CapHeight(l.st)
	_, l.desc = c.Metrics(l.st)
	width := c.W
	if punch != "" {
		l.indent = c.Advance("– ", l.pst)
		l.gap = size / 2
		width -= l.indent
	}
	l.setup = c.WrapStyle(setup, l.st, width, width)
	l.punch = c.WrapStyle(punch, l.pst, width, width)
	n := len(l.setup) + len(l.punch)
	l.height = l.capH + (n-1)*l.step + l.desc
	if len(l.punch) > 0 {
		l.height += l.gap
	}
	return l
}

// cut drops lines from the end until the layout fits h, marking the cut with "…".
func (l *layout) cut(c *render.Canvas, h int) {
	for l.height > h && len(l.setup)+len(l.punch) > 1 {
		l.truncated = true
		if len(l.punch) > 0 {
			l.punch = l.punch[:len(l.punch)-1]
			if len(l.punch) == 0 {
				l.height -= l.gap
			}
		} else {
			l.setup = l.setup[:len(l.setup)-1]
		}
		l.height -= l.step
	}
	if !l.truncated {
		return
	}
	width := c.W - l.indent
	if n := len(l.punch); n > 0 {
		l.punch[n-1] = c.Fit(l.punch[n-1]+" …", l.pst, width)
	} else if n := len(l.setup); n > 0 {
		l.setup[n-1] = c.Fit(l.setup[n-1]+" …", l.st, width)
	}
}

// draw sets the joke as large as fits (from 1.6× down to the body size), the
// setup in the regular face, the punchline in medium below it after a pause,
// marked by a red dash hanging in the margin. The block is centred vertically.
//
//	  What do you call a fish
//	  wearing a bowtie?
//
//	– Sofishticated.
func draw(c *render.Canvas, setup, punch string, body int) {
	var l layout
	for size := body * 8 / 5; ; size-- {
		l = typeset(c, setup, punch, size)
		if l.height <= c.H || size <= body*4/5 || size <= 8 {
			break
		}
	}
	l.cut(c, c.H)
	y := (c.H-l.height)/2 + l.capH
	for _, line := range l.setup {
		c.Text(l.indent, y, line, l.st, render.InkBlack)
		y += l.step
	}
	if len(l.punch) == 0 {
		return
	}
	y += l.gap
	c.Text(0, y, "–", l.pst, render.InkColour)
	for _, line := range l.punch {
		c.Text(l.indent, y, line, l.pst, render.InkBlack)
		y += l.step
	}
}
