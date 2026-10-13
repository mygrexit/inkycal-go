package feeds

import (
	"testing"

	"inkycal/internal/config"
	"inkycal/internal/modules/modulestest"
	"inkycal/internal/render"
)

func TestShapes(t *testing.T) {
	items := []item{
		{"Bahnstreik ab Donnerstag: Was Pendler jetzt wissen müssen", "Die GDL hat einen 48-stündigen Streik angekündigt. Die Bahn will einen Notfahrplan anbieten.", "tagesschau.de"},
		{"Herbstferien: Stau-Prognose für das Wochenende", "Der ADAC erwartet volle Autobahnen vor allem in Richtung Süden.", "ADAC"},
		{"Neue Kita in der Südstadt eröffnet", "Nach zwei Jahren Bauzeit bietet die Einrichtung Platz für 90 Kinder.", "Stadtanzeiger"},
	}
	modulestest.Shapes(t, Meta, func(c config.Common) (*render.Canvas, error) {
		w, h := c.InnerSize()
		cv := render.NewCanvas(w, h, render.FontDefault, int(c.FontSize))
		draw(cv, append([]item(nil), items...), int(c.FontSize), c.Language)
		return cv, nil
	})
}
