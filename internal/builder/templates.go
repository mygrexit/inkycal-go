package builder

import "inkycal/internal/layout"

// quotes fill the "quote of the day" tiles; one line per day.
const quotes = "Carpe diem.\nLess is more.\nOne step at a time.\nBe kind.\nEnjoy the little things.\nToday is a good day."

// Templates use the default 12 × 16 grid and these paddings; the assistant fills in the
// language, font size and every field's default.
const (
	templatePadX = 16
	templatePadY = 12
)

type templateModule struct {
	Name   string         `json:"name"`
	Cell   layout.Cell    `json:"cell"`
	Config map[string]any `json:"config,omitempty"`
}

type template struct {
	ID          string           `json:"id"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Modules     []templateModule `json:"modules"`
}

var templates = []template{
	{"family", "Family calendar", "Weather, the family calendar and a note for everyone.", []templateModule{
		{"Weather", layout.Cell{0, 0, 12, 4}, nil},
		{"Calendar", layout.Cell{0, 4, 12, 9}, nil},
		{"TextToDisplay", layout.Cell{0, 13, 12, 3}, map[string]any{"source": "text", "text": "# Notes\nHave a nice day!"}},
	}},
	{"weather_calendar", "Weather and appointments", "Today's weather and the coming days as a list.", []templateModule{
		{"Weather", layout.Cell{0, 0, 12, 5}, nil},
		{"Agenda", layout.Cell{0, 5, 12, 11}, nil},
	}},
	{"daily", "Daily overview", "Weather, the month with your appointments, a quote of the day and the news.", []templateModule{
		{"Weather", layout.Cell{0, 0, 12, 4}, nil},
		{"Calendar", layout.Cell{0, 4, 12, 8}, nil},
		{"TextToDisplay", layout.Cell{0, 12, 12, 2}, map[string]any{"source": "text", "one_per_day": true, "text": quotes}},
		{"Feeds", layout.Cell{0, 14, 12, 2}, nil},
	}}, {"picture", "Picture frame", "A big picture with the weather below.", []templateModule{
		{"Inkyimage", layout.Cell{0, 0, 12, 12}, nil},
		{"Weather", layout.Cell{0, 12, 12, 4}, nil},
	}},
	{"word", "Word of the day", "A Greek word for kids, a quote of the day and the weather.", []templateModule{
		{"Greek", layout.Cell{0, 0, 12, 5}, nil},
		{"TextToDisplay", layout.Cell{0, 5, 12, 5}, map[string]any{"source": "text", "one_per_day": true,
			"text": quotes}},
		{"Weather", layout.Cell{0, 10, 12, 6}, nil},
	}},
}
