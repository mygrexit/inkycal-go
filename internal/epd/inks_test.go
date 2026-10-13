package epd

import "testing"

func TestEveryModelHasAnInkSet(t *testing.T) {
	for m := range Models {
		switch Inks(m) {
		case "bw", "bwr", "gray16":
		default:
			t.Errorf("%s: ink set %q", m, Inks(m))
		}
	}
	want := map[string]string{
		"epd_12_in_48_colour_V2": "bwr", "epd_7_in_5_v2_colour": "bwr", "epd5in83b_V2": "bwr",
		"epd_7_in_5_v2": "bw", "epd_4_in_2": "bw", "epd_12_in_48": "bw",
		"9_in_7": "gray16", "10_in_3": "gray16", "7_in_8": "gray16",
		"image_file": "bwr",
	}
	for m, ink := range want {
		if got := Inks(m); got != ink {
			t.Errorf("Inks(%s) = %q, want %q", m, got, ink)
		}
	}
	if d := Diagonal("epd_12_in_48_colour_V2"); d != 12.48 {
		t.Errorf("diagonal %v", d)
	}
}
