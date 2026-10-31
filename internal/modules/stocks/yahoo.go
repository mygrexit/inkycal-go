package stocks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"

	"inkycal/internal/fetch"
	"inkycal/internal/problem"
)

// yahooBaseURL is a variable so tests can point it at an httptest server.
// The chart endpoint is unofficial (it is what Yahoo's own pages and yfinance use)
// and may change or start refusing requests without notice.
var yahooBaseURL = "https://query1.finance.yahoo.com/v8/finance/chart/"

// quote is what one ticker shows: the last month of closes and the day's move.
type quote struct {
	Name      string
	Currency  string
	Index     bool // an index is quoted in points, not money
	Precision int
	Closes    []float64 // oldest first, the last one is the current price
	DayPct    *float64  // Yahoo's own day change; Closes can have gaps
}

// Price is the current quote, Previous the close before it.
func (q quote) Price() float64 { return q.Closes[len(q.Closes)-1] }

func (q quote) Previous() float64 {
	if len(q.Closes) < 2 {
		return q.Price()
	}
	return q.Closes[len(q.Closes)-2]
}

// ChangePct is the day's move in percent: Yahoo's figure when it sends one (a missing
// close for yesterday would otherwise compare with two sessions ago), else upstream's
// (1 - current/previous) * -100.
func (q quote) ChangePct() float64 {
	if q.DayPct != nil {
		return *q.DayPct
	}
	p := q.Previous()
	if p == 0 {
		return 0
	}
	return (q.Price()/p - 1) * 100
}

type chartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency           string   `json:"currency"`
				Symbol             string   `json:"symbol"`
				ShortName          string   `json:"shortName"`
				LongName           string   `json:"longName"`
				InstrumentType     string   `json:"instrumentType"`
				PriceHint          *int     `json:"priceHint"`
				RegularMarketPrice *float64 `json:"regularMarketPrice"`
				RegularMarketPct   *float64 `json:"regularMarketChangePercent"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

var minorUnits = map[string]string{"GBp": "GBP", "GBX": "GBP", "ZAc": "ZAR", "ZAC": "ZAR", "ILA": "ILS"}

func chartURL(ticker string) string {
	return yahooBaseURL + url.PathEscape(ticker) + "?range=1mo&interval=1d"
}

// parseChart turns a chart response into a quote. Missing closes (holidays,
// halted days) are skipped; the live price replaces the last close, as yfinance does.
func parseChart(ticker string, body []byte) (quote, error) {
	var r chartResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return quote{}, problem.Mark(problem.BadResponse, fmt.Errorf("%s: invalid JSON: %w", ticker, err))
	}
	if e := r.Chart.Error; e != nil {
		return quote{}, fmt.Errorf("%s: %s: %s", ticker, e.Code, e.Description)
	}
	if len(r.Chart.Result) == 0 {
		return quote{}, fmt.Errorf("%s: no data", ticker)
	}
	res := r.Chart.Result[0]
	m := res.Meta
	q := quote{Name: m.ShortName, Currency: m.Currency, Index: m.InstrumentType == "INDEX", Precision: 2}
	if p := m.RegularMarketPct; p != nil && !math.IsNaN(*p) && !math.IsInf(*p, 0) {
		q.DayPct = p
	}
	if q.Name == "" {
		q.Name = m.LongName
	}
	if q.Name == "" {
		q.Name = ticker
	}
	if m.PriceHint != nil && *m.PriceHint >= 0 && *m.PriceHint <= 6 {
		q.Precision = *m.PriceHint
	}
	if len(res.Indicators.Quote) > 0 {
		for _, c := range res.Indicators.Quote[0].Close {
			if c != nil && !math.IsNaN(*c) && !math.IsInf(*c, 0) {
				q.Closes = append(q.Closes, *c)
			}
		}
	}
	if p := m.RegularMarketPrice; p != nil && *p > 0 {
		if len(q.Closes) == 0 {
			q.Closes = []float64{*p}
		} else {
			q.Closes[len(q.Closes)-1] = *p
		}
	}
	if len(q.Closes) == 0 {
		return quote{}, fmt.Errorf("%s: no prices", ticker)
	}
	// Minor units (London pence "GBp", Johannesburg cents "ZAc", Tel Aviv agorot "ILA")
	// are shown in the main currency, as one would read the price.
	if major, ok := minorUnits[q.Currency]; ok {
		for i := range q.Closes {
			q.Closes[i] /= 100
		}
		q.Currency = major
	}
	return q, nil
}

// result is one ticker's outcome; Err leaves the row as "no data".
type result struct {
	Quote quote
	Stale bool
	Err   error
}

// fetchAll loads the tickers in parallel (one round trip instead of n on a
// battery wake) and keeps their order.
func fetchAll(ctx context.Context, fc *fetch.Client, tickers []string) []result {
	out := make([]result, len(tickers))
	done := make(chan struct{})
	for i, t := range tickers {
		go func() {
			defer func() { done <- struct{}{} }()
			res, err := fc.Get(ctx, chartURL(t), nil)
			if err != nil {
				out[i].Err = err
				return
			}
			out[i].Stale = res.Stale
			out[i].Quote, out[i].Err = parseChart(t, res.Body)
		}()
	}
	for range tickers {
		<-done
	}
	return out
}

// allFailed reports whether no ticker could be loaded and joins the errors.
func allFailed(rs []result) error {
	var errs []error
	for _, r := range rs {
		if r.Err == nil {
			return nil
		}
		errs = append(errs, r.Err)
	}
	return errors.Join(errs...)
}
