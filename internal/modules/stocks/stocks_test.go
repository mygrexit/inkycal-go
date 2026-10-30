package stocks

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
	"reflect"
	"strings"
	"testing"
	"time"

	"inkycal/internal/fetch"
	"inkycal/internal/modules"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestNewTickers(t *testing.T) {
	c := modulestest.Common(t, `{"size":[904,140],"fontsize":22}`)
	for raw, want := range map[string][]string{
		`{"tickers":"TSLA, U, NVDA, EURUSD=X"}`: {"TSLA", "U", "NVDA", "EURUSD=X"},
		`{"tickers":["AAPL","^GDAXI"]}`:         {"AAPL", "^GDAXI"},
	} {
		m, err := New(c, json.RawMessage(raw))
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if got := m.(*Module).tickers; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %q want %q", raw, got, want)
		}
	}
	for _, raw := range []string{`{}`, `{"tickers":""}`, `{"tickers":null}`, `{"tickers":5}`} {
		if _, err := New(c, json.RawMessage(raw)); err == nil {
			t.Errorf("%s: expected an error", raw)
		}
	}
	tiny := modulestest.Common(t, `{"size":[100,100]}`)
	if _, err := New(tiny, json.RawMessage(`{"tickers":"AAPL"}`)); err == nil {
		t.Error("expected size error")
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		v    float64
		prec int
		lang string
		want string
	}{
		{1234.567, 2, "de", "1.234,57"},
		{1234.567, 2, "en", "1,234.57"},
		{25426.61, 2, "de", "25.426,61"},
		{0.98765, 4, "en", "0.9877"},
		{-3.5, 1, "en", "–3.5"},
		{-0.001, 2, "en", "0.00"},
		{999, 0, "de", "999"},
		{1000000, 0, "de", "1.000.000"},
	}
	for _, c := range cases {
		if got := formatNum(c.v, c.prec, c.lang); got != c.want {
			t.Errorf("formatNum(%v,%d,%s) = %q want %q", c.v, c.prec, c.lang, got, c.want)
		}
	}
	if got := changeLabel(1.286, "de"); got != "+1,29 %" {
		t.Errorf("de change %q", got)
	}
	if got := changeLabel(-0.4, "en"); got != "–0.40%" {
		t.Errorf("en change %q", got)
	}
	usd := quote{Currency: "USD", Precision: 2, Closes: []float64{340.97}}
	if got := priceLabel(usd, "en"); got != "$340.97" {
		t.Errorf("en price %q", got)
	}
	if got := priceLabel(usd, "de"); got != "340,97 $" {
		t.Errorf("de price %q", got)
	}
	chf := quote{Currency: "CHF", Precision: 2, Closes: []float64{12.4}}
	if got := priceLabel(chf, "en"); got != "12.40 CHF" {
		t.Errorf("chf price %q", got)
	}
	idx := quote{Currency: "EUR", Index: true, Precision: 2, Closes: []float64{25426.61}}
	if got := priceLabel(idx, "de"); got != "25.426,61" {
		t.Errorf("index price %q", got)
	}
}

func TestLabelGlyphs(t *testing.T) {
	for _, r := range append(sampleRows("de"), sampleRows("en")...) {
		for _, s := range []string{r.Price, r.Change, r.Name, r.Symbol} {
			if !render.HasGlyphs(render.FontRegular, s) || !render.HasGlyphs(render.FontDefault, s) {
				t.Errorf("%q: missing glyph", s)
			}
		}
	}
}

func TestMakeRowRedOnlyForNegativeDay(t *testing.T) {
	up := makeRow("A", result{Quote: quote{Name: "A", Precision: 2, Closes: []float64{100, 101}}}, "en")
	down := makeRow("B", result{Quote: quote{Name: "B", Precision: 2, Closes: []float64{100, 99}}}, "en")
	flat := makeRow("C", result{Quote: quote{Name: "C", Precision: 2, Closes: []float64{100, 99.9999}}}, "en")
	if up.Down || !down.Down || flat.Down {
		t.Errorf("down flags: up %v down %v flat %v", up.Down, down.Down, flat.Down)
	}
	if up.Change != "+1.00%" || down.Change != "–1.00%" {
		t.Errorf("changes %q %q", up.Change, down.Change)
	}
	failed := makeRow("XX", result{Err: fmt.Errorf("404")}, "de")
	if failed.Price != "" || failed.Name != "keine Daten" {
		t.Errorf("failed row %+v", failed)
	}
}

const aaplJSON = `{"chart":{"result":[{"meta":{"currency":"USD","symbol":"AAPL","instrumentType":"EQUITY","regularMarketPrice":341.5,"shortName":"Apple Inc.","longName":"Apple Inc.","priceHint":2},
"timestamp":[1,2,3,4],"indicators":{"quote":[{"close":[330.0,null,336.64,340.97]}]}}],"error":null}}`

func TestParseChart(t *testing.T) {
	q, err := parseChart("AAPL", []byte(aaplJSON))
	if err != nil {
		t.Fatal(err)
	}
	if q.Name != "Apple Inc." || q.Currency != "USD" || q.Precision != 2 || q.Index {
		t.Errorf("meta %+v", q)
	}
	if !reflect.DeepEqual(q.Closes, []float64{330, 336.64, 341.5}) {
		t.Errorf("closes %v (null skipped, live price last)", q.Closes)
	}
	if q.Previous() != 336.64 {
		t.Errorf("previous %v", q.Previous())
	}
	if _, err := parseChart("X", []byte(`{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found"}}}`)); err == nil || !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("error not passed on: %v", err)
	}
	if _, err := parseChart("X", []byte(`{"chart":{"result":[{"meta":{},"indicators":{"quote":[{"close":[null]}]}}]}}`)); err == nil {
		t.Error("expected no-prices error")
	}
	noName, err := parseChart("EURUSD=X", []byte(`{"chart":{"result":[{"meta":{"currency":"USD","priceHint":4},"indicators":{"quote":[{"close":[1.1,1.2]}]}}]}}`))
	if err != nil || noName.Name != "EURUSD=X" || noName.Precision != 4 {
		t.Errorf("fallbacks: %+v %v", noName, err)
	}
}

func TestSparkPoints(t *testing.T) {
	pts := sparkPoints([]float64{1, 3, 2}, 100, 20)
	want := [][2]int{{0, 20}, {50, 0}, {100, 10}}
	if !reflect.DeepEqual(pts, want) {
		t.Errorf("got %v want %v", pts, want)
	}
	flat := sparkPoints([]float64{5, 5}, 10, 20)
	if flat[0][1] != 10 || flat[1][1] != 10 {
		t.Errorf("flat %v", flat)
	}
}

func TestColumnsDegrade(t *testing.T) {
	c := render.NewCanvas(904, 140, render.FontDefault, 22)
	rows := sampleRows("en")
	st := render.Style{Font: render.FontDefault, Size: 22}
	wide := columns(c, rows, 904, 22, st, st, st)
	if wide.nameW == 0 || wide.sparkW == 0 {
		t.Errorf("wide layout dropped something: %+v", wide)
	}
	narrow := columns(c, rows, 360, 22, st, st, st)
	if narrow.nameW != 0 {
		t.Errorf("narrow layout keeps names: %+v", narrow)
	}
	tiny := columns(c, rows, 250, 22, st, st, st)
	if tiny.nameW != 0 || tiny.sparkW != 0 {
		t.Errorf("tiny layout: %+v", tiny)
	}
}

func TestDrawRowsOverflowAndRed(t *testing.T) {
	rows := sampleRows("en")
	c := render.NewCanvas(904, 140, render.FontDefault, 22)
	drawRows(c, rows, 22, "en") // 140/35 = 4 slots: 3 rows + "+ n more"
	rowH := 22 * 8 / 5
	for i := range 3 {
		_, red := modulestest.InkIn(c, 0, i*rowH, c.W, (i+1)*rowH)
		if red != rows[i].Down {
			t.Errorf("row %d: red %v, down %v", i, red, rows[i].Down)
		}
	}
	if b, _ := modulestest.InkIn(c, 0, 3*rowH, 200, 4*rowH); !b {
		t.Error("no overflow line")
	}
}

// chartServer answers like Yahoo's chart endpoint for a few symbols.
func chartServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sym := strings.TrimPrefix(r.URL.Path, "/")
		if r.URL.Query().Get("range") != "1mo" {
			http.Error(w, "range", http.StatusBadRequest)
			return
		}
		q, ok := sampleQuotes[sym]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`)
			return
		}
		closes, _ := json.Marshal(q.Closes)
		typ := "EQUITY"
		if q.Index {
			typ = "INDEX"
		}
		fmt.Fprintf(w, `{"chart":{"result":[{"meta":{"currency":%q,"symbol":%q,"instrumentType":%q,"shortName":%q,"priceHint":%d},"indicators":{"quote":[{"close":%s}]}}],"error":null}}`,
			q.Currency, sym, typ, q.Name, q.Precision, closes)
	}))
	old := yahooBaseURL
	yahooBaseURL = srv.URL + "/"
	t.Cleanup(func() { yahooBaseURL = old; srv.Close() })
	return srv
}

func series(start float64, steps ...float64) []float64 {
	out := []float64{start}
	for _, s := range steps {
		start += s
		out = append(out, start)
	}
	return out
}

var sampleQuotes = map[string]quote{
	"AAPL":     {Name: "Apple Inc.", Currency: "USD", Precision: 2, Closes: series(318, 2, 3, -1, 4, 2, -3, 5, 1, 2, -2, 3, 4, -1, 2, 1, 3, -2, 1, 4, 2.97)},
	"TSLA":     {Name: "Tesla, Inc.", Currency: "USD", Precision: 2, Closes: series(262, -4, 6, 8, -3, 2, -6, -5, 4, 9, -2, 3, -8, -4, 5, 2, -3, -6, 2, 1, -7.4)},
	"^GDAXI":   {Name: "DAX P", Currency: "EUR", Index: true, Precision: 2, Closes: series(24800, 80, -40, 120, 60, -90, 30, 110, 40, -20, 70, 50, -60, 90, 30, 20, -10, 40, 60, -30, 148.93)},
	"SAP.DE":   {Name: "SAP SE", Currency: "EUR", Precision: 2, Closes: series(241, -1, 2, -3, -2, 1, -4, 2, -1, -2, 3, -1, -2, 1, -3, 2, -1, -2, 1, -1, -2.35)},
	"EURUSD=X": {Name: "EUR/USD", Currency: "USD", Precision: 4, Closes: series(1.1650, .002, -.001, .003, .001, -.002, .004, .001, -.003, .002, .001, .002, -.001, .001, .003, -.002, .001, .001, -.001, .002, .0012)},
	"BTC-USD":  {Name: "Bitcoin USD", Currency: "USD", Precision: 2, Closes: series(112000, 800, -1500, 2200, 900, -300, 1800, -2500, 1200, 600, -400, 1500, 2000, -800, 700, 300, -1200, 900, 400, -600, 512.5)},
}

func sampleRows(lang string) []row {
	var rows []row
	for _, s := range []string{"AAPL", "TSLA", "^GDAXI", "SAP.DE", "EURUSD=X", "BTC-USD"} {
		rows = append(rows, makeRow(s, result{Quote: sampleQuotes[s]}, lang))
	}
	return rows
}

func TestGenerate(t *testing.T) {
	chartServer(t)
	c := modulestest.Common(t, `{"size":[944,180],"padding_x":20,"padding_y":20,"fontsize":22,"language":"de"}`)
	m, err := New(c, json.RawMessage(`{"tickers":"AAPL, TSLA, NOPE"}`))
	if err != nil {
		t.Fatal(err)
	}
	env := &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: fetch.New("")}
	canvas, err := m.Generate(context.Background(), env)
	if err != nil {
		t.Fatal(err)
	}
	if canvas.W != 904 || canvas.H != 140 {
		t.Errorf("canvas %dx%d", canvas.W, canvas.H)
	}
	if _, red := modulestest.InkIn(canvas, 0, 0, canvas.W, canvas.H); !red {
		t.Error("TSLA is down but nothing is red")
	}

	all, err := New(c, json.RawMessage(`{"tickers":"NOPE, GONE"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := all.Generate(context.Background(), env); err == nil || !strings.Contains(err.Error(), "Not Found") {
		t.Errorf("all tickers failing should be an error, got %v", err)
	}
}

// TestPreview writes PNGs for a visual check when INKYCAL_PREVIEW_DIR is set.
func TestPreview(t *testing.T) {
	dir := os.Getenv("INKYCAL_PREVIEW_DIR")
	if dir == "" {
		t.Skip("INKYCAL_PREVIEW_DIR not set")
	}
	chartServer(t)
	for _, p := range []struct {
		file, lang, tickers string
		w, h                int
	}{
		{"stocks-904x140.png", "de", "AAPL, TSLA, ^GDAXI, SAP.DE, EURUSD=X, BTC-USD", 904, 140},
		{"stocks-904x600.png", "de", "AAPL, TSLA, ^GDAXI, SAP.DE, EURUSD=X, BTC-USD, NOPE", 904, 600},
		{"stocks-904x140-en.png", "en", "AAPL, TSLA, ^GDAXI", 904, 140},
		{"stocks-400x200.png", "en", "AAPL, TSLA, ^GDAXI, SAP.DE, EURUSD=X", 400, 200},
	} {
		c := modulestest.Common(t, fmt.Sprintf(`{"size":[%d,%d],"fontsize":22,"language":%q}`, p.w, p.h, p.lang))
		m, err := New(c, json.RawMessage(fmt.Sprintf(`{"tickers":%q}`, p.tickers)))
		if err != nil {
			t.Fatal(err)
		}
		canvas, err := m.Generate(context.Background(), &modules.Env{Now: time.Now(), Loc: time.UTC, Fetch: fetch.New("")})
		if err != nil {
			t.Fatal(err)
		}
		writePNG(t, filepath.Join(dir, p.file), canvas)
	}
}

func writePNG(t *testing.T, path string, c *render.Canvas) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for y := 0; y < c.H; y++ {
		for x := 0; x < c.W; x++ {
			col := color.RGBA{255, 255, 255, 255}
			if b, r := c.InkAt(x, y); r {
				col = color.RGBA{220, 0, 0, 255}
			} else if b {
				col = color.RGBA{0, 0, 0, 255}
			}
			img.Set(x, y, col)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func TestReviewFixes(t *testing.T) {
	// Yahoo leaves yesterday's close empty: its own day change wins over the closes.
	body := []byte(`{"chart":{"result":[{"meta":{"currency":"GBp","regularMarketPrice":118.7,"regularMarketChangePercent":1.714,"priceHint":2},
		"indicators":{"quote":[{"close":[118.35,null,118.70]}]}}]}}`)
	q, err := parseChart("VOD.L", body)
	if err != nil {
		t.Fatal(err)
	}
	if got := q.ChangePct(); got != 1.714 {
		t.Errorf("day change %v, want Yahoo's 1.714", got)
	}
	// Pence are shown as pounds.
	if q.Currency != "GBP" || q.Price() < 1.18 || q.Price() > 1.19 {
		t.Errorf("price %v %s, want 1.187 GBP", q.Price(), q.Currency)
	}
	if got := priceLabel(q, "en"); got != "£1.19" {
		t.Errorf("label %q", got)
	}
}

func TestOneRowStillShowsATicker(t *testing.T) {
	c := render.NewCanvas(600, 30, render.FontDefault, 18)
	rows := []row{{Symbol: "AAPL", Name: "Apple", Price: "1", Change: "+1%"}, {Symbol: "MSFT", Name: "Microsoft", Price: "2", Change: "+2%"}}
	drawRows(c, rows, 18, "en")
	black := 0
	for x := range 60 {
		for y := 0; y < c.H; y++ {
			if b, _ := c.InkAt(x, y); b {
				black++
			}
		}
	}
	if black == 0 {
		t.Error("first ticker symbol not drawn")
	}
}
