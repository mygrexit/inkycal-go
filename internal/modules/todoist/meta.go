package todoist

import "inkycal/internal/modules"

var Meta = modules.Meta{
	Name: "Todoist", Title: "Todoist", Description: "Open tasks as a two-column checklist, most urgent first.",
	MinW: 120, MinH: 48,
	Fields: []modules.Field{
		{Key: "api_key", Label: "Todoist API token", Kind: modules.KindSecret, Required: true},
		{Key: "project_filter", Label: "Only these projects", Help: "Empty shows all projects", Kind: modules.KindTextList},
		{Key: "show_priority", Label: "Mark priority 1 in red", Kind: modules.KindBool, Default: true},
	},
}
