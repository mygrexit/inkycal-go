package app

import (
	"math"

	"inkycal/internal/config"
	"inkycal/internal/render"
)

var fontRoles = map[string]render.FontID{
	"text": render.FontRegular, "strong": render.FontDefault, "bold": render.FontBold, "display": render.FontDisplay,
}

// themeFor turns a module's effective style into its fonts; nil keeps the defaults.
func themeFor(st config.Style, base string) *render.Theme {
	if (st.Font == "" || st.Font == "sans" || st.Font == "custom") && len(st.CustomFonts) == 0 {
		return nil
	}
	t := &render.Theme{Fonts: render.Pair(st.Font), Base: base}
	for role, file := range st.CustomFonts {
		if id, ok := fontRoles[role]; ok && file != "" {
			t.Fonts[id] = render.FontSource{File: file}
		}
	}
	return t
}

var inkByName = map[string]render.Ink{"white": render.InkWhite, "black": render.InkBlack, "colour": render.InkColour}

// inksFor turns a module's colours into an ink map; nil when they are the defaults.
func inksFor(st config.Style) *render.InkMap {
	m := render.DefaultInks
	if ink, ok := inkByName[st.Colors.Paper]; ok {
		m.Paper = ink
	}
	if ink, ok := inkByName[st.Colors.Ink]; ok {
		m.Ink = ink
	}
	if ink, ok := inkByName[st.Colors.Accent]; ok {
		m.Accent = ink
	}
	if m == render.DefaultInks {
		return nil
	}
	return &m
}

// scaled applies a style's size factor to a font size.
func scaled(size config.FlexInt, scale float64) config.FlexInt {
	if scale == 0 {
		return size
	}
	return config.FlexInt(max(6, int(math.Round(float64(size)*scale))))
}
