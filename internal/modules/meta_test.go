package modules

import (
	"strings"
	"testing"

	"inkycal/internal/config"
)

func TestCheckMinSize(t *testing.T) {
	m := Meta{Name: "Todoist", MinW: 50, MinH: 20}
	ok := config.Common{Size: [2]int{70, 40}, PaddingX: 10, PaddingY: 10}
	if err := CheckMinSize(ok, m); err != nil {
		t.Errorf("exact minimum refused: %v", err)
	}
	small := config.Common{Size: [2]int{69, 40}, PaddingX: 10, PaddingY: 10}
	if err := CheckMinSize(small, m); err == nil || !strings.Contains(err.Error(), "todoist: inner size 49x20 too small (min 50x20)") {
		t.Errorf("err = %v", err)
	}
}

func TestAutoFontSize(t *testing.T) {
	cases := []struct{ w, h, want int }{
		{904, 624, 22}, // the user's calendar: width-bound
		{904, 250, 22},
		{300, 60, 12},    // floor
		{2000, 2000, 32}, // ceiling
	}
	for _, c := range cases {
		if got := AutoFontSize(c.w, c.h); got != c.want {
			t.Errorf("AutoFontSize(%d, %d) = %d, want %d", c.w, c.h, got, c.want)
		}
	}
}

func TestDefaults(t *testing.T) {
	m := Meta{Fields: []Field{{Key: "a", Default: 24}, {Key: "b"}, {Key: "c", Default: "x"}}}
	d := m.Defaults()
	if len(d) != 2 || d["a"] != 24 || d["c"] != "x" {
		t.Errorf("defaults %v", d)
	}
}
