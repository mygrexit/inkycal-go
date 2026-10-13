package epd

import (
	"sort"
	"strings"
)

// Panel describes a model in words a buyer recognises from the box.
type Panel struct {
	Model  string  `json:"model"`
	Name   string  `json:"name"`
	Inches float64 `json:"inches"`
	Inks   string  `json:"inks"`
	W      int     `json:"w"`
	H      int     `json:"h"`
	Group  string  `json:"group"` // size group for the picker
	Hint   string  `json:"hint,omitempty"`
}

// panelText names and explains each model; colours and sizes come from Inks and Models.
var panelText = map[string][2]string{ // model → {name without colours, hint}
	"epd_4_in_2":             {"Waveshare 4.2″", ""},
	"epd_4_in_2_colour":      {"Waveshare 4.2″ (B/C)", "Colour models print red (B) or yellow (C): pick yours under Theme."},
	"epd_5_in_83":            {"Waveshare 5.83″ V1", "V1 has 600 × 448 pixels; newer panels are V2."},
	"epd_5_in_83_colour":     {"Waveshare 5.83″ V1 (B/C)", "V1 has 600 × 448 pixels; newer panels are V2."},
	"epd_5_in_83_V2":         {"Waveshare 5.83″ V2", "V2 has 648 × 480 pixels (sticker on the back)."},
	"epd5in83b_V2":           {"Waveshare 5.83″ V2 (B)", "V2 has 648 × 480 pixels (sticker on the back)."},
	"epd_7_in_5":             {"Waveshare 7.5″ V1", "The first 7.5″ panel: 640 × 384 pixels."},
	"epd_7_in_5_colour":      {"Waveshare 7.5″ V1 (B/C)", "The first 7.5″ panel: 640 × 384 pixels. Red (B) or yellow (C)."},
	"epd_7_in_5_v2":          {"Waveshare 7.5″ V2", "800 × 480 pixels, the most common 7.5″ panel."},
	"epd_7_in_5_v2_colour":   {"Waveshare 7.5″ V2 (B)", "800 × 480 pixels, the most common 7.5″ panel."},
	"epd_7_in_5_v3":          {"Waveshare 7.5″ HD", "880 × 528 pixels, sold as HD or V3."},
	"epd_7_in_5_v3_colour":   {"Waveshare 7.5″ HD (B)", "880 × 528 pixels, sold as HD or V3."},
	"epd_12_in_48":           {"Waveshare 12.48″", ""},
	"epd_12_in_48_colour":    {"Waveshare 12.48″ V1 (B)", "Older colour version; the current one is V2."},
	"epd_12_in_48_colour_V2": {"Waveshare 12.48″ V2", "Current colour version (sticker on the back says V2)."},
	"epd_13_in_3":            {"Waveshare 13.3″ (K)", ""},
	"epd_13_in_3_colour":     {"Waveshare 13.3″ (B)", ""},
	"7_in_8":                 {"Waveshare 7.8″ IT8951 HAT", "16 grey levels, driven by the IT8951 board."},
	"9_in_7":                 {"Waveshare 9.7″ IT8951 HAT", "16 grey levels, driven by the IT8951 board."},
	"10_in_3":                {"Waveshare 10.3″ IT8951 HAT", "16 grey levels, driven by the IT8951 board."},
	"image_file":             {"No display (PNG, 7.5″ size)", "Writes the picture to a file — for trying things out."},
	"image_file_12_in_48":    {"No display (PNG, 12.48″ size)", "Writes the picture to a file — for trying things out."},
}

var inkWords = map[string]string{"bw": "black, white", "bwr": "black, white, red", "gray16": "16 greys"}

// PanelOf describes one model; unknown models get their code as name.
func PanelOf(model string) Panel {
	size := Models[model]
	text, ok := panelText[model]
	if !ok {
		text = [2]string{model, ""}
	}
	inches := Diagonal(model)
	p := Panel{Model: model, Inks: Inks(model), W: size[0], H: size[1], Inches: inches, Hint: text[1]}
	p.Name = text[0] + " · " + inkWords[p.Inks]
	if strings.HasPrefix(model, "image_file") {
		p.Name = text[0]
	}
	switch {
	case inches < 5:
		p.Group = "4″"
	case inches < 7:
		p.Group = "5–6″"
	case inches < 9:
		p.Group = "7–8″"
	case inches < 12:
		p.Group = "9–10″"
	default:
		p.Group = "12–13″"
	}
	return p
}

// Panels lists every supported model, smallest first.
func Panels() []Panel {
	out := make([]Panel, 0, len(Models))
	for m := range Models {
		out = append(out, PanelOf(m))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Inches != out[j].Inches {
			return out[i].Inches < out[j].Inches
		}
		return out[i].Name < out[j].Name
	})
	return out
}
