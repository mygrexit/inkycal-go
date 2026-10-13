// Package textfile ports inkycal_textfile_to_display.py: the content of a plain
// text file, read from a local path or a URL, set as wrapped paragraphs and lists.
package textfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"inkycal/internal/config"
	"inkycal/internal/i18n"
	"inkycal/internal/modules"
	"inkycal/internal/render"
)

// maxFile caps what is read from a local file; far more than any panel shows.
const maxFile = 1 << 20

type Module struct {
	common config.Common
	path   string
	remote bool
}

type rawConfig struct {
	Filepath string `json:"filepath"`
}

func New(c config.Common, raw json.RawMessage) (modules.Module, error) {
	var rc rawConfig
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &rc); err != nil {
			return nil, fmt.Errorf("textfile: config: %w", err)
		}
	}
	p := strings.TrimSpace(rc.Filepath)
	if p == "" {
		return nil, errors.New("textfile: filepath is missing")
	}
	if w, h := c.InnerSize(); w < 100 || h < 20 {
		return nil, fmt.Errorf("textfile: inner size %dx%d too small (min 100x20)", w, h)
	}
	lower := strings.ToLower(p)
	return &Module{
		common: c,
		path:   p,
		// upstream only treats https:// as remote; plain http is accepted too
		remote: strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://"),
	}, nil
}

func (m *Module) read(ctx context.Context, env *modules.Env) (string, error) {
	var body []byte
	if m.remote {
		res, err := env.Fetch.Get(ctx, m.path, nil)
		if err != nil {
			return "", fmt.Errorf("textfile: %w", err)
		}
		env.Stale = env.Stale || res.Stale
		body = res.Body
	} else {
		f, err := os.Open(m.path)
		if err != nil {
			return "", fmt.Errorf("textfile: %w", err)
		}
		defer f.Close()
		if body, err = io.ReadAll(io.LimitReader(f, maxFile)); err != nil {
			return "", fmt.Errorf("textfile: %w", err)
		}
	}
	if !utf8.Valid(body) {
		body = []byte(strings.ToValidUTF8(string(body), "�"))
	}
	return string(body), nil
}

func (m *Module) Generate(ctx context.Context, env *modules.Env) (*render.Canvas, error) {
	text, err := m.read(ctx, env)
	if err != nil {
		return nil, err
	}
	w, h := m.common.InnerSize()
	c := render.NewCanvas(w, h, render.FontRegular, int(m.common.FontSize))
	draw(c, parse(text), int(m.common.FontSize), m.common.Language)
	return c, nil
}

// para is one source line: a paragraph, list item or heading; blank lines become gaps.
type para struct {
	Level   int    // list nesting from leading whitespace
	Marker  string // "•", "–", "3.", or "" for plain text
	Text    string
	Heading bool // a Markdown-style "# " line
	Blank   bool
}

var listRe = regexp.MustCompile(`^([-*+•–·]|\d{1,3}[.)])\s+(.*)$`)

// parse splits text into paragraphs. Leading whitespace (a tab counts as four
// spaces, two spaces make one level) nests list items.
func parse(text string) []para {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimPrefix(text, "\ufeff")
	var out []para
	for _, line := range strings.Split(text, "\n") {
		indent := 0
		for _, r := range line {
			if r == ' ' {
				indent++
			} else if r == '\t' {
				indent += 4
			} else {
				break
			}
		}
		line = strings.TrimSpace(line)
		if line == "" {
			if len(out) > 0 && !out[len(out)-1].Blank {
				out = append(out, para{Blank: true})
			}
			continue
		}
		p := para{Text: strings.Join(strings.Fields(line), " ")}
		if h := strings.TrimLeft(line, "#"); h != line && strings.HasPrefix(h, " ") {
			p.Heading, p.Text = true, strings.TrimSpace(h)
		} else if mm := listRe.FindStringSubmatch(line); mm != nil {
			p.Level = 1 + indent/2
			p.Marker = mm[1]
			switch p.Marker {
			case "-", "*", "+", "·":
				p.Marker = "•"
			}
			p.Text = strings.Join(strings.Fields(mm[2]), " ")
		} else if indent >= 2 {
			p.Level = indent / 2 // indented continuation text
		}
		out = append(out, p)
	}
	for len(out) > 0 && out[len(out)-1].Blank {
		out = out[:len(out)-1]
	}
	return out
}

// line is one typeset line.
type line struct {
	X, Baseline int
	Marker      string // drawn at MarkerX, only on an item's first line
	MarkerX     int
	Text        string
	Style       render.Style
	para        int // index of the source paragraph
}

type metrics struct {
	body, head, small render.Style
	step, gap, level  int
	capH, desc        int
}

func newMetrics(c *render.Canvas, size int) metrics {
	m := metrics{
		body:  render.Style{Font: render.FontRegular, Size: size},
		head:  render.Style{Font: render.FontDefault, Size: size},
		small: render.Style{Font: render.FontRegular, Size: size * 5 / 6},
		step:  size * 7 / 5,
		gap:   size * 3 / 5,
		level: size * 6 / 5,
	}
	m.capH = c.CapHeight(m.body)
	_, m.desc = c.Metrics(m.body)
	return m
}

// layout typesets paragraphs from the top until the next line would not fit
// above limit (a baseline + descender). It returns the lines and the index of
// the first paragraph not started; cut reports a paragraph broken off mid-way.
func layout(c *render.Canvas, ps []para, m metrics, limit int) (lines []line, next int, cut bool) {
	y := m.capH
	for i, p := range ps {
		if p.Blank {
			y += m.gap
			continue
		}
		st := m.body
		if p.Heading {
			st = m.head
			if i > 0 && !ps[i-1].Blank {
				y += m.gap
			}
		}
		x := p.Level * m.level
		markerX := x
		if p.Marker != "" {
			markerX = (p.Level - 1) * m.level
			x = markerX + max(m.level, c.Advance(p.Marker, st)+c.Advance(" ", st)*2)
		}
		for j, t := range c.WrapStyle(p.Text, st, c.W-x, c.W-x) {
			if y+m.desc > limit {
				if j > 0 {
					lines[len(lines)-1].Text = c.Fit(lines[len(lines)-1].Text+" …", st, c.W-x)
					return lines, i + 1, true
				}
				return lines, i, false
			}
			l := line{X: x, Baseline: y, Text: t, Style: st, para: i}
			if j == 0 {
				l.Marker, l.MarkerX = p.Marker, markerX
			}
			lines = append(lines, l)
			y += m.step
		}
		if p.Heading {
			y += m.gap / 2
		}
	}
	return lines, len(ps), false
}

// count is the number of non-blank paragraphs in ps.
func count(ps []para) int {
	n := 0
	for _, p := range ps {
		if !p.Blank {
			n++
		}
	}
	return n
}

// draw sets the text top-down: body lines in the regular face, headings in
// medium, list markers hanging left of their text so wrapped lines align with
// the first. What does not fit is counted in a small last line.
func draw(c *render.Canvas, ps []para, size int, lang string) {
	m := newMetrics(c, size)
	lines, next, _ := layout(c, ps, m, c.H)
	rest := count(ps[next:])
	if rest > 0 {
		// leave room for the "+ n more" line and lay out again
		lines, next, _ = layout(c, ps, m, c.H-m.step)
		rest = count(ps[next:])
	}
	for _, l := range lines {
		if l.Marker != "" {
			c.Text(l.MarkerX, l.Baseline, l.Marker, l.Style, render.InkBlack)
		}
		c.Text(l.X, l.Baseline, l.Text, l.Style, render.InkBlack)
	}
	if rest == 0 {
		return
	}
	more := i18n.T(lang, "more_lines", rest)
	if rest == 1 {
		more = i18n.T(lang, "more_line")
	}
	y := m.capH
	x := 0
	if len(lines) > 0 {
		last := lines[len(lines)-1]
		y, x = last.Baseline+m.step, last.X
	}
	c.Text(x, min(y, c.H-m.desc), more, m.small, render.InkBlack)
}
