package weather

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"inkycal/internal/fetch"
)

func TestCoordsURL(t *testing.T) {
	u, err := url.Parse(CoordsURL("http://x/data/2.5", "forecast", "key", 50.94, -6.5, "de"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Path != "/data/2.5/forecast" || q.Get("lat") != "50.94" || q.Get("lon") != "-6.5" ||
		q.Get("appid") != "key" || q.Get("units") != "metric" || q.Get("lang") != "de" || q.Has("q") || q.Has("id") {
		t.Errorf("url = %s", u)
	}
}

// The extra fields fullweather needs: description, feels-like, gusts and rain + snow.
func TestFetchExtraFields(t *testing.T) {
	loc := berlin(t)
	t1 := time.Date(2026, 10, 10, 12, 0, 0, 0, loc)
	body := fmt.Sprintf(`{"list":[{"dt":%d,"main":{"temp":3},"weather":[{"icon":"13d"}],"wind":{"speed":2,"gust":5.5},"rain":{"3h":0.5},"snow":{"3h":1.25}},`+
		`{"dt":%d,"main":{"temp":4},"weather":[{"icon":"03d"}],"wind":{"speed":2}}]}`, t1.Unix(), t1.Add(3*time.Hour).Unix())
	serveOWM(t, `{"weather":[{"icon":"13d","description":"light snow"}],"main":{"temp":-1,"feels_like":-4.5}}`, body)

	cur, _, err := FetchCurrent(context.Background(), fetch.New(""), owmBaseURL+"/weather?units=metric", loc)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Description != "light snow" || cur.FeelsLike != -4.5 {
		t.Errorf("current = %+v", cur)
	}
	points, _, err := FetchForecast(context.Background(), fetch.New(""), owmBaseURL+"/forecast?units=metric", loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 2 || points[0].Gust != 5.5 || points[0].Precip != 1.75 || points[1].Gust != 2 || points[1].Precip != 0 {
		t.Errorf("points = %+v", points)
	}
}
